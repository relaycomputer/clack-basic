package codegen

/*
 ==============================================================================
 ⚡ CODE-GENERATOR FÜR CLACK-BASIC
 ==============================================================================
 Übersetzt den abstrakten Syntaxbaum (AST) in 16-Bit Relais-Maschinencode.
 Behandelt:
  - Ein-Register-Architektur (Accu)
  - Dynamisches Operand-Patching für Variablenarithmetik
  - 2er-Komplement-Subtraktion (A + ~B + 1)
  - Automatische Einbindung von Runtime-Routinen (Multiplikation, Division, Display)
  - 2-Pass Label- und Sprungadressen-Auflösung
 ==============================================================================
*/

import (
	"fmt"
	"strings"

	"clack-basic/pkg/ast"
)

const (
	MaxMemorySize = 1024
	VarBaseAddr   = 930 // Variablen werden ab Adresse 930 abgelegt
	TempBaseAddr  = 950 // Temporäre Berechnungszellen ab Adresse 950
	DisplayBase   = 960 // Display-Bereich: 960..1023
)

// CodeGenerator übersetzt AST in Maschinencode.
type CodeGenerator struct {
	memory       [MaxMemorySize]uint16
	pc           uint16
	symbols      map[string]uint16 // Variablenname -> RAM-Adresse
	nextVarAddr  uint16
	nextTempAddr uint16

	labels       map[string]uint16 // Label-Name -> PC-Adresse
	fixups       []labelFixup      // Unaufgelöste Sprungziele (Forward Jumps)

	// Runtime-Bedarf
	needMul  bool
	needDiv  bool
	needSqr  bool
	needCls  bool
	needPset bool
	needLine bool

	// DATA-Tabelle
	dataTable []int
	dataPtr   uint16

	forStack []forLoopContext
	comments map[uint16]string
}

type labelFixup struct {
	patchPC uint16
	label   string
	line    int
}

type forLoopContext struct {
	varName   string
	stepVal   int
	loopStart uint16
	endAddr   uint16
}

// NewCodeGenerator initialisiert den Code-Generator.
func NewCodeGenerator() *CodeGenerator {
	return &CodeGenerator{
		symbols:      make(map[string]uint16),
		nextVarAddr:  VarBaseAddr,
		nextTempAddr: TempBaseAddr,
		labels:       make(map[string]uint16),
		fixups:       []labelFixup{},
		dataTable:    []int{},
		forStack:     []forLoopContext{},
		comments:     make(map[uint16]string),
	}
}

// Generate übersetzt das Programm und liefert das fertige 1024-Wort-Speicherabbild.
func (g *CodeGenerator) Generate(prog *ast.Program) ([MaxMemorySize]uint16, map[uint16]string, error) {
	// Start-Sprung über Boot-Bereich (Sprung zu eigentlichem Programmstart)
	g.emit(OpBA, 0, "Initialer Sprung zum Hauptprogramm")
	g.fixups = append(g.fixups, labelFixup{patchPC: 1, label: "__main_start"})

	// Hauptprogramm-Einstieg
	g.pc = 10 // Start ab PC = 10
	g.labels["__main_start"] = g.pc

	// Initialisiere __data_ptr nur wenn DATA existiert
	if len(g.dataTable) > 0 {
		ptrAddr := g.getOrCreateVar("__data_ptr")
		g.emit(OpLDA, 0, "Init __data_ptr")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__data_table_start"})
		g.emit(OpSTAM, ptrAddr, "Speichere __data_ptr")
	}

	// Statements nacheinander übersetzen
	for _, stmt := range prog.Statements {
		if err := g.compileStmt(stmt); err != nil {
			return g.memory, g.comments, err
		}
	}

	// Programmabschluss (HALT / Endlosschleife)
	haltPC := g.pc
	g.emit(OpBA, haltPC, "Ende des Programms (Halt-Schleife)")

	// Eingebettete Subroutinen anhängen
	g.emitRuntimeRoutines()

	// DATA-Tabelle im oberen Speicher ablegen
	g.emitDataTable()

	if g.pc >= VarBaseAddr {
		return g.memory, g.comments, fmt.Errorf("Programm zu groß: Code-Größe (%d Worte) überschreitet Variablenbereich ab Adresse %d", g.pc, VarBaseAddr)
	}

	// 2. Pass: Alle offenen Sprungmarken (Fixups) auflösen
	for _, fix := range g.fixups {
		targetPC, exists := g.labels[fix.label]
		if !exists {
			return g.memory, g.comments, fmt.Errorf("Undefiniertes Sprungziel / Label '%s'", fix.label)
		}
		g.memory[fix.patchPC] = targetPC
	}

	return g.memory, g.comments, nil
}

// ============================================================================
// STATEMENTS KOMPILIEREN
// ============================================================================

func (g *CodeGenerator) compileStmt(stmt ast.Stmt) error {
	switch s := stmt.(type) {
	case *ast.LabelStmt:
		g.labels[s.Name] = g.pc
		return nil

	case *ast.LetStmt:
		varAddr := g.getOrCreateVar(s.VarName)
		// Wert des Ausdrucks in Accu berechnen
		if err := g.compileExpr(s.Value); err != nil {
			return err
		}
		// Accu in Variable speichern
		g.emit(OpSTAM, varAddr, fmt.Sprintf("LET %s = Accu", s.VarName))
		return nil

	case *ast.GotoStmt:
		g.emit(OpBA, 0, fmt.Sprintf("GOTO %s", s.Target))
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: s.Target, line: s.Line})
		return nil

	case *ast.GosubStmt:
		g.emit(OpCALL, 0, fmt.Sprintf("GOSUB %s", s.Target))
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: s.Target, line: s.Line})
		return nil

	case *ast.ReturnStmt:
		g.emitSingle(OpRET, "RETURN aus Unterprogramm")
		return nil

	case *ast.IfStmt:
		return g.compileIf(s)

	case *ast.ForStmt:
		return g.compileFor(s)

	case *ast.DataStmt:
		g.dataTable = append(g.dataTable, s.Values...)
		return nil

	case *ast.ReadStmt:
		return g.compileRead(s)

	case *ast.RestoreStmt:
		ptrAddr := g.getOrCreateVar("__data_ptr")
		g.emit(OpLDA, 0, "RESTORE DATA Pointer")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__data_table_start"})
		g.emit(OpSTAM, ptrAddr, "Speichere __data_ptr")
		return nil

	case *ast.PokeStmt:
		// 1. Berechne Wert
		if err := g.compileExpr(s.Value); err != nil {
			return err
		}
		valTemp := g.allocTemp()
		g.emit(OpSTAM, valTemp, "POKE Temp Wert")

		// 2. Berechne Adresse
		if err := g.compileExpr(s.Address); err != nil {
			return err
		}
		// Dynamischer Store über Opcode-Patching
		patchStore := g.pc + 5
		g.emit(OpSTAM, patchStore, "Patch POKE Zieladresse")
		g.emit(OpLDAM, valTemp, "Lade POKE Wert")
		g.emit(OpSTAM, 0, "POKE [dynamisch]")
		return nil

	case *ast.ClsStmt:
		g.needCls = true
		ptrCls := g.getOrCreateVar("__cls_ptr")
		endCls := g.getOrCreateVar("__cls_end_addr")

		if s.StartRow != nil {
			// StartAddr = DisplayBase + StartRow * 2
			if err := g.compileExpr(s.StartRow); err != nil {
				return err
			}
			g.emit(OpSHL, 0, "StartRow * 2")
			g.emit(OpADD, DisplayBase, "DisplayBase + StartRow*2")
			g.emit(OpSTAM, ptrCls, "Setze __cls_ptr")

			// EndAddr = DisplayBase + EndRow * 2 + 2
			if err := g.compileExpr(s.EndRow); err != nil {
				return err
			}
			g.emit(OpSHL, 0, "EndRow * 2")
			g.emit(OpADD, DisplayBase+2, "DisplayBase + EndRow*2 + 2")
			g.emit(OpSTAM, endCls, "Setze __cls_end_addr")
		} else {
			// Volles Display löschen: 960 bis 1024
			g.emit(OpLDA, DisplayBase, "Init __cls_ptr (960)")
			g.emit(OpSTAM, ptrCls, "Speichere __cls_ptr")
			g.emit(OpLDA, 1024, "Init __cls_end_addr (1024)")
			g.emit(OpSTAM, endCls, "Speichere __cls_end_addr")
		}

		g.emit(OpCALL, 0, "CALL __cls (Display loeschen)")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__runtime_cls"})
		return nil

	case *ast.PsetStmt:
		g.needPset = true
		// X, Y, Color in Parameter-Register ablegen
		if err := g.compileExpr(s.X); err != nil {
			return err
		}
		g.emit(OpSTAM, g.getOrCreateVar("__pset_x"), "PSET X")

		if err := g.compileExpr(s.Y); err != nil {
			return err
		}
		g.emit(OpSTAM, g.getOrCreateVar("__pset_y"), "PSET Y")

		if err := g.compileExpr(s.Color); err != nil {
			return err
		}
		g.emit(OpSTAM, g.getOrCreateVar("__pset_c"), "PSET Color")

		g.emit(OpCALL, 0, "CALL __pset")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__runtime_pset"})
		return nil

	case *ast.PixelMoveStmt:
		g.needPset = true
		xReg := g.getOrCreateVar("__pset_x")
		yReg := g.getOrCreateVar("__pset_y")
		cReg := g.getOrCreateVar("__pset_c")
		newXTemp := g.allocTemp()
		newYTemp := g.allocTemp()

		// 1. Berechne neue Zielkoordinaten
		if s.IsRelative {
			if err := g.compileExpr(&ast.BinaryExpr{Left: s.FromX, Op: "+", Right: s.ToX}); err != nil {
				return err
			}
			g.emit(OpSTAM, newXTemp, "PIXELMOVE newX (relative)")

			if err := g.compileExpr(&ast.BinaryExpr{Left: s.FromY, Op: "+", Right: s.ToY}); err != nil {
				return err
			}
			g.emit(OpSTAM, newYTemp, "PIXELMOVE newY (relative)")
		} else {
			if err := g.compileExpr(s.ToX); err != nil {
				return err
			}
			g.emit(OpSTAM, newXTemp, "PIXELMOVE newX (absolute)")

			if err := g.compileExpr(s.ToY); err != nil {
				return err
			}
			g.emit(OpSTAM, newYTemp, "PIXELMOVE newY (absolute)")
		}

		// 2. Alten Pixel loeschen (Color = 0)
		if err := g.compileExpr(s.FromX); err != nil {
			return err
		}
		g.emit(OpSTAM, xReg, "PIXELMOVE oldX")

		if err := g.compileExpr(s.FromY); err != nil {
			return err
		}
		g.emit(OpSTAM, yReg, "PIXELMOVE oldY")

		g.emit(OpLDA, 0, "PIXELMOVE Color=0 (Erase)")
		g.emit(OpSTAM, cReg, "PSET Color 0")
		g.emit(OpCALL, 0, "CALL __pset (Erase Old Pixel)")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__runtime_pset"})

		// 3. Neuen Pixel setzen
		g.emit(OpLDAM, newXTemp, "Lade newX")
		g.emit(OpSTAM, xReg, "PSET newX")
		if v, ok := s.FromX.(*ast.VariableExpr); ok && s.IsRelative {
			g.emit(OpSTAM, g.getOrCreateVar(v.Name), "Update Var X")
		}

		g.emit(OpLDAM, newYTemp, "Lade newY")
		g.emit(OpSTAM, yReg, "PSET newY")
		if v, ok := s.FromY.(*ast.VariableExpr); ok && s.IsRelative {
			g.emit(OpSTAM, g.getOrCreateVar(v.Name), "Update Var Y")
		}

		if err := g.compileExpr(s.Color); err != nil {
			return err
		}
		g.emit(OpSTAM, cReg, "PSET newColor")
		g.emit(OpCALL, 0, "CALL __pset (Draw New Pixel)")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__runtime_pset"})
		return nil

	case *ast.LineStmt:
		g.needLine = true
		x1Reg := g.getOrCreateVar("__line_x1")
		y1Reg := g.getOrCreateVar("__line_y1")
		x2Reg := g.getOrCreateVar("__line_x2")
		y2Reg := g.getOrCreateVar("__line_y2")
		cReg := g.getOrCreateVar("__line_c")

		if err := g.compileExpr(s.X1); err != nil {
			return err
		}
		g.emit(OpSTAM, x1Reg, "LINE X1")

		if err := g.compileExpr(s.Y1); err != nil {
			return err
		}
		g.emit(OpSTAM, y1Reg, "LINE Y1")

		if err := g.compileExpr(s.X2); err != nil {
			return err
		}
		g.emit(OpSTAM, x2Reg, "LINE X2")

		if err := g.compileExpr(s.Y2); err != nil {
			return err
		}
		g.emit(OpSTAM, y2Reg, "LINE Y2")

		if err := g.compileExpr(s.Color); err != nil {
			return err
		}
		g.emit(OpSTAM, cReg, "LINE Color")

		g.emit(OpCALL, 0, "CALL __line")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__runtime_line"})
		return nil

	case *ast.EndStmt:
		haltPC := g.pc
		g.emit(OpBA, haltPC, "END / HALT")
		return nil

	default:
		return fmt.Errorf("Nicht unterstütztes Statement: %T", stmt)
	}
}

func (g *CodeGenerator) compileIf(s *ast.IfStmt) error {
	// Bedingung in Accu auswerten: 0 = false, != 0 = true
	if err := g.compileExpr(s.Condition); err != nil {
		return err
	}

	// Wenn Accu == 0 (false), springe zu Else / EndIf
	elseLabel := fmt.Sprintf("__else_%d", g.pc)
	endIfLabel := fmt.Sprintf("__endif_%d", g.pc)

	targetFalse := elseLabel
	if len(s.ElseBranch) == 0 {
		targetFalse = endIfLabel
	}

	g.emit(OpBZ, 0, "IF False -> Skip")
	g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: targetFalse})

	// THEN Zweig
	for _, tStmt := range s.ThenBranch {
		if err := g.compileStmt(tStmt); err != nil {
			return err
		}
	}

	// Falls ELSE existiert
	if len(s.ElseBranch) > 0 {
		g.emit(OpBA, 0, "GOTO EndIf")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: endIfLabel})

		g.labels[elseLabel] = g.pc
		for _, eStmt := range s.ElseBranch {
			if err := g.compileStmt(eStmt); err != nil {
				return err
			}
		}
	}

	g.labels[endIfLabel] = g.pc
	return nil
}

func (g *CodeGenerator) compileFor(s *ast.ForStmt) error {
	varAddr := g.getOrCreateVar(s.VarName)

	// 1. Initialisiere Startwert: Var = Start
	if err := g.compileExpr(s.Start); err != nil {
		return err
	}
	g.emit(OpSTAM, varAddr, fmt.Sprintf("FOR %s Startwert", s.VarName))

	// 2. Berechne und speichere Endwert
	if err := g.compileExpr(s.End); err != nil {
		return err
	}
	endAddr := g.allocTemp()
	g.emit(OpSTAM, endAddr, fmt.Sprintf("FOR %s Endwert", s.VarName))

	loopStartPC := g.pc
	loopStartLabel := fmt.Sprintf("__for_start_%s_%d", s.VarName, loopStartPC)
	loopEndLabel := fmt.Sprintf("__for_end_%s_%d", s.VarName, loopStartPC)
	g.labels[loopStartLabel] = loopStartPC

	// 3. Schleifenbedingung: Var <= End
	// Berechne End - Var >= 0
	// 2er Komplement von Var: ~Var + 1
	g.emit(OpLDAM, varAddr, "Pruefe FOR Limit")
	g.emit(OpNOT, 0, "~Var")
	g.emit(OpADD, 1, "~Var + 1")
	patchAddr := g.pc + 5
	g.emit(OpSTAM, patchAddr, "Patch Limit Addition")
	g.emit(OpLDAM, endAddr, "Lade Endwert")
	g.emit(OpADD, 0, "Endwert - Var")
	// Wenn Negativ (Bit 15 gesetzt) -> Schleife beenden
	g.emit(OpBN, 0, "FOR Beenden wenn Var > End")
	g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: loopEndLabel})

	// 4. Schleifenrumpf (Body)
	for _, bStmt := range s.Body {
		if err := g.compileStmt(bStmt); err != nil {
			return err
		}
	}

	// 5. Schrittweite addieren (Inkrement): Var = Var + Step
	stepVal := 1
	if num, ok := s.Step.(*ast.NumberLiteral); ok {
		stepVal = num.Value
	}
	g.emit(OpLDAM, varAddr, "Lade Zaehler")
	g.emit(OpADD, uint16(stepVal), fmt.Sprintf("Zaehler + %d", stepVal))
	g.emit(OpSTAM, varAddr, "Speichere Zaehler")

	// Sprung zurück zum Schleifenanfang
	g.emit(OpBA, 0, "FOR Loop Sprung")
	g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: loopStartLabel})

	g.labels[loopEndLabel] = g.pc
	return nil
}

func (g *CodeGenerator) compileRead(s *ast.ReadStmt) error {
	ptrAddr := g.getOrCreateVar("__data_ptr")

	for _, vName := range s.VarNames {
		vAddr := g.getOrCreateVar(vName)

		// Indirekter Zugriff: Lade [__data_ptr] über Pointer
		g.emit(OpLDAMM, ptrAddr, fmt.Sprintf("READ %s", vName))
		g.emit(OpSTAM, vAddr, fmt.Sprintf("Speichere in %s", vName))

		// Inkrementiere Pointer: __data_ptr++
		g.emit(OpLDAM, ptrAddr, "Lade __data_ptr")
		g.emit(OpADD, 1, "__data_ptr + 1")
		g.emit(OpSTAM, ptrAddr, "Aktualisiere __data_ptr")
	}
	return nil
}

// ============================================================================
// AUSDRÜCKE (EXPRESSIONS) IN ACCU KOMPILIEREN
// ============================================================================

func (g *CodeGenerator) compileExpr(expr ast.Expr) error {
	switch e := expr.(type) {
	case *ast.NumberLiteral:
		g.emit(OpLDA, uint16(e.Value), fmt.Sprintf("Konstante %d", e.Value))
		return nil

	case *ast.VariableExpr:
		addr := g.getOrCreateVar(e.Name)
		g.emit(OpLDAM, addr, fmt.Sprintf("Variable %s", e.Name))
		return nil

	case *ast.UnaryExpr:
		if err := g.compileExpr(e.Right); err != nil {
			return err
		}
		if e.Op == "-" {
			// Negation: ~Accu + 1
			g.emit(OpNOT, 0, "Negation: NOT")
			g.emit(OpADD, 1, "Negation: + 1")
		} else if e.Op == "NOT" {
			g.emit(OpNOT, 0, "Bitweises NOT")
		}
		return nil

	case *ast.BinaryExpr:
		return g.compileBinary(e)

	case *ast.PeekExpr:
		if err := g.compileExpr(e.Address); err != nil {
			return err
		}
		patchLoad := g.pc + 3
		g.emit(OpSTAM, patchLoad, "Patch PEEK Quelladresse")
		g.emit(OpLDAM, 0, "PEEK [dynamisch]")
		return nil

	case *ast.SqrExpr:
		g.needSqr = true
		if err := g.compileExpr(e.Value); err != nil {
			return err
		}
		g.emit(OpSTAM, g.getOrCreateVar("__r0"), "Radikand __r0")

		if e.Decimals != nil {
			if err := g.compileExpr(e.Decimals); err != nil {
				return err
			}
		} else {
			g.emit(OpLDA, 0, "Default 0 Nachkommastellen")
		}
		g.emit(OpSTAM, g.getOrCreateVar("__r1"), "Nachkommastellen __r1")

		g.emit(OpCALL, 0, "CALL __sqr")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__runtime_sqr"})
		return nil

	default:
		return fmt.Errorf("Unbekannter Ausdruckstyp: %T", expr)
	}
}

func (g *CodeGenerator) compileBinary(e *ast.BinaryExpr) error {
	op := strings.ToUpper(e.Op)

	// Spezialfall: Multiplikation (*)
	if op == "*" {
		g.needMul = true
		if err := g.compileExpr(e.Left); err != nil {
			return err
		}
		g.emit(OpSTAM, g.getOrCreateVar("__r0"), "Multiplikand __r0")

		if err := g.compileExpr(e.Right); err != nil {
			return err
		}
		g.emit(OpSTAM, g.getOrCreateVar("__r1"), "Multiplikator __r1")

		g.emit(OpCALL, 0, "CALL __mul")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__runtime_mul"})
		return nil
	}

	// Spezialfall: Division (/) oder Modulo (MOD)
	if op == "/" || op == "MOD" {
		g.needDiv = true
		if err := g.compileExpr(e.Left); err != nil {
			return err
		}
		g.emit(OpSTAM, g.getOrCreateVar("__r0"), "Dividend __r0")

		if err := g.compileExpr(e.Right); err != nil {
			return err
		}
		g.emit(OpSTAM, g.getOrCreateVar("__r1"), "Divisor __r1")

		g.emit(OpCALL, 0, "CALL __div")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__runtime_div"})

		if op == "MOD" {
			g.emit(OpLDAM, g.getOrCreateVar("__rem"), "Lade Divisionsrest")
		}
		return nil
	}

	// Schnelle Optimierung für unmittelbare Zahlenkonstanten (+, -, AND, OR)
	if num, ok := e.Right.(*ast.NumberLiteral); ok {
		if op == "+" {
			if err := g.compileExpr(e.Left); err != nil {
				return err
			}
			g.emit(OpADD, uint16(num.Value&0xFFFF), fmt.Sprintf("Accu + %d", num.Value))
			return nil
		} else if op == "-" {
			if err := g.compileExpr(e.Left); err != nil {
				return err
			}
			g.emit(OpADD, uint16((-num.Value)&0xFFFF), fmt.Sprintf("Accu - %d", num.Value))
			return nil
		} else if op == "AND" {
			if err := g.compileExpr(e.Left); err != nil {
				return err
			}
			g.emit(OpAND, uint16(num.Value&0xFFFF), fmt.Sprintf("Accu AND 0x%04X", num.Value))
			return nil
		} else if op == "OR" {
			if err := g.compileExpr(e.Left); err != nil {
				return err
			}
			g.emit(OpOR, uint16(num.Value&0xFFFF), fmt.Sprintf("Accu OR 0x%04X", num.Value))
			return nil
		}
	}

	// Rechte Seite in temporäre Zelle evaluieren
	if err := g.compileExpr(e.Right); err != nil {
		return err
	}
	tempRight := g.allocTemp()
	g.emit(OpSTAM, tempRight, "Rechter Operand")

	// Linke Seite in Accu evaluieren
	if err := g.compileExpr(e.Left); err != nil {
		return err
	}

	// Operation auf Accu ausführen
	switch op {
	case "+":
		// Accu = Accu + [tempRight]
		tempA := g.allocTemp()
		g.emit(OpSTAM, tempA, "Temp Accu sichern")
		g.emit(OpLDAM, tempRight, "Lade B")
		patchAddr := g.pc + 5
		g.emit(OpSTAM, patchAddr, "Patch ADD Operand")
		g.emit(OpLDAM, tempA, "Lade A zurueck")
		g.emit(OpADD, 0, "A + B")

	case "-":
		// Accu = Accu - [tempRight] = A + (~B + 1)
		tempA := g.allocTemp()
		g.emit(OpSTAM, tempA, "Temp A sichern")
		g.emit(OpLDAM, tempRight, "Lade B")
		g.emit(OpNOT, 0, "~B")
		g.emit(OpADD, 1, "~B + 1 (-B)")
		patchAddr := g.pc + 5
		g.emit(OpSTAM, patchAddr, "Patch ADD Operand mit -B")
		g.emit(OpLDAM, tempA, "Lade A zurueck")
		g.emit(OpADD, 0, "A + (-B)")

	case "AND":
		g.emit(OpAND, 0xFFFF, "AND Operation")

	case "OR":
		g.emit(OpOR, 0x0000, "OR Operation")

	case "SHL":
		g.emit(OpSHL, 0, "SHL Linksshift")

	case "SHR":
		g.emit(OpSHR, 0, "SHR Rechtsshift")

	case "=", "==":
		// A == B: Subtrahiere A - B, wenn Zero -> 1, sonst 0
		return g.compileComparison(tempRight, OpBZ)

	case "<>", "!=":
		// A <> B: Wenn Zero -> 0, sonst 1
		return g.compileComparison(tempRight, OpBZ, true)

	case "<":
		// A < B: Wenn Negative -> 1, sonst 0
		return g.compileComparison(tempRight, OpBN)

	case ">=":
		// A >= B: Wenn Negative -> 0, sonst 1
		return g.compileComparison(tempRight, OpBN, true)

	case ">":
		// A > B: B - A < 0 (Negative) -> 1, sonst 0
		return g.compileComparisonReverse(tempRight, OpBN)

	case "<=":
		// A <= B: B - A < 0 -> 0, sonst 1
		return g.compileComparisonReverse(tempRight, OpBN, true)
	}

	return nil
}

func (g *CodeGenerator) compileComparison(tempRight uint16, branchOp uint16, invert ...bool) error {
	// A liegt im Accu. Berechne A - B
	tempA := g.allocTemp()
	g.emit(OpSTAM, tempA, "Sichere A")
	g.emit(OpLDAM, tempRight, "Lade B")
	g.emit(OpNOT, 0, "~B")
	g.emit(OpADD, 1, "-B")
	patchAddr := g.pc + 5
	g.emit(OpSTAM, patchAddr, "Patch -B")
	g.emit(OpLDAM, tempA, "Lade A")
	g.emit(OpADD, 0, "A - B")

	trueLabel := fmt.Sprintf("__comp_true_%d", g.pc)
	endLabel := fmt.Sprintf("__comp_end_%d", g.pc)

	inv := len(invert) > 0 && invert[0]

	if !inv {
		g.emit(branchOp, 0, "Branch Bedingung erfuellt")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: trueLabel})
		g.emit(OpLDA, 0, "Ergebnis = 0 (False)")
		g.emit(OpBA, 0, "Sprung zu Comp-Ende")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: endLabel})

		g.labels[trueLabel] = g.pc
		g.emit(OpLDA, 1, "Ergebnis = 1 (True)")
		g.labels[endLabel] = g.pc
	} else {
		g.emit(branchOp, 0, "Branch invertiert")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: trueLabel})
		g.emit(OpLDA, 1, "Ergebnis = 1 (True)")
		g.emit(OpBA, 0, "Sprung zu Comp-Ende")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: endLabel})

		g.labels[trueLabel] = g.pc
		g.emit(OpLDA, 0, "Ergebnis = 0 (False)")
		g.labels[endLabel] = g.pc
	}

	return nil
}

func (g *CodeGenerator) compileComparisonReverse(tempRight uint16, branchOp uint16, invert ...bool) error {
	// A liegt im Accu. Berechne B - A
	tempA := g.allocTemp()
	g.emit(OpSTAM, tempA, "Sichere A")
	g.emit(OpNOT, 0, "~A")
	g.emit(OpADD, 1, "-A")
	patchAddr := g.pc + 5
	g.emit(OpSTAM, patchAddr, "Patch -A")
	g.emit(OpLDAM, tempRight, "Lade B")
	g.emit(OpADD, 0, "B - A")

	trueLabel := fmt.Sprintf("__comp_rev_true_%d", g.pc)
	endLabel := fmt.Sprintf("__comp_rev_end_%d", g.pc)

	inv := len(invert) > 0 && invert[0]

	if !inv {
		g.emit(branchOp, 0, "Branch Bedingung erfuellt")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: trueLabel})
		g.emit(OpLDA, 0, "Ergebnis = 0 (False)")
		g.emit(OpBA, 0, "Sprung zu Comp-Ende")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: endLabel})

		g.labels[trueLabel] = g.pc
		g.emit(OpLDA, 1, "Ergebnis = 1 (True)")
		g.labels[endLabel] = g.pc
	} else {
		g.emit(branchOp, 0, "Branch invertiert")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: trueLabel})
		g.emit(OpLDA, 1, "Ergebnis = 1 (True)")
		g.emit(OpBA, 0, "Sprung zu Comp-Ende")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: endLabel})

		g.labels[trueLabel] = g.pc
		g.emit(OpLDA, 0, "Ergebnis = 0 (False)")
		g.labels[endLabel] = g.pc
	}

	return nil
}

// ============================================================================
// RUNTIME-BIBLIOTHEK (MULTIPLIKATION, DIVISION, DISPLAY)
// ============================================================================

func (g *CodeGenerator) emitRuntimeRoutines() {
	// 1. Multiplikation __mul (__r0 * __r1 -> Accu)
	if g.needMul {
		g.labels["__runtime_mul"] = g.pc
		r0Addr := g.getOrCreateVar("__r0")
		r1Addr := g.getOrCreateVar("__r1")
		prodAddr := g.getOrCreateVar("__prod")

		g.emit(OpLDA, 0, "__mul: Initialisiere Produkt = 0")
		g.emit(OpSTAM, prodAddr, "prod = 0")

		loopPC := g.pc
		g.labels["__mul_loop"] = loopPC

		g.emit(OpLDAM, r1Addr, "__mul: Lade Faktor 2")
		g.emit(OpBZ, 0, "__mul: Wenn Faktor 2 == 0 -> Fertig")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__mul_done"})

		// Prüfe LSB(r1) mit BE (Even)
		g.emit(OpBE, 0, "__mul: Wenn Even -> Skip Addition")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__mul_skip_add"})

		// Addiere r0 zu prod
		g.emit(OpLDAM, r0Addr, "__mul: Lade r0")
		patchAdd := g.pc + 5
		g.emit(OpSTAM, patchAdd, "Patch ADD")
		g.emit(OpLDAM, prodAddr, "Lade prod")
		g.emit(OpADD, 0, "prod + r0")
		g.emit(OpSTAM, prodAddr, "Aktualisiere prod")

		g.labels["__mul_skip_add"] = g.pc
		// Shift r0 links, r1 rechts
		g.emit(OpLDAM, r0Addr, "Shift r0 links")
		g.emit(OpSHL, 0, "r0 << 1")
		g.emit(OpSTAM, r0Addr, "Speichere r0")

		g.emit(OpLDAM, r1Addr, "Shift r1 rechts")
		g.emit(OpSHR, 0, "r1 >> 1")
		g.emit(OpSTAM, r1Addr, "Speichere r1")

		g.emit(OpBA, loopPC, "Loop __mul")

		g.labels["__mul_done"] = g.pc
		g.emit(OpLDAM, prodAddr, "__mul: Rueckgabe Produkt")
		g.emitSingle(OpRET, "RETURN __mul")
	}

	// 2. Division __div (__r0 / __r1 -> Accu, Rest in __rem)
	if g.needDiv {
		g.labels["__runtime_div"] = g.pc
		r0Addr := g.getOrCreateVar("__r0")
		r1Addr := g.getOrCreateVar("__r1")
		quotAddr := g.getOrCreateVar("__quot")
		remAddr := g.getOrCreateVar("__rem")

		g.emit(OpLDA, 0, "__div: Initialisiere Quotient = 0")
		g.emit(OpSTAM, quotAddr, "quot = 0")
		g.emit(OpLDAM, r0Addr, "Lade Dividend in Remainder")
		g.emit(OpSTAM, remAddr, "rem = dividend")

		divLoopPC := g.pc
		g.labels["__div_loop"] = divLoopPC

		// Berechne rem - divisor
		g.emit(OpLDAM, r1Addr, "Lade Divisor")
		g.emit(OpNOT, 0, "~Divisor")
		g.emit(OpADD, 1, "-Divisor")
		patchDiv := g.pc + 5
		g.emit(OpSTAM, patchDiv, "Patch Divisor")
		g.emit(OpLDAM, remAddr, "Lade rem")
		g.emit(OpADD, 0, "rem - Divisor")

		// Wenn negativ -> Divisor passt nicht mehr hinein -> Fertig
		g.emit(OpBN, 0, "__div: Fertig wenn rem < Divisor")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__div_done"})

		// Sonst: rem = rem - divisor, quot++
		g.emit(OpSTAM, remAddr, "Aktualisiere rem")
		g.emit(OpLDAM, quotAddr, "Lade quot")
		g.emit(OpADD, 1, "quot++")
		g.emit(OpSTAM, quotAddr, "Speichere quot")
		g.emit(OpBA, divLoopPC, "Loop __div")

		g.labels["__div_done"] = g.pc
		g.emit(OpLDAM, quotAddr, "__div: Rueckgabe Quotient")
		g.emitSingle(OpRET, "RETURN __div")
	}

	// 2b. Quadratwurzel __sqr (SQR/SQRT Radikand __r0 mit __r1 Nachkommastellen -> Accu)
	if g.needSqr {
		g.labels["__runtime_sqr"] = g.pc
		r0Addr := g.getOrCreateVar("__r0")
		r1Addr := g.getOrCreateVar("__r1")
		sqrR := g.getOrCreateVar("__sqr_r")
		sqrRem := g.getOrCreateVar("__sqr_rem")
		sqrTrial := g.getOrCreateVar("__sqr_trial")
		sqrDec := g.getOrCreateVar("__sqr_dec")
		sqrD := g.getOrCreateVar("__sqr_d")
		sqrDelta := g.getOrCreateVar("__sqr_delta")
		sqrTemp := g.getOrCreateVar("__sqr_temp")

		// 1. Wenn Radikand negativ (Bit 15 == 1) -> Rueckgabe 0
		g.emit(OpLDAM, r0Addr, "__sqr: Lade Radikand")
		g.emit(OpBN, 0, "__sqr: Wenn negativ -> return 0")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__sqr_neg"})

		// Initialisierung: R = 0, rem = Radikand
		g.emit(OpLDA, 0, "__sqr: Init R = 0")
		g.emit(OpSTAM, sqrR, "R = 0")
		g.emit(OpLDAM, r0Addr, "Lade Radikand")
		g.emit(OpSTAM, sqrRem, "rem = radikand")

		// 2. 8-Schritt Bit-by-Bit Integer-Wurzel (k = 7 abwaerts bis 0)
		// Step k = 7 (b = 128, trial = 16384, da R = 0)
		g.emit(OpLDAM, sqrRem, "__sqr: k=7 rem - 16384")
		g.emit(OpADD, uint16(49152), "rem + (-16384)")
		skip7 := fmt.Sprintf("__sqr_skip_step_%d", g.pc)
		g.emit(OpBN, 0, "__sqr: k=7 skip wenn negativ")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: skip7})
		g.emit(OpSTAM, sqrRem, "Speichere rem")
		g.emit(OpLDA, 128, "Setze Bit 7 in R")
		g.emit(OpSTAM, sqrR, "R = 128")
		g.labels[skip7] = g.pc

		// Steps k = 6 abwaerts bis 0:
		// trial = (R << (k+1)) + (1 << (2*k))
		for k := 6; k >= 0; k-- {
			// trial berechnen: Lade R, shift (k+1) mal links, addiere 1<<(2*k)
			g.emit(OpLDAM, sqrR, fmt.Sprintf("__sqr: k=%d Lade R", k))
			for s := 0; s < k+1; s++ {
				g.emit(OpSHL, 0, "R << 1")
			}
			g.emit(OpADD, uint16(1<<(2*k)), fmt.Sprintf("+ %d", 1<<(2*k)))
			g.emit(OpSTAM, sqrTrial, "Speichere trial")

			// rem - trial berechnen via 2er-Komplement (~trial + 1)
			g.emit(OpNOT, 0, "~trial")
			g.emit(OpADD, 1, "-trial")
			patchSub := g.pc + 5
			g.emit(OpSTAM, patchSub, "Patch Subtrahend")
			g.emit(OpLDAM, sqrRem, "Lade rem")
			g.emit(OpADD, 0, "rem - trial")

			skipK := fmt.Sprintf("__sqr_skip_step_%d", g.pc)
			g.emit(OpBN, 0, fmt.Sprintf("__sqr: k=%d skip wenn negativ", k))
			g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: skipK})

			// rem = rem - trial
			g.emit(OpSTAM, sqrRem, "Speichere neues rem")
			// R = R + (1 << k)
			g.emit(OpLDAM, sqrR, "Lade R")
			g.emit(OpADD, uint16(1<<k), fmt.Sprintf("R + %d", 1<<k))
			g.emit(OpSTAM, sqrR, "Speichere R")
			g.labels[skipK] = g.pc
		}

		// 3. Nachkommastellen (Festkomma)
		// Pruefe ob __r1 == 0 -> wenn ja, sofort fertig!
		g.emit(OpLDAM, r1Addr, "__sqr: Lade Nachkommastellen D")
		g.emit(OpBZ, 0, "__sqr: Wenn D == 0 -> Fertig")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__sqr_done"})
		g.emit(OpSTAM, sqrDec, "Speichere Dec-Zaehler")

		decLoopPC := g.pc
		g.labels[fmt.Sprintf("__sqr_dec_loop_%d", decLoopPC)] = decLoopPC

		// a) rem = rem * 100 (2x * 10)
		for pass := 0; pass < 2; pass++ {
			// temp = rem << 1
			g.emit(OpLDAM, sqrRem, "Lade rem fuer * 10")
			g.emit(OpSHL, 0, "rem << 1")
			g.emit(OpSTAM, sqrTemp, "temp = rem * 2")

			// patch upcoming ADD with temp
			patchMul10 := g.pc + 11
			g.emit(OpSTAM, patchMul10, "Patch ADD mit temp")

			// Accu = rem << 3 (rem * 8)
			g.emit(OpLDAM, sqrRem, "Lade rem")
			g.emit(OpSHL, 0, "rem << 1")
			g.emit(OpSHL, 0, "rem << 2")
			g.emit(OpSHL, 0, "rem << 3")
			g.emit(OpADD, 0, "rem*8 + rem*2 = rem*10")
			g.emit(OpSTAM, sqrRem, "Speichere rem*10")
		}

		// b) delta = 20 * R + 1
		// Berechne 10 * R:
		g.emit(OpLDAM, sqrR, "Lade R fuer 20*R")
		g.emit(OpSHL, 0, "R << 1")
		g.emit(OpSTAM, sqrTemp, "temp = R * 2")
		patchMulR := g.pc + 11
		g.emit(OpSTAM, patchMulR, "Patch ADD mit temp")
		g.emit(OpLDAM, sqrR, "Lade R")
		g.emit(OpSHL, 0, "R << 1")
		g.emit(OpSHL, 0, "R << 2")
		g.emit(OpSHL, 0, "R << 3")
		g.emit(OpADD, 0, "R*8 + R*2 = 10*R")
		g.emit(OpSHL, 0, "10*R * 2 = 20*R")
		g.emit(OpADD, 1, "20*R + 1")
		g.emit(OpSTAM, sqrDelta, "Speichere delta = 20*R + 1")

		// c) Init d = 0
		g.emit(OpLDA, 0, "Init d = 0")
		g.emit(OpSTAM, sqrD, "d = 0")

		// d) Ziffernbestimmungs-Schleife (d_loop)
		dLoopPC := g.pc
		g.labels[fmt.Sprintf("__sqr_d_loop_%d", dLoopPC)] = dLoopPC

		// rem - delta berechnen
		g.emit(OpLDAM, sqrDelta, "Lade delta")
		g.emit(OpNOT, 0, "~delta")
		g.emit(OpADD, 1, "-delta")
		patchDSub := g.pc + 5
		g.emit(OpSTAM, patchDSub, "Patch delta-Subtraktion")
		g.emit(OpLDAM, sqrRem, "Lade rem")
		g.emit(OpADD, 0, "rem - delta")

		dDoneLabel := fmt.Sprintf("__sqr_d_done_%d", g.pc)
		g.emit(OpBN, 0, "Wenn rem < delta -> Ziffer fertig")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: dDoneLabel})

		// rem = rem - delta
		g.emit(OpSTAM, sqrRem, "rem = rem - delta")
		// delta = delta + 2
		g.emit(OpLDAM, sqrDelta, "Lade delta")
		g.emit(OpADD, 2, "delta + 2")
		g.emit(OpSTAM, sqrDelta, "Speichere delta")
		// d = d + 1
		g.emit(OpLDAM, sqrD, "Lade d")
		g.emit(OpADD, 1, "d + 1")
		g.emit(OpSTAM, sqrD, "Speichere d")
		g.emit(OpBA, dLoopPC, "Loop d_loop")

		// e) Ziffer d fertig: R = R * 10 + d
		g.labels[dDoneLabel] = g.pc
		g.emit(OpLDAM, sqrR, "Lade R fuer R*10")
		g.emit(OpSHL, 0, "R << 1")
		g.emit(OpSTAM, sqrTemp, "temp = R * 2")
		patchNewR := g.pc + 11
		g.emit(OpSTAM, patchNewR, "Patch ADD mit temp")
		g.emit(OpLDAM, sqrR, "Lade R")
		g.emit(OpSHL, 0, "R << 1")
		g.emit(OpSHL, 0, "R << 2")
		g.emit(OpSHL, 0, "R << 3")
		g.emit(OpADD, 0, "R*10")
		g.emit(OpSTAM, sqrTemp, "temp = R * 10")

		// Addiere d zu R*10
		patchAddD := g.pc + 5
		g.emit(OpSTAM, patchAddD, "Patch ADD mit R*10")
		g.emit(OpLDAM, sqrD, "Lade d")
		g.emit(OpADD, 0, "R*10 + d")
		g.emit(OpSTAM, sqrR, "Speichere neues R")

		// f) Dekrementiere Dezimalstellen-Zaehler
		g.emit(OpLDAM, sqrDec, "Lade Dec-Counter")
		g.emit(OpADD, uint16(0xFFFF), "Dec - 1")
		g.emit(OpSTAM, sqrDec, "Speichere Dec")
		g.emit(OpBZ, 0, "Wenn Dec == 0 -> Fertig")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__sqr_done"})

		// Overflow-Schutz fuer 16-Bit: wenn R >= 3276, keine weitere Nachkommastelle
		g.emit(OpLDAM, sqrR, "Pruefe Overflow R >= 3276")
		g.emit(OpADD, uint16((-3276)&0xFFFF), "R - 3276")
		g.emit(OpBN, 0, "Wenn R < 3276 -> Naechste Stelle")
		nextDecLabel := fmt.Sprintf("__sqr_next_dec_%d", g.pc)
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: nextDecLabel})
		g.emit(OpBA, 0, "R >= 3276 -> Vorzeitiges Ende")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__sqr_done"})

		g.labels[nextDecLabel] = g.pc
		g.emit(OpBA, decLoopPC, "Naechste Nachkommastelle")

		// g) Fertig & Rueckgabe
		g.labels["__sqr_done"] = g.pc
		g.emit(OpLDAM, sqrR, "__sqr: Rueckgabe Wurzel R in Accu")
		g.emitSingle(OpRET, "RETURN __sqr")

		// Negativeingabe -> 0
		g.labels["__sqr_neg"] = g.pc
		g.emit(OpLDA, 0, "__sqr: Negativeingabe Rueckgabe 0")
		g.emitSingle(OpRET, "RETURN __sqr (negativ)")
	}

	// 3. Display Löschen __cls (von __cls_ptr bis __cls_end_addr)
	if g.needCls {
		g.labels["__runtime_cls"] = g.pc
		ptrCls := g.getOrCreateVar("__cls_ptr")
		endCls := g.getOrCreateVar("__cls_end_addr")

		clsLoop := g.pc
		g.labels["__cls_loop"] = clsLoop

		g.emit(OpLDAM, ptrCls, "Lade Zeiger")
		patchCls := g.pc + 5
		g.emit(OpSTAM, patchCls, "Patch Zieladresse")
		g.emit(OpLDA, 0, "Pixeldaten = 0")
		g.emit(OpSTAM, 0, "Schreibe in Display")

		g.emit(OpLDAM, ptrCls, "Zeiger inkrementieren")
		g.emit(OpADD, 1, "ptr++")
		g.emit(OpSTAM, ptrCls, "Speichere ptr")

		// Prüfe ob ptr == endCls: berechne ptr - endCls
		g.emit(OpLDAM, endCls, "Lade endCls")
		g.emit(OpNOT, 0, "~endCls")
		g.emit(OpADD, 1, "~endCls + 1")
		patchEnd := g.pc + 5
		g.emit(OpSTAM, patchEnd, "Patch Endvergleich")
		g.emit(OpLDAM, ptrCls, "Lade ptr")
		g.emit(OpADD, 0, "ptr - endCls")
		g.emit(OpBZ, 0, "__cls: Fertig bei endCls")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__cls_done"})

		g.emit(OpBA, clsLoop, "Loop __cls")

		g.labels["__cls_done"] = g.pc
		g.emitSingle(OpRET, "RETURN __cls")
	}

	// 4. Pixel setzen/löschen __pset (PSET X, Y, Color) mit echtem Bit-Masking
	if g.needPset {
		g.labels["__runtime_pset"] = g.pc
		xAddr := g.getOrCreateVar("__pset_x")
		yAddr := g.getOrCreateVar("__pset_y")
		cAddr := g.getOrCreateVar("__pset_c")
		wordAddr := g.getOrCreateVar("__pset_word_addr")
		maskAddr := g.getOrCreateVar("__pset_mask")
		countAddr := g.getOrCreateVar("__pset_count")
		currWordAddr := g.getOrCreateVar("__pset_curr_word")

		// 1. Berechne WordAddr = DisplayBase + (Y * 2) + WordOffset
		// Auf realer 32x32 Hardware:
		// Word 0 (Offset 0) = Rechte Hälfte (X = 16..31)
		// Word 1 (Offset 1) = Linke Hälfte  (X = 0..15)
		g.emit(OpLDAM, yAddr, "__pset: Lade Y")
		g.emit(OpSHL, 0, "Y * 2")
		g.emit(OpADD, DisplayBase+1, "Base + Y*2 + 1 (Default Word 1)")
		g.emit(OpSTAM, wordAddr, "Speichere wordAddr")

		// Prüfe X >= 16: Wenn X < 16 bleibt Word 1, wenn X >= 16 ziehe 1 ab -> Word 0
		g.emit(OpLDAM, xAddr, "Lade X")
		g.emit(OpAND, 0x0010, "X >= 16 ?")
		g.emit(OpBZ, 0, "Wenn X < 16 -> Word 1 bereit")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__pset_word_ready"})

		// X >= 16: Ziehe 1 ab (ADD 0xFFFF) -> Word 0
		g.emit(OpLDAM, wordAddr, "Lade wordAddr")
		g.emit(OpADD, 0xFFFF, "wordAddr - 1 -> Word 0")
		g.emit(OpSTAM, wordAddr, "Speichere Word 0")

		g.labels["__pset_word_ready"] = g.pc

		// 2. Berechne Bitmaske = 0x8000 >> (X & 15)
		g.emit(OpLDA, 0x8000, "Init Maske = 0x8000 (Bit 15)")
		g.emit(OpSTAM, maskAddr, "Speichere Startmaske")
		g.emit(OpLDAM, xAddr, "Lade X")
		g.emit(OpAND, 0x000F, "X & 15 (Bit-Index)")
		g.emit(OpSTAM, countAddr, "Speichere Shift-Count")

		// Mask-Shift-Schleife
		maskLoop := g.pc
		g.emit(OpLDAM, countAddr, "Lade Shift-Count")
		g.emit(OpBZ, 0, "Fertig wenn Count == 0")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__pset_mask_ready"})
		g.emit(OpADD, 0xFFFF, "Count - 1")
		g.emit(OpSTAM, countAddr, "Speichere Count")
		g.emit(OpLDAM, maskAddr, "Lade Maske")
		g.emit(OpSHR, 0, "Maske >> 1")
		g.emit(OpSTAM, maskAddr, "Speichere Maske")
		g.emit(OpBA, maskLoop, "Loop Mask-Shift")

		g.labels["__pset_mask_ready"] = g.pc

		// 3. Lade aktuellen Bildschirminhalt an wordAddr
		g.emit(OpLDAM, wordAddr, "Lade wordAddr")
		patchRead := g.pc + 3
		g.emit(OpSTAM, patchRead, "Patch Leseadresse")
		g.emit(OpLDAM, 0, "Lade aktuellen Wortwert")
		g.emit(OpSTAM, currWordAddr, "Speichere currWord")

		// 4. Prüfe Color: Wenn Color == 0 -> Pixel löschen, sonst Pixel setzen
		g.emit(OpLDAM, cAddr, "Lade Color")
		g.emit(OpBZ, 0, "Wenn Color == 0 -> Clear Pixel")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__pset_clear"})

		// SET PIXEL: currWord | mask
		g.emit(OpLDAM, maskAddr, "Lade Maske")
		patchOr := g.pc + 5
		g.emit(OpSTAM, patchOr, "Patch OR-Operand")
		g.emit(OpLDAM, currWordAddr, "Lade currWord")
		g.emit(OpOR, 0, "currWord | mask")
		g.emit(OpSTAM, currWordAddr, "Speichere neues currWord")
		g.emit(OpBA, 0, "Sprung zu Write")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__pset_write"})

		// CLEAR PIXEL: currWord & ~mask
		g.labels["__pset_clear"] = g.pc
		g.emit(OpLDAM, maskAddr, "Lade Maske")
		g.emit(OpNOT, 0, "~mask (invertiert)")
		patchAnd := g.pc + 5
		g.emit(OpSTAM, patchAnd, "Patch AND-Operand")
		g.emit(OpLDAM, currWordAddr, "Lade currWord")
		g.emit(OpAND, 0, "currWord & ~mask")
		g.emit(OpSTAM, currWordAddr, "Speichere neues currWord")

		// 5. Schreibe modifiziertes Wort zurück in Display
		g.labels["__pset_write"] = g.pc
		g.emit(OpLDAM, wordAddr, "Lade wordAddr")
		patchWrite := g.pc + 5
		g.emit(OpSTAM, patchWrite, "Patch Schreibadresse")
		g.emit(OpLDAM, currWordAddr, "Lade geändertes Wort")
		g.emit(OpSTAM, 0, "Schreibe Pixelwort in Display")

		g.emitSingle(OpRET, "RETURN __pset")
	}

	// 5. Bresenham Linie zeichnen __line (LINE X1, Y1 TO X2, Y2, Color)
	if g.needLine {
		g.labels["__runtime_line"] = g.pc
		x1Addr := g.getOrCreateVar("__line_x1")
		y1Addr := g.getOrCreateVar("__line_y1")
		x2Addr := g.getOrCreateVar("__line_x2")
		y2Addr := g.getOrCreateVar("__line_y2")
		cAddr := g.getOrCreateVar("__line_c")

		dxAddr := g.getOrCreateVar("__line_dx")
		dyAddr := g.getOrCreateVar("__line_dy")
		sxAddr := g.getOrCreateVar("__line_sx")
		syAddr := g.getOrCreateVar("__line_sy")
		errAddr := g.getOrCreateVar("__line_err")
		e2Addr := g.getOrCreateVar("__line_e2")

		wordAddr := g.getOrCreateVar("__line_word_addr")
		maskAddr := g.getOrCreateVar("__line_mask")
		countAddr := g.getOrCreateVar("__line_count")
		currWordAddr := g.getOrCreateVar("__line_curr_word")

		// 1. Berechne dx und sx:
		// Berechne x2 - x1: (~x1 + 1) + x2
		g.emit(OpLDAM, x1Addr, "__line: Lade x1")
		g.emit(OpNOT, 0, "~x1")
		g.emit(OpADD, 1, "~x1 + 1")
		patchX2 := g.pc + 5
		g.emit(OpSTAM, patchX2, "Patch x2 Subtraktion")
		g.emit(OpLDAM, x2Addr, "Lade x2")
		g.emit(OpADD, 0, "x2 - x1")

		// Wenn x2 - x1 < 0 (Bit 15 gesetzt) -> x1 > x2
		g.emit(OpBN, 0, "Wenn x2 < x1 -> sx=-1")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__line_x_neg"})

		// x1 <= x2: sx = 1, dx = x2 - x1
		g.emit(OpSTAM, dxAddr, "dx = x2 - x1")
		g.emit(OpLDA, 1, "sx = 1")
		g.emit(OpSTAM, sxAddr, "Speichere sx")
		g.emit(OpBA, 0, "Weiter zu Y-Berechnung")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__line_calc_y"})

		// x1 > x2: sx = -1 (0xFFFF), dx = -(x2 - x1) = ~Accu + 1
		g.labels["__line_x_neg"] = g.pc
		g.emit(OpNOT, 0, "~(x2 - x1)")
		g.emit(OpADD, 1, "dx = abs(x2 - x1)")
		g.emit(OpSTAM, dxAddr, "Speichere dx")
		g.emit(OpLDA, 0xFFFF, "sx = -1")
		g.emit(OpSTAM, sxAddr, "Speichere sx")

		// 2. Berechne dy und sy:
		g.labels["__line_calc_y"] = g.pc
		// Berechne y2 - y1: (~y1 + 1) + y2
		g.emit(OpLDAM, y1Addr, "Lade y1")
		g.emit(OpNOT, 0, "~y1")
		g.emit(OpADD, 1, "~y1 + 1")
		patchY2 := g.pc + 5
		g.emit(OpSTAM, patchY2, "Patch y2 Subtraktion")
		g.emit(OpLDAM, y2Addr, "Lade y2")
		g.emit(OpADD, 0, "y2 - y1")

		// Wenn y2 - y1 < 0 -> y1 > y2
		g.emit(OpBN, 0, "Wenn y2 < y1 -> sy=-1")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__line_y_neg"})

		// y1 <= y2: sy = 1, dy = -(y2 - y1)
		g.emit(OpNOT, 0, "~(y2 - y1)")
		g.emit(OpADD, 1, "dy = -abs(y2 - y1)")
		g.emit(OpSTAM, dyAddr, "Speichere dy")
		g.emit(OpLDA, 1, "sy = 1")
		g.emit(OpSTAM, syAddr, "Speichere sy")
		g.emit(OpBA, 0, "Weiter zu err-Berechnung")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__line_calc_err"})

		// y1 > y2: sy = -1 (0xFFFF), dy = y2 - y1 (bereits negativ)
		g.labels["__line_y_neg"] = g.pc
		g.emit(OpSTAM, dyAddr, "dy = y2 - y1 (negativ)")
		g.emit(OpLDA, 0xFFFF, "sy = -1")
		g.emit(OpSTAM, syAddr, "Speichere sy")

		// 3. Berechne err = dx + dy
		g.labels["__line_calc_err"] = g.pc
		g.emit(OpLDAM, dyAddr, "Lade dy")
		patchDy := g.pc + 5
		g.emit(OpSTAM, patchDy, "Patch dy Addition")
		g.emit(OpLDAM, dxAddr, "Lade dx")
		g.emit(OpADD, 0, "dx + dy")
		g.emit(OpSTAM, errAddr, "err = dx + dy")

		// 4. Bresenham Hauptschleife
		lineLoopPC := g.pc
		g.labels["__line_loop"] = lineLoopPC

		// 4a. Punkt (x1, y1) zeichnen
		// wordAddr = DisplayBase + (y1 * 2) + WordOffset
		// Auf realer 32x32 Hardware:
		// Word 0 (Offset 0) = Rechte Hälfte (x1 = 16..31)
		// Word 1 (Offset 1) = Linke Hälfte  (x1 = 0..15)
		lineWordReadyLabel := fmt.Sprintf("__line_word_ready_%d", g.pc)
		g.emit(OpLDAM, y1Addr, "Lade y1")
		g.emit(OpSHL, 0, "y1 * 2")
		g.emit(OpADD, DisplayBase+1, "DisplayBase + y1*2 + 1 (Default Word 1)")
		g.emit(OpSTAM, wordAddr, "Speichere wordAddr")

		// Prüfe x1 >= 16: Wenn x1 < 16 bleibt Word 1, wenn x1 >= 16 ziehe 1 ab -> Word 0
		g.emit(OpLDAM, x1Addr, "Lade x1")
		g.emit(OpAND, 0x0010, "x1 >= 16 ?")
		g.emit(OpBZ, 0, "Wenn x1 < 16 -> Word 1 bereit")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: lineWordReadyLabel})

		// x1 >= 16: Ziehe 1 ab -> Word 0
		g.emit(OpLDAM, wordAddr, "Lade wordAddr")
		g.emit(OpADD, 0xFFFF, "wordAddr - 1 -> Word 0")
		g.emit(OpSTAM, wordAddr, "Speichere Word 0")

		g.labels[lineWordReadyLabel] = g.pc

		// Bitmaske = 0x8000 >> (x1 & 15)
		g.emit(OpLDA, 0x8000, "Init Maske = 0x8000")
		g.emit(OpSTAM, maskAddr, "Speichere Startmaske")
		g.emit(OpLDAM, x1Addr, "Lade x1")
		g.emit(OpAND, 0x000F, "x1 & 15")
		g.emit(OpSTAM, countAddr, "Speichere Shift-Count")

		maskLoopPC := g.pc
		lineMaskLoopLabel := fmt.Sprintf("__line_mask_loop_%d", maskLoopPC)
		lineMaskReadyLabel := fmt.Sprintf("__line_mask_ready_%d", maskLoopPC)
		g.labels[lineMaskLoopLabel] = maskLoopPC

		g.emit(OpLDAM, countAddr, "Lade Shift-Count")
		g.emit(OpBZ, 0, "Fertig wenn Count == 0")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: lineMaskReadyLabel})
		g.emit(OpADD, 0xFFFF, "Count - 1")
		g.emit(OpSTAM, countAddr, "Speichere Count")
		g.emit(OpLDAM, maskAddr, "Lade Maske")
		g.emit(OpSHR, 0, "Maske >> 1")
		g.emit(OpSTAM, maskAddr, "Speichere Maske")
		g.emit(OpBA, maskLoopPC, "Loop Mask-Shift")

		g.labels[lineMaskReadyLabel] = g.pc

		// Lese aktuellen Wert an wordAddr
		g.emit(OpLDAM, wordAddr, "Lade wordAddr")
		patchLineRead := g.pc + 3
		g.emit(OpSTAM, patchLineRead, "Patch Leseadresse")
		g.emit(OpLDAM, 0, "Lade aktuellen Wortwert")
		g.emit(OpSTAM, currWordAddr, "Speichere currWord")

		// Color prüfen
		lineClearLabel := fmt.Sprintf("__line_clear_%d", g.pc)
		lineWriteLabel := fmt.Sprintf("__line_write_%d", g.pc)

		g.emit(OpLDAM, cAddr, "Lade Color")
		g.emit(OpBZ, 0, "Wenn Color == 0 -> Clear Pixel")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: lineClearLabel})

		// SET: currWord | mask
		g.emit(OpLDAM, maskAddr, "Lade Maske")
		patchLineOr := g.pc + 5
		g.emit(OpSTAM, patchLineOr, "Patch OR-Operand")
		g.emit(OpLDAM, currWordAddr, "Lade currWord")
		g.emit(OpOR, 0, "currWord | mask")
		g.emit(OpSTAM, currWordAddr, "Speichere neues currWord")
		g.emit(OpBA, 0, "Sprung zu Write")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: lineWriteLabel})

		// CLEAR: currWord & ~mask
		g.labels[lineClearLabel] = g.pc
		g.emit(OpLDAM, maskAddr, "Lade Maske")
		g.emit(OpNOT, 0, "~mask")
		patchLineAnd := g.pc + 5
		g.emit(OpSTAM, patchLineAnd, "Patch AND-Operand")
		g.emit(OpLDAM, currWordAddr, "Lade currWord")
		g.emit(OpAND, 0, "currWord & ~mask")
		g.emit(OpSTAM, currWordAddr, "Speichere neues currWord")

		// Write back
		g.labels[lineWriteLabel] = g.pc
		g.emit(OpLDAM, wordAddr, "Lade wordAddr")
		patchLineWrite := g.pc + 5
		g.emit(OpSTAM, patchLineWrite, "Patch Schreibadresse")
		g.emit(OpLDAM, currWordAddr, "Lade geändertes Wort")
		g.emit(OpSTAM, 0, "Schreibe Pixelwort in Display")

		// 4b. Prüfe ob Ziel (x1 == x2 && y1 == y2) erreicht ist
		lineStepLabel := fmt.Sprintf("__line_step_%d", g.pc)

		// Prüfe x1 == x2: Berechne x1 - x2: (~x2 + 1) + x1
		g.emit(OpLDAM, x2Addr, "Prüfe x1 == x2")
		g.emit(OpNOT, 0, "~x2")
		g.emit(OpADD, 1, "~x2 + 1")
		patchEqX := g.pc + 5
		g.emit(OpSTAM, patchEqX, "Patch x1 Vergleich")
		g.emit(OpLDAM, x1Addr, "Lade x1")
		g.emit(OpADD, 0, "x1 - x2")
		lineCheckYLabel := fmt.Sprintf("__line_check_y_%d", g.pc)
		g.emit(OpBZ, 0, "Wenn x1 == x2 -> Prüfe Y")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: lineCheckYLabel})
		g.emit(OpBA, 0, "x1 != x2 -> Mache Schritt")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: lineStepLabel})

		g.labels[lineCheckYLabel] = g.pc
		// Prüfe y1 == y2
		g.emit(OpLDAM, y2Addr, "Prüfe y1 == y2")
		g.emit(OpNOT, 0, "~y2")
		g.emit(OpADD, 1, "~y2 + 1")
		patchEqY := g.pc + 5
		g.emit(OpSTAM, patchEqY, "Patch y1 Vergleich")
		g.emit(OpLDAM, y1Addr, "Lade y1")
		g.emit(OpADD, 0, "y1 - y2")
		g.emit(OpBZ, 0, "Wenn auch y1 == y2 -> Fertig!")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__line_done"})

		// 4c. Schritt berechnen
		g.labels[lineStepLabel] = g.pc
		// e2 = err << 1
		g.emit(OpLDAM, errAddr, "Lade err")
		g.emit(OpSHL, 0, "err << 1")
		g.emit(OpSTAM, e2Addr, "Speichere e2")

		// Prüfe if e2 >= dy  (d.h. e2 - dy >= 0 => bit 15 == 0)
		// Berechne e2 - dy: (~dy + 1) + e2
		g.emit(OpLDAM, dyAddr, "Lade dy")
		g.emit(OpNOT, 0, "~dy")
		g.emit(OpADD, 1, "~dy + 1")
		patchSubDy := g.pc + 5
		g.emit(OpSTAM, patchSubDy, "Patch dy Subtraktion")
		g.emit(OpLDAM, e2Addr, "Lade e2")
		g.emit(OpADD, 0, "e2 - dy")
		lineSkipXStepLabel := fmt.Sprintf("__line_skip_x_%d", g.pc)
		g.emit(OpBN, 0, "Wenn e2 < dy -> Skip X-Step")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: lineSkipXStepLabel})

		// err = err + dy
		g.emit(OpLDAM, dyAddr, "Lade dy")
		patchAddDy := g.pc + 5
		g.emit(OpSTAM, patchAddDy, "Patch dy Addition")
		g.emit(OpLDAM, errAddr, "Lade err")
		g.emit(OpADD, 0, "err + dy")
		g.emit(OpSTAM, errAddr, "Aktualisiere err")

		// x1 = x1 + sx
		g.emit(OpLDAM, sxAddr, "Lade sx")
		patchAddSx := g.pc + 5
		g.emit(OpSTAM, patchAddSx, "Patch sx Addition")
		g.emit(OpLDAM, x1Addr, "Lade x1")
		g.emit(OpADD, 0, "x1 + sx")
		g.emit(OpSTAM, x1Addr, "Aktualisiere x1")

		g.labels[lineSkipXStepLabel] = g.pc

		// Prüfe if e2 <= dx (d.h. dx - e2 >= 0 => bit 15 == 0)
		// Berechne dx - e2: (~e2 + 1) + dx
		g.emit(OpLDAM, e2Addr, "Lade e2")
		g.emit(OpNOT, 0, "~e2")
		g.emit(OpADD, 1, "~e2 + 1")
		patchSubE2 := g.pc + 5
		g.emit(OpSTAM, patchSubE2, "Patch e2 Subtraktion")
		g.emit(OpLDAM, dxAddr, "Lade dx")
		g.emit(OpADD, 0, "dx - e2")
		lineSkipYStepLabel := fmt.Sprintf("__line_skip_y_%d", g.pc)
		g.emit(OpBN, 0, "Wenn e2 > dx -> Skip Y-Step")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: lineSkipYStepLabel})

		// err = err + dx
		g.emit(OpLDAM, dxAddr, "Lade dx")
		patchAddDx := g.pc + 5
		g.emit(OpSTAM, patchAddDx, "Patch dx Addition")
		g.emit(OpLDAM, errAddr, "Lade err")
		g.emit(OpADD, 0, "err + dx")
		g.emit(OpSTAM, errAddr, "Aktualisiere err")

		// y1 = y1 + sy
		g.emit(OpLDAM, syAddr, "Lade sy")
		patchAddSy := g.pc + 5
		g.emit(OpSTAM, patchAddSy, "Patch sy Addition")
		g.emit(OpLDAM, y1Addr, "Lade y1")
		g.emit(OpADD, 0, "y1 + sy")
		g.emit(OpSTAM, y1Addr, "Aktualisiere y1")

		g.labels[lineSkipYStepLabel] = g.pc

		// Nächste Iteration
		g.emit(OpBA, lineLoopPC, "Loop __line")

		// Fertig
		g.labels["__line_done"] = g.pc
		g.emitSingle(OpRET, "RETURN __line")
	}
}

func (g *CodeGenerator) emitDataTable() {
	g.dataPtr = g.pc
	g.labels["__data_table_start"] = g.dataPtr

	if len(g.dataTable) == 0 {
		return
	}

	// Setze __data_ptr Initialwert
	ptrAddr := g.getOrCreateVar("__data_ptr")
	g.memory[ptrAddr] = g.dataPtr

	for _, val := range g.dataTable {
		g.memory[g.pc] = uint16(val & 0xFFFF)
		g.comments[g.pc] = fmt.Sprintf("DATA %d", val)
		g.pc++
	}
}

// ============================================================================
// HILFSFUNKTIONEN: EMIT & SPEICHERVERWALTUNG
// ============================================================================

func (g *CodeGenerator) emit(op uint16, operand uint16, comment string) {
	if g.pc >= MaxMemorySize-1 {
		return
	}
	g.memory[g.pc] = op
	g.comments[g.pc] = comment
	g.pc++

	g.memory[g.pc] = operand
	g.comments[g.pc] = fmt.Sprintf("Operand #0x%04X (%d)", operand, operand)
	g.pc++
}

func (g *CodeGenerator) emitSingle(op uint16, comment string) {
	if g.pc >= MaxMemorySize {
		return
	}
	g.memory[g.pc] = op
	g.comments[g.pc] = comment
	g.pc++
}

func (g *CodeGenerator) getOrCreateVar(name string) uint16 {
	upper := strings.ToUpper(name)
	if addr, exists := g.symbols[upper]; exists {
		return addr
	}
	addr := g.nextVarAddr
	g.symbols[upper] = addr
	g.nextVarAddr++
	return addr
}

func (g *CodeGenerator) allocTemp() uint16 {
	addr := g.nextTempAddr
	g.nextTempAddr++
	if g.nextTempAddr >= DisplayBase {
		g.nextTempAddr = TempBaseAddr // Wrap around
	}
	return addr
}

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
	VarBaseAddr   = 920 // Variablen ab Adresse 920 (32 Variablen: 920..951)
	TempBaseAddr  = 952 // Temporäre Berechnungszellen ab 952 (8 Zellen: 952..959)
	DisplayBase   = 960 // Display-Bereich: 960..1023
)

// CodeGenerator übersetzt AST in Maschinencode.
type CodeGenerator struct {
	memory           [MaxMemorySize]uint16
	pc               uint16
	symbols          map[string]uint16       // Variablenname -> RAM-Adresse
	varTypes         map[string]ast.DataType // Variablenname -> Datentyp (INT16 / FIXED8)
	explicitVarTypes map[string]bool         // Wurde Variable explizit via DIM typisiert?
	nextVarAddr      uint16
	nextTempAddr     uint16

	displayBase  uint16
	varBaseAddr  uint16
	tempBaseAddr uint16
	resolution   int

	labels map[string]uint16 // Label-Name -> PC-Adresse
	fixups []labelFixup      // Unaufgelöste Sprungziele (Forward Jumps)

	// Runtime-Bedarf
	needMul     bool
	needDiv     bool
	needSqr     bool
	needFMulQ88 bool
	needFDivQ88 bool
	needFtoiQ88 bool
	needFAddF16 bool
	needFSubF16 bool
	needFMulF16 bool
	needFDivF16 bool
	needItofF16 bool
	needFtoiF16 bool
	needCls     bool
	needPset    bool
	needLine    bool

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
		symbols:          make(map[string]uint16),
		varTypes:         make(map[string]ast.DataType),
		explicitVarTypes: make(map[string]bool),
		displayBase:      DisplayBase,
		varBaseAddr:  VarBaseAddr,
		tempBaseAddr: TempBaseAddr,
		nextVarAddr:  VarBaseAddr,
		nextTempAddr: TempBaseAddr,
		resolution:   32,
		labels:       make(map[string]uint16),
		fixups:       []labelFixup{},
		dataTable:    []int{},
		forStack:     []forLoopContext{},
		comments:     make(map[uint16]string),
	}
}

// SetResolution konfiguriert die Speicheraufteilung anhand der Display-Auflösung (32x32 oder 64x64).
func (g *CodeGenerator) SetResolution(res int) {
	g.resolution = res
	if res == 64 {
		g.displayBase = 768
		g.varBaseAddr = 734
		g.tempBaseAddr = 756
		g.nextVarAddr = 734
		g.nextTempAddr = 756
	} else {
		g.displayBase = DisplayBase
		g.varBaseAddr = VarBaseAddr
		g.tempBaseAddr = TempBaseAddr
		g.nextVarAddr = VarBaseAddr
		g.nextTempAddr = TempBaseAddr
	}
}

// Generate übersetzt das Programm und liefert das fertige 1024-Wort-Speicherabbild.
func (g *CodeGenerator) Generate(prog *ast.Program) ([MaxMemorySize]uint16, map[uint16]string, error) {
	// Vorab-Scan nach Auflösung (z.B. SCREEN 64) und DATA-Anweisungen
	hasData := false
	for _, stmt := range prog.Statements {
		if sc, ok := stmt.(*ast.ScreenStmt); ok {
			g.SetResolution(sc.Resolution)
		}
		if _, ok := stmt.(*ast.DataStmt); ok {
			hasData = true
		}
	}

	// Start-Sprung über Boot-Bereich & Hardware-I/O-Register (Sprung über Addr 2=OUT, 3=IN zu Hauptprogramm)
	g.emit(OpBA, 0, "Initialer Sprung über Hardware I/O (Addr 2=OUT, 3=IN)")
	g.fixups = append(g.fixups, labelFixup{patchPC: 1, label: "__main_start"})

	// Hardware Register 2 & 3 reservieren
	g.comments[2] = "Hardware Output Register (8-Bit)"
	g.comments[3] = "Hardware Input Register (8-Bit)"

	// Hauptprogramm-Einstieg ab Adresse 4 (überspringt 2 & 3)
	g.pc = 4
	g.labels["__main_start"] = g.pc

	// Initialisiere __data_ptr nur wenn DATA existiert
	if hasData || len(g.dataTable) > 0 {
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

	if g.pc >= g.varBaseAddr {
		return g.memory, g.comments, fmt.Errorf("Programm zu groß: Code-Größe (%d Worte) überschreitet Variablenbereich ab Adresse %d", g.pc, g.varBaseAddr)
	}
	if g.nextVarAddr > g.tempBaseAddr {
		return g.memory, g.comments, fmt.Errorf("Zu viele Variablen: Variablen-Bereich (%d bis %d) überschreitet temporären Bereich ab Adresse %d", g.varBaseAddr, g.nextVarAddr, g.tempBaseAddr)
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

	case *ast.DimStmt:
		for _, vName := range s.VarNames {
			upper := strings.ToUpper(vName)
			g.varTypes[upper] = s.VarType
			g.explicitVarTypes[upper] = true
			g.getOrCreateVar(vName)
		}
		return nil

	case *ast.LetStmt:
		upper := strings.ToUpper(s.VarName)
		valType := g.exprType(s.Value)
		if !g.explicitVarTypes[upper] {
			if strings.HasSuffix(upper, "#") || valType == ast.TypeFloat16 {
				g.varTypes[upper] = ast.TypeFloat16
			} else if valType == ast.TypeFixed8 {
				g.varTypes[upper] = ast.TypeFixed8
			}
		}
		targetType := g.getVarType(s.VarName)
		varAddr := g.getOrCreateVar(s.VarName)

		switch targetType {
		case ast.TypeFloat16:
			if err := g.compileExprAsFloat16(s.Value); err != nil {
				return err
			}
		case ast.TypeFixed8:
			if err := g.compileExprAsFixed8(s.Value); err != nil {
				return err
			}
		default:
			if err := g.compileExpr(s.Value); err != nil {
				return err
			}
			if valType == ast.TypeFloat16 {
				g.emitF16ToInt()
			} else if valType == ast.TypeFixed8 {
				g.emitQ88ToInt()
			}
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
		// Spezialfall: Konstante Adresse (z.B. POKE 2, X oder POKE 960, 0)
		if addrVal, ok := evalConstExpr(s.Address); ok {
			if err := g.compileExpr(s.Value); err != nil {
				return err
			}
			g.emit(OpSTAM, uint16(addrVal&0xFFFF), fmt.Sprintf("POKE [%d] = Accu (direkt)", addrVal))
			return nil
		}

		// Dynamische Adresse: Lade Wert, speichere Adresse in Pointer, schreibe via STAMM
		if err := g.compileExpr(s.Value); err != nil {
			return err
		}
		valTemp := g.allocTemp()
		g.emit(OpSTAM, valTemp, "POKE Temp Wert")

		if err := g.compileExpr(s.Address); err != nil {
			return err
		}
		addrTemp := g.allocTemp()
		g.emit(OpSTAM, addrTemp, "POKE Zeigeradresse")
		g.emit(OpLDAM, valTemp, "Lade POKE Wert")
		g.emit(OpSTAMM, addrTemp, "POKE [[addrTemp]] (indirekt via STAMM)")
		return nil

	case *ast.OutStmt:
		// OUT Port, Value (Hardware Port 2 = OUT Register)
		if portVal, ok := evalConstExpr(s.Port); ok {
			if err := g.compileExpr(s.Value); err != nil {
				return err
			}
			g.emit(OpSTAM, uint16(portVal&0xFFFF), fmt.Sprintf("OUT Port %d = Accu (direkt)", portVal))
			return nil
		}
		if err := g.compileExpr(s.Value); err != nil {
			return err
		}
		valTemp := g.allocTemp()
		g.emit(OpSTAM, valTemp, "OUT Temp Wert")

		if err := g.compileExpr(s.Port); err != nil {
			return err
		}
		portTemp := g.allocTemp()
		g.emit(OpSTAM, portTemp, "OUT Port Zeiger")
		g.emit(OpLDAM, valTemp, "Lade OUT Wert")
		g.emit(OpSTAMM, portTemp, "OUT [[portTemp]] (indirekt via STAMM)")
		return nil

	case *ast.ScreenStmt:
		// SCREEN Stmt konfiguriert Speicheraufteilung (Adresse 768..1023 bzw. 960..1023)
		return nil

	case *ast.ClsStmt:
		g.needCls = true
		ptrCls := g.getOrCreateVar("__cls_ptr")
		endCls := g.getOrCreateVar("__cls_end_addr")

		if s.StartRow != nil {
			shiftCount := 1
			rowMultiplier := uint16(2)
			if g.resolution == 64 {
				shiftCount = 2
				rowMultiplier = 4
			}
			// StartAddr = DisplayBase + StartRow * rowMultiplier
			if err := g.compileExpr(s.StartRow); err != nil {
				return err
			}
			for sc := 0; sc < shiftCount; sc++ {
				g.emit(OpSHL, 0, "StartRow shift")
			}
			g.emit(OpADD, g.displayBase, "DisplayBase + StartRow*shift")
			g.emit(OpSTAM, ptrCls, "Setze __cls_ptr")

			// EndAddr = DisplayBase + EndRow * rowMultiplier + rowMultiplier
			if err := g.compileExpr(s.EndRow); err != nil {
				return err
			}
			for sc := 0; sc < shiftCount; sc++ {
				g.emit(OpSHL, 0, "EndRow shift")
			}
			g.emit(OpADD, g.displayBase+rowMultiplier, "DisplayBase + EndRow*shift + rowMultiplier")
			g.emit(OpSTAM, endCls, "Setze __cls_end_addr")
		} else {
			// Volles Display löschen: displayBase bis 1024
			g.emit(OpLDA, g.displayBase, fmt.Sprintf("Init __cls_ptr (%d)", g.displayBase))
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

	case *ast.PrintStmt:
		for _, item := range s.Items {
			if _, ok := item.(*ast.StringLiteral); ok {
				// String-Literale dienen der Konsolen-Dokumentation; auf der 16-Bit Relais-Hardware sind sie ein No-Op
				continue
			}
			if err := g.compileExpr(item); err != nil {
				return err
			}
			g.emit(OpSTAM, 2, "PRINT Wert auf Hardware-Output (Port 2)")
		}
		return nil

	default:
		return fmt.Errorf("Nicht unterstütztes Statement: %T", stmt)
	}
}

func (g *CodeGenerator) compileIf(s *ast.IfStmt) error {
	// Spezialfall: IF <Condition> THEN GOTO <Target> (ohne ELSE) -> Direkter Bedingungssprung ohne Umwege!
	if len(s.ElseBranch) == 0 && len(s.ThenBranch) == 1 {
		if gotoStmt, ok := s.ThenBranch[0].(*ast.GotoStmt); ok {
			return g.compileIfGoto(s.Condition, gotoStmt.Target, gotoStmt.Line)
		}
	}

	elseLabel := fmt.Sprintf("__else_%d", g.pc)
	endIfLabel := fmt.Sprintf("__endif_%d", g.pc)

	targetFalse := elseLabel
	if len(s.ElseBranch) == 0 {
		targetFalse = endIfLabel
	}

	// Bedingung direkt als Sprung auf targetFalse übersetzen (spart Materialisierung von 0/1 in Accu)
	if err := g.compileIfCondition(s.Condition, targetFalse); err != nil {
		return err
	}

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

// compileIfGoto optimiert "IF <cond> THEN GOTO <target>" zu einem direkten Sprung ohne Zwischenlabels.
func (g *CodeGenerator) compileIfGoto(cond ast.Expr, target string, line int) error {
	if val, ok := evalConstExpr(cond); ok {
		if (val & 0xFFFF) != 0 {
			g.emit(OpBA, 0, fmt.Sprintf("IF Konstante != 0 -> GOTO %s", target))
			g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: target, line: line})
		}
		return nil
	}

	if bin, ok := cond.(*ast.BinaryExpr); ok {
		branchOp, trueOnBranch, isComp, err := g.emitComparisonDiff(bin)
		if err != nil {
			return err
		}
		if isComp {
			if trueOnBranch {
				g.emit(branchOp, 0, fmt.Sprintf("IF %s -> GOTO %s (direkt)", bin.Op, target))
				g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: target, line: line})
				return nil
			}
			skipLabel := fmt.Sprintf("__if_skip_%d", g.pc)
			g.emit(branchOp, 0, fmt.Sprintf("IF NOT (%s) -> Skip GOTO", bin.Op))
			g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: skipLabel})
			g.emit(OpBA, 0, fmt.Sprintf("GOTO %s", target))
			g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: target, line: line})
			g.labels[skipLabel] = g.pc
			return nil
		}
	}

	skipLabel := fmt.Sprintf("__if_skip_%d", g.pc)
	if err := g.compileExpr(cond); err != nil {
		return err
	}
	g.emit(OpBZ, 0, "IF False -> Skip GOTO")
	g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: skipLabel})
	g.emit(OpBA, 0, fmt.Sprintf("GOTO %s", target))
	g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: target, line: line})
	g.labels[skipLabel] = g.pc
	return nil
}

// compileIfCondition wertet cond aus und springt direkt zu targetFalse, wenn die Bedingung nicht erfüllt ist.
func (g *CodeGenerator) compileIfCondition(cond ast.Expr, targetFalse string) error {
	if val, ok := evalConstExpr(cond); ok {
		if (val & 0xFFFF) == 0 {
			g.emit(OpBA, 0, "IF Konstante 0 -> Skip")
			g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: targetFalse})
		}
		return nil
	}

	if bin, ok := cond.(*ast.BinaryExpr); ok {
		branchOp, trueOnBranch, isComp, err := g.emitComparisonDiff(bin)
		if err != nil {
			return err
		}
		if isComp {
			if !trueOnBranch {
				// Branch-Opcode feuert genau dann, wenn die Bedingung FALSE ist (z.B. >=, <=, <>)
				g.emit(branchOp, 0, fmt.Sprintf("IF False (%s) -> Skip", bin.Op))
				g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: targetFalse})
				return nil
			}
			// Branch-Opcode feuert wenn Bedingung TRUE ist (z.B. =, <, >) -> überspringe BA targetFalse
			thenStartLabel := fmt.Sprintf("__then_%d", g.pc)
			g.emit(branchOp, 0, fmt.Sprintf("IF True (%s) -> Then", bin.Op))
			g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: thenStartLabel})
			g.emit(OpBA, 0, "IF False -> Skip")
			g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: targetFalse})
			g.labels[thenStartLabel] = g.pc
			return nil
		}
	}

	if err := g.compileExpr(cond); err != nil {
		return err
	}
	g.emit(OpBZ, 0, "IF False -> Skip")
	g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: targetFalse})
	return nil
}

// emitComparisonDiff berechnet für Vergleichsoperatoren (=, <>, <, >=, >, <=) die Differenz im Accu
// und gibt den zugehörigen Branch-Opcode (OpBZ / OpBN) sowie trueOnBranch zurück.
func (g *CodeGenerator) emitComparisonDiff(e *ast.BinaryExpr) (branchOp uint16, trueOnBranch bool, isComp bool, err error) {
	op := strings.ToUpper(e.Op)
	reverse := false
	switch op {
	case "=", "==":
		branchOp = OpBZ
		trueOnBranch = true
	case "<>", "!=":
		branchOp = OpBZ
		trueOnBranch = false
	case "<":
		branchOp = OpBN
		trueOnBranch = true
	case ">=":
		branchOp = OpBN
		trueOnBranch = false
	case ">":
		branchOp = OpBN
		trueOnBranch = true
		reverse = true
	case "<=":
		branchOp = OpBN
		trueOnBranch = false
		reverse = true
	default:
		return 0, false, false, nil
	}

	isFloat16Comp := g.exprType(e.Left) == ast.TypeFloat16 || g.exprType(e.Right) == ast.TypeFloat16
	if isFloat16Comp {
		g.needFSubF16 = true
		g.needFAddF16 = true
		if !reverse {
			if err := g.emitF16BinaryCall(e.Left, e.Right, false, "__f16_sub"); err != nil {
				return 0, false, true, err
			}
		} else {
			if err := g.emitF16BinaryCall(e.Right, e.Left, false, "__f16_sub"); err != nil {
				return 0, false, true, err
			}
		}
		return branchOp, trueOnBranch, true, nil
	}

	isFixedComp := g.exprType(e.Left) == ast.TypeFixed8 || g.exprType(e.Right) == ast.TypeFixed8
	if isFixedComp {
		if rVal, ok := evalConstFixed8Expr(e.Right); ok {
			if err := g.compileExprAsFixed8(e.Left); err != nil {
				return 0, false, true, err
			}
			if !reverse {
				if (rVal & 0xFFFF) != 0 {
					g.emit(OpADD, uint16((-rVal)&0xFFFF), fmt.Sprintf("Q8.8 Vergleich Accu - %d", rVal))
				}
			} else {
				g.emit(OpNOT, 0, "~Left")
				if ((rVal + 1) & 0xFFFF) != 0 {
					g.emit(OpADD, uint16((rVal+1)&0xFFFF), fmt.Sprintf("Q8.8 ~Left + %d", rVal+1))
				}
			}
			return branchOp, trueOnBranch, true, nil
		}
		if lVal, ok := evalConstFixed8Expr(e.Left); ok {
			if err := g.compileExprAsFixed8(e.Right); err != nil {
				return 0, false, true, err
			}
			if !reverse {
				g.emit(OpNOT, 0, "~Right")
				if ((lVal + 1) & 0xFFFF) != 0 {
					g.emit(OpADD, uint16((lVal+1)&0xFFFF), fmt.Sprintf("Q8.8 ~Right + %d", lVal+1))
				}
			} else {
				if (lVal & 0xFFFF) != 0 {
					g.emit(OpADD, uint16((-lVal)&0xFFFF), fmt.Sprintf("Q8.8 Vergleich Accu - %d", lVal))
				}
			}
			return branchOp, trueOnBranch, true, nil
		}
		if err := g.compileExprAsFixed8(e.Right); err != nil {
			return 0, false, true, err
		}
		tempRight := g.allocTemp()
		g.emit(OpSTAM, tempRight, "Q8.8 Rechter Vergleichsoperand")
		if err := g.compileExprAsFixed8(e.Left); err != nil {
			return 0, false, true, err
		}
		if !reverse {
			tempA := g.allocTemp()
			g.emit(OpSTAM, tempA, "Sichere A")
			g.emit(OpLDAM, tempRight, "Lade B")
			g.emit(OpNOT, 0, "~B")
			g.emit(OpADD, 1, "-B")
			patchAddr := g.pc + 5
			g.emit(OpSTAM, patchAddr, "Patch -B")
			g.emit(OpLDAM, tempA, "Lade A")
			g.emit(OpADD, 0, "Q8.8 A - B")
		} else {
			g.emit(OpNOT, 0, "~A")
			g.emit(OpADD, 1, "-A")
			patchAddr := g.pc + 5
			g.emit(OpSTAM, patchAddr, "Patch -A")
			g.emit(OpLDAM, tempRight, "Lade B")
			g.emit(OpADD, 0, "Q8.8 B - A")
		}
		return branchOp, trueOnBranch, true, nil
	}

	// Fall 1: Rechter Operand ist eine Konstante (z.B. X > 64, ZX < 0, I = 15)
	if rVal, ok := evalConstExpr(e.Right); ok {
		if err := g.compileExpr(e.Left); err != nil {
			return 0, false, true, err
		}
		if !reverse {
			// Berechne Left - rVal = Left + (-rVal)
			if (rVal & 0xFFFF) != 0 {
				g.emit(OpADD, uint16((-rVal)&0xFFFF), fmt.Sprintf("Vergleich Accu - %d", rVal))
			}
		} else {
			// Berechne rVal - Left = ~Left + (rVal + 1)
			g.emit(OpNOT, 0, "~Left")
			if ((rVal + 1) & 0xFFFF) != 0 {
				g.emit(OpADD, uint16((rVal+1)&0xFFFF), fmt.Sprintf("~Left + %d (%d - Left)", rVal+1, rVal))
			}
		}
		return branchOp, trueOnBranch, true, nil
	}

	// Fall 2: Linker Operand ist eine Konstante (z.B. 0 < X)
	if lVal, ok := evalConstExpr(e.Left); ok {
		if err := g.compileExpr(e.Right); err != nil {
			return 0, false, true, err
		}
		if !reverse {
			// Berechne lVal - Right = ~Right + (lVal + 1)
			g.emit(OpNOT, 0, "~Right")
			if ((lVal + 1) & 0xFFFF) != 0 {
				g.emit(OpADD, uint16((lVal+1)&0xFFFF), fmt.Sprintf("~Right + %d (%d - Right)", lVal+1, lVal))
			}
		} else {
			// Berechne Right - lVal = Right + (-lVal)
			if (lVal & 0xFFFF) != 0 {
				g.emit(OpADD, uint16((-lVal)&0xFFFF), fmt.Sprintf("Vergleich Accu - %d", lVal))
			}
		}
		return branchOp, trueOnBranch, true, nil
	}

	// Fall 3: Rechter Operand ist eine einfache Variable (spart Temp-Allokation!)
	if vRight, ok := e.Right.(*ast.VariableExpr); ok {
		vAddr := g.getOrCreateVar(vRight.Name)
		if !reverse {
			// Berechne Left - vRight
			g.emit(OpLDAM, vAddr, fmt.Sprintf("Lade %s", vRight.Name))
			g.emit(OpNOT, 0, fmt.Sprintf("~%s", vRight.Name))
			g.emit(OpADD, 1, fmt.Sprintf("-%s", vRight.Name))
			patchAddr := g.pc + 5
			g.emit(OpSTAM, patchAddr, "Patch Vergleichs-ADD")
			if err := g.compileExpr(e.Left); err != nil {
				return 0, false, true, err
			}
			g.emit(OpADD, 0, fmt.Sprintf("Left - %s", vRight.Name))
		} else {
			// Berechne vRight - Left
			if err := g.compileExpr(e.Left); err != nil {
				return 0, false, true, err
			}
			g.emit(OpNOT, 0, "~Left")
			g.emit(OpADD, 1, "-Left")
			patchAddr := g.pc + 5
			g.emit(OpSTAM, patchAddr, "Patch Vergleichs-ADD")
			g.emit(OpLDAM, vAddr, fmt.Sprintf("Lade %s", vRight.Name))
			g.emit(OpADD, 0, fmt.Sprintf("%s - Left", vRight.Name))
		}
		return branchOp, trueOnBranch, true, nil
	}

	// Fall 4: Allgemeine Ausdrücke Left und Right
	if err := g.compileExpr(e.Right); err != nil {
		return 0, false, true, err
	}
	tempRight := g.allocTemp()
	g.emit(OpSTAM, tempRight, "Rechter Vergleichsoperand")

	if err := g.compileExpr(e.Left); err != nil {
		return 0, false, true, err
	}

	if !reverse {
		tempA := g.allocTemp()
		g.emit(OpSTAM, tempA, "Sichere A")
		g.emit(OpLDAM, tempRight, "Lade B")
		g.emit(OpNOT, 0, "~B")
		g.emit(OpADD, 1, "-B")
		patchAddr := g.pc + 5
		g.emit(OpSTAM, patchAddr, "Patch -B")
		g.emit(OpLDAM, tempA, "Lade A")
		g.emit(OpADD, 0, "A - B")
	} else {
		g.emit(OpNOT, 0, "~A")
		g.emit(OpADD, 1, "-A")
		patchAddr := g.pc + 5
		g.emit(OpSTAM, patchAddr, "Patch -A")
		g.emit(OpLDAM, tempRight, "Lade B")
		g.emit(OpADD, 0, "B - A")
	}
	return branchOp, trueOnBranch, true, nil
}

func (g *CodeGenerator) compileFor(s *ast.ForStmt) error {
	upper := strings.ToUpper(s.VarName)
	if !g.explicitVarTypes[upper] {
		if strings.HasSuffix(upper, "#") || g.exprType(s.Start) == ast.TypeFloat16 || g.exprType(s.End) == ast.TypeFloat16 || (s.Step != nil && g.exprType(s.Step) == ast.TypeFloat16) {
			g.varTypes[upper] = ast.TypeFloat16
		} else if g.exprType(s.Start) == ast.TypeFixed8 || g.exprType(s.End) == ast.TypeFixed8 || (s.Step != nil && g.exprType(s.Step) == ast.TypeFixed8) {
			g.varTypes[upper] = ast.TypeFixed8
		}
	}
	if g.getVarType(s.VarName) == ast.TypeFloat16 {
		varAddr := g.getOrCreateVar(s.VarName)
		if err := g.compileExprAsFloat16(s.Start); err != nil {
			return err
		}
		g.emit(OpSTAM, varAddr, fmt.Sprintf("FOR %s Startwert (F16)", s.VarName))

		endAddr := g.getOrCreateVar(fmt.Sprintf("__for_limit_%s", s.VarName))
		if err := g.compileExprAsFloat16(s.End); err != nil {
			return err
		}
		g.emit(OpSTAM, endAddr, fmt.Sprintf("FOR %s Endwert (F16)", s.VarName))

		stepAddr := g.getOrCreateVar(fmt.Sprintf("__for_step_%s", s.VarName))
		stepNeg := false
		if s.Step != nil {
			if cStep, ok := evalConstFloat16Expr(s.Step); ok {
				stepNeg = (cStep & 0x8000) != 0
				g.emit(OpLDA, cStep, "FOR Step (F16)")
			} else {
				if err := g.compileExprAsFloat16(s.Step); err != nil {
					return err
				}
			}
		} else {
			g.emit(OpLDA, F16One, "FOR Default Step 1.0 (F16)")
		}
		g.emit(OpSTAM, stepAddr, fmt.Sprintf("FOR %s Schrittweite (F16)", s.VarName))

		loopStartPC := g.pc
		loopStartLabel := fmt.Sprintf("__for_start_%s_%d", s.VarName, loopStartPC)
		loopEndLabel := fmt.Sprintf("__for_end_%s_%d", s.VarName, loopStartPC)
		g.labels[loopStartLabel] = loopStartPC

		g.needFSubF16 = true
		g.needFAddF16 = true
		r0Addr := g.getOrCreateVar("__r0")
		r1Addr := g.getOrCreateVar("__r1")
		if !stepNeg {
			g.emit(OpLDAM, endAddr, "FOR F16 Lade End")
			g.emit(OpSTAM, r0Addr, "__r0 = End")
			g.emit(OpLDAM, varAddr, "FOR F16 Lade Var")
			g.emit(OpSTAM, r1Addr, "__r1 = Var")
		} else {
			g.emit(OpLDAM, varAddr, "FOR F16 Lade Var")
			g.emit(OpSTAM, r0Addr, "__r0 = Var")
			g.emit(OpLDAM, endAddr, "FOR F16 Lade End")
			g.emit(OpSTAM, r1Addr, "__r1 = End")
		}
		g.emit(OpCALL, 0, "CALL __f16_sub (FOR Limit-Check)")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_sub"})
		g.emit(OpBN, 0, "FOR F16 Beenden")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: loopEndLabel})

		for _, bStmt := range s.Body {
			if err := g.compileStmt(bStmt); err != nil {
				return err
			}
		}

		g.emit(OpLDAM, varAddr, "FOR F16 Lade Var")
		g.emit(OpSTAM, r0Addr, "__r0 = Var")
		g.emit(OpLDAM, stepAddr, "FOR F16 Lade Step")
		g.emit(OpSTAM, r1Addr, "__r1 = Step")
		g.emit(OpCALL, 0, "CALL __f16_add (FOR Step)")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_add"})
		g.emit(OpSTAM, varAddr, "FOR F16 Speichere Var")

		g.emit(OpBA, 0, "FOR Loop Sprung")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: loopStartLabel})
		g.labels[loopEndLabel] = g.pc
		return nil
	}
	isFixedLoop := g.getVarType(s.VarName) == ast.TypeFixed8
	varAddr := g.getOrCreateVar(s.VarName)

	// 1. Initialisiere Startwert: Var = Start
	if isFixedLoop {
		if err := g.compileExprAsFixed8(s.Start); err != nil {
			return err
		}
	} else {
		if err := g.compileExpr(s.Start); err != nil {
			return err
		}
	}
	g.emit(OpSTAM, varAddr, fmt.Sprintf("FOR %s Startwert", s.VarName))

	// Ermittle Schrittweite (Standard: +1 bzw. +1.0 bei FIXED8, unterstützt auch negative Konstanten)
	stepVal := 1
	if isFixedLoop {
		stepVal = 256
		if s.Step != nil {
			if val, ok := evalConstFixed8Expr(s.Step); ok {
				stepVal = val
			}
		}
	} else if s.Step != nil {
		if val, ok := evalConstExpr(s.Step); ok {
			stepVal = val
		} else if num, ok := s.Step.(*ast.NumberLiteral); ok {
			stepVal = num.Value
		}
	}

	// Spezialfall: Konstanter Endwert (z.B. FOR I = 1 TO 31 oder FOR Y = 31 TO 0 STEP -1)
	var endVal int
	var endIsConst bool
	if isFixedLoop {
		endVal, endIsConst = evalConstFixed8Expr(s.End)
	} else {
		endVal, endIsConst = evalConstExpr(s.End)
	}
	if endIsConst {
		loopStartPC := g.pc
		loopStartLabel := fmt.Sprintf("__for_start_%s_%d", s.VarName, loopStartPC)
		loopEndLabel := fmt.Sprintf("__for_end_%s_%d", s.VarName, loopStartPC)
		g.labels[loopStartLabel] = loopStartPC

		if stepVal >= 0 {
			// Aufwärts zählen: Bedingung Var <= endVal -> endVal - Var >= 0 (~Var + endVal + 1)
			g.emit(OpLDAM, varAddr, "Pruefe FOR Limit (optimiert)")
			g.emit(OpNOT, 0, "~Var")
			if ((endVal + 1) & 0xFFFF) != 0 {
				g.emit(OpADD, uint16((endVal+1)&0xFFFF), fmt.Sprintf("~Var + %d (Endwert-Check)", endVal+1))
			}
			g.emit(OpBN, 0, "FOR Beenden wenn Var > End")
			g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: loopEndLabel})
		} else {
			// Abwärts zählen (stepVal < 0): Bedingung Var >= endVal -> Var - endVal >= 0
			g.emit(OpLDAM, varAddr, "Pruefe FOR Limit abwaerts (optimiert)")
			if (endVal & 0xFFFF) != 0 {
				g.emit(OpADD, uint16((-endVal)&0xFFFF), fmt.Sprintf("Var - %d (Endwert-Check)", endVal))
			}
			g.emit(OpBN, 0, "FOR Beenden wenn Var < End")
			g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: loopEndLabel})
		}

		// Schleifenrumpf (Body)
		for _, bStmt := range s.Body {
			if err := g.compileStmt(bStmt); err != nil {
				return err
			}
		}

		// Schrittweite addieren: Var = Var + Step
		g.emit(OpLDAM, varAddr, "Lade Zaehler")
		g.emit(OpADD, uint16(stepVal&0xFFFF), fmt.Sprintf("Zaehler + %d", stepVal))
		g.emit(OpSTAM, varAddr, "Speichere Zaehler")

		// Sprung zurück zum Schleifenanfang
		g.emit(OpBA, 0, "FOR Loop Sprung")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: loopStartLabel})

		g.labels[loopEndLabel] = g.pc
		return nil
	}

	// Dynamischer Endwert: Eigene Symbol-Zelle statt flüchtigem allocTemp() verwenden,
	// damit komplexe Ausdrücke im Schleifenrumpf niemals das Schleifenlimit überschreiben!
	if isFixedLoop {
		if err := g.compileExprAsFixed8(s.End); err != nil {
			return err
		}
	} else {
		if err := g.compileExpr(s.End); err != nil {
			return err
		}
	}
	endAddr := g.getOrCreateVar(fmt.Sprintf("__for_limit_%s", s.VarName))
	g.emit(OpSTAM, endAddr, fmt.Sprintf("FOR %s Endwert", s.VarName))

	loopStartPC := g.pc
	loopStartLabel := fmt.Sprintf("__for_start_%s_%d", s.VarName, loopStartPC)
	loopEndLabel := fmt.Sprintf("__for_end_%s_%d", s.VarName, loopStartPC)
	g.labels[loopStartLabel] = loopStartPC

	if stepVal >= 0 {
		// 3a. Schleifenbedingung aufwärts: Var <= End (End - Var >= 0)
		g.emit(OpLDAM, varAddr, "Pruefe FOR Limit")
		g.emit(OpNOT, 0, "~Var")
		g.emit(OpADD, 1, "~Var + 1")
		patchAddr := g.pc + 5
		g.emit(OpSTAM, patchAddr, "Patch Limit Addition")
		g.emit(OpLDAM, endAddr, "Lade Endwert")
		g.emit(OpADD, 0, "Endwert - Var")
		g.emit(OpBN, 0, "FOR Beenden wenn Var > End")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: loopEndLabel})
	} else {
		// 3b. Schleifenbedingung abwärts: Var >= End (Var - End >= 0)
		g.emit(OpLDAM, endAddr, "Lade Endwert")
		g.emit(OpNOT, 0, "~End")
		g.emit(OpADD, 1, "~End + 1")
		patchAddr := g.pc + 5
		g.emit(OpSTAM, patchAddr, "Patch Limit Subtraktion")
		g.emit(OpLDAM, varAddr, "Lade Zaehler")
		g.emit(OpADD, 0, "Var - Endwert")
		g.emit(OpBN, 0, "FOR Beenden wenn Var < End")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: loopEndLabel})
	}

	// 4. Schleifenrumpf (Body)
	for _, bStmt := range s.Body {
		if err := g.compileStmt(bStmt); err != nil {
			return err
		}
	}

	// 5. Schrittweite addieren (Inkrement): Var = Var + Step
	g.emit(OpLDAM, varAddr, "Lade Zaehler")
	g.emit(OpADD, uint16(stepVal&0xFFFF), fmt.Sprintf("Zaehler + %d", stepVal))
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
// COMPILE-TIME CONSTANT FOLDING
// ============================================================================

// evalConstExpr wertet Zahlenliterale sowie unäre Konstanten (z.B. -47, -1, NOT 0)
// direkt zur Compile-Zeit aus, damit negative Literale als echte Konstanten erkannt werden.
func evalConstExpr(expr ast.Expr) (int, bool) {
	if expr == nil {
		return 0, false
	}
	switch e := expr.(type) {
	case *ast.NumberLiteral:
		return e.Value, true
	case *ast.UnaryExpr:
		val, ok := evalConstExpr(e.Right)
		if !ok {
			return 0, false
		}
		switch strings.ToUpper(e.Op) {
		case "-":
			return -val, true
		case "NOT":
			return int(int16(^uint16(val & 0xFFFF))), true
		}
		return 0, false
	default:
		return 0, false
	}
}

// ============================================================================
// AUSDRÜCKE (EXPRESSIONS) IN ACCU KOMPILIEREN
// ============================================================================

func (g *CodeGenerator) compileExpr(expr ast.Expr) error {
	// Compile-Time Constant Folding (z.B. -47, 10 + 20, 1 SHL 8)
	if val, ok := evalConstExpr(expr); ok {
		g.emit(OpLDA, uint16(val&0xFFFF), fmt.Sprintf("Konstante %d", val))
		return nil
	}

	switch e := expr.(type) {
	case *ast.NumberLiteral:
		g.emit(OpLDA, uint16(e.Value&0xFFFF), fmt.Sprintf("Konstante %d", e.Value))
		return nil

	case *ast.StringLiteral:
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
			if g.exprType(e.Right) == ast.TypeFloat16 {
				g.emitF16Neg()
				return nil
			}
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
		// Spezialfall: Konstante Adresse (z.B. PEEK(3))
		if addrVal, ok := evalConstExpr(e.Address); ok {
			g.emit(OpLDAM, uint16(addrVal&0xFFFF), fmt.Sprintf("PEEK([%d]) direkt", addrVal))
			return nil
		}
		if err := g.compileExpr(e.Address); err != nil {
			return err
		}
		ptrTemp := g.allocTemp()
		g.emit(OpSTAM, ptrTemp, "PEEK Zeigeradresse")
		g.emit(OpLDAMM, ptrTemp, "PEEK [[ptrTemp]] (indirekt via LDAMM)")
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

	case *ast.FixedFuncExpr:
		return g.compileFixedFunc(e)

	default:
		return fmt.Errorf("Unbekannter Ausdruckstyp: %T", expr)
	}
}

func (g *CodeGenerator) compileBinary(e *ast.BinaryExpr) error {
	op := strings.ToUpper(e.Op)

	// Wenn einer der Operanden vom Typ FLOAT16 (CLACK-F16) ist, nutze CLACK-F16 Gleitkomma-Arithmetik
	if g.exprType(e.Left) == ast.TypeFloat16 || g.exprType(e.Right) == ast.TypeFloat16 {
		switch op {
		case "+", "-", "*", "/", "=", "==", "<>", "!=", "<", "<=", ">", ">=":
			return g.compileBinaryFloat16(e)
		}
	}

	// Wenn einer der Operanden vom Typ FIXED8 (Q8.8) ist, nutze Q8.8 Festkomma-Arithmetik
	if g.exprType(e.Left) == ast.TypeFixed8 || g.exprType(e.Right) == ast.TypeFixed8 {
		switch op {
		case "+", "-", "*", "/", "=", "==", "<>", "!=", "<", "<=", ">", ">=":
			return g.compileBinaryFixed8(e)
		}
	}

	rConstVal, rIsConst := evalConstExpr(e.Right)
	lConstVal, lIsConst := evalConstExpr(e.Left)

	// Spezialfall: Multiplikation (*)
	if op == "*" {
		// Optimierung: Multiplikation mit 0
		if rIsConst && rConstVal == 0 {
			g.emit(OpLDA, 0, "Accu = 0 (* 0)")
			return nil
		}
		if lIsConst && lConstVal == 0 {
			g.emit(OpLDA, 0, "Accu = 0 (0 *)")
			return nil
		}

		// Optimierung: Multiplikation mit 1
		if rIsConst && rConstVal == 1 {
			return g.compileExpr(e.Left)
		}
		if lIsConst && lConstVal == 1 {
			return g.compileExpr(e.Right)
		}

		// Optimierung: Multiplikation mit Zweierpotenz (2, 4, 8, 16, 32, ...) per Hardware-Linksshift SHL
		if rIsConst && rConstVal > 0 && (rConstVal&(rConstVal-1)) == 0 && rConstVal <= 32768 {
			if err := g.compileExpr(e.Left); err != nil {
				return err
			}
			shifts := 0
			val := rConstVal
			for val > 1 {
				val >>= 1
				shifts++
			}
			for s := 0; s < shifts; s++ {
				g.emit(OpSHL, 0, fmt.Sprintf("SHL Linksshift (*%d, %d/%d)", rConstVal, s+1, shifts))
			}
			return nil
		}
		if lIsConst && lConstVal > 0 && (lConstVal&(lConstVal-1)) == 0 && lConstVal <= 32768 {
			if err := g.compileExpr(e.Right); err != nil {
				return err
			}
			shifts := 0
			val := lConstVal
			for val > 1 {
				val >>= 1
				shifts++
			}
			for s := 0; s < shifts; s++ {
				g.emit(OpSHL, 0, fmt.Sprintf("SHL Linksshift (*%d, %d/%d)", lConstVal, s+1, shifts))
			}
			return nil
		}

		g.needMul = true
		if err := g.compileExpr(e.Left); err != nil {
			return err
		}
		g.emit(OpSTAM, g.getOrCreateVar("__r0"), "Multiplikand __r0")

		// Spezialfall: Quadrieren derselben Variable (z.B. X2 * X2) -> Accu direkt auch in __r1 speichern!
		if vL, okL := e.Left.(*ast.VariableExpr); okL {
			if vR, okR := e.Right.(*ast.VariableExpr); okR && strings.EqualFold(vL.Name, vR.Name) {
				g.emit(OpSTAM, g.getOrCreateVar("__r1"), "Multiplikator __r1 (Quadrat)")
				g.emit(OpCALL, 0, "CALL __mul")
				g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__runtime_mul"})
				return nil
			}
		}

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
		// Optimierung: Division durch 1
		if op == "/" && rIsConst && rConstVal == 1 {
			return g.compileExpr(e.Left)
		}

		// Optimierung: Modulo 1 -> immer 0
		if op == "MOD" && rIsConst && rConstVal == 1 {
			g.emit(OpLDA, 0, "Accu = 0 (MOD 1)")
			return nil
		}

		// Optimierung: Division durch Zweierpotenz (2, 4, 8, 16, 32, 64, ...) per Hardware-Rechtsshift SHR
		if op == "/" && rIsConst && rConstVal > 0 && (rConstVal&(rConstVal-1)) == 0 && rConstVal <= 32768 {
			if err := g.compileExpr(e.Left); err != nil {
				return err
			}
			shifts := 0
			val := rConstVal
			for val > 1 {
				val >>= 1
				shifts++
			}
			for s := 0; s < shifts; s++ {
				g.emit(OpSHR, 0, fmt.Sprintf("SHR Rechtsshift (/%d, %d/%d)", rConstVal, s+1, shifts))
			}
			return nil
		}

		// Optimierung: Modulo durch Zweierpotenz (2, 4, 8, 16, 32, 64, ...) per Bitmaske AND (value - 1)
		if op == "MOD" && rIsConst && rConstVal > 0 && (rConstVal&(rConstVal-1)) == 0 && rConstVal <= 32768 {
			if err := g.compileExpr(e.Left); err != nil {
				return err
			}
			mask := uint16((rConstVal - 1) & 0xFFFF)
			g.emit(OpAND, mask, fmt.Sprintf("Accu AND 0x%04X (MOD %d)", mask, rConstVal))
			return nil
		}

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

	// Schnelle Optimierung für unmittelbare Zahlenkonstanten (+, -, AND, OR, XOR, SHR, SHL)
	if rIsConst {
		if op == "+" {
			if err := g.compileExpr(e.Left); err != nil {
				return err
			}
			if (rConstVal & 0xFFFF) != 0 {
				g.emit(OpADD, uint16(rConstVal&0xFFFF), fmt.Sprintf("Accu + %d", rConstVal))
			}
			return nil
		} else if op == "-" {
			if err := g.compileExpr(e.Left); err != nil {
				return err
			}
			if (rConstVal & 0xFFFF) != 0 {
				g.emit(OpADD, uint16((-rConstVal)&0xFFFF), fmt.Sprintf("Accu - %d", rConstVal))
			}
			return nil
		} else if op == "AND" {
			if err := g.compileExpr(e.Left); err != nil {
				return err
			}
			g.emit(OpAND, uint16(rConstVal&0xFFFF), fmt.Sprintf("Accu AND 0x%04X", rConstVal))
			return nil
		} else if op == "OR" {
			if err := g.compileExpr(e.Left); err != nil {
				return err
			}
			g.emit(OpOR, uint16(rConstVal&0xFFFF), fmt.Sprintf("Accu OR 0x%04X", rConstVal))
			return nil
		} else if op == "XOR" {
			if err := g.compileExpr(e.Left); err != nil {
				return err
			}
			tempA := g.allocTemp()
			g.emit(OpSTAM, tempA, "Temp A sichern")
			g.emit(OpOR, uint16(rConstVal&0xFFFF), "A OR B")
			tempOr := g.allocTemp()
			g.emit(OpSTAM, tempOr, "Temp (A OR B) sichern")
			g.emit(OpLDAM, tempA, "Lade A")
			g.emit(OpAND, uint16(rConstVal&0xFFFF), "A AND B")
			g.emit(OpNOT, 0, "NOT (A AND B)")
			patchAnd := g.pc + 5
			g.emit(OpSTAM, patchAnd, "Patch AND")
			g.emit(OpLDAM, tempOr, "Lade (A OR B)")
			g.emit(OpAND, 0, "(A OR B) AND NOT (A AND B)")
			return nil
		} else if op == "SHR" {
			if err := g.compileExpr(e.Left); err != nil {
				return err
			}
			shifts := rConstVal
			if shifts <= 0 {
				shifts = 1
			}
			for s := 0; s < shifts; s++ {
				g.emit(OpSHR, 0, "SHR Rechtsshift")
			}
			return nil
		} else if op == "SHL" {
			if err := g.compileExpr(e.Left); err != nil {
				return err
			}
			shifts := rConstVal
			if shifts <= 0 {
				shifts = 1
			}
			for s := 0; s < shifts; s++ {
				g.emit(OpSHL, 0, "SHL Linksshift")
			}
			return nil
		}
	}

	// Schnelle Optimierung: Linke Seite ist eine Zahlenkonstante (z.B. 10 + X oder 1 - S)
	if lIsConst {
		if op == "+" {
			if err := g.compileExpr(e.Right); err != nil {
				return err
			}
			if (lConstVal & 0xFFFF) != 0 {
				g.emit(OpADD, uint16(lConstVal&0xFFFF), fmt.Sprintf("Accu + %d", lConstVal))
			}
			return nil
		} else if op == "-" {
			// lConstVal - Right = ~Right + (lConstVal + 1)
			if err := g.compileExpr(e.Right); err != nil {
				return err
			}
			g.emit(OpNOT, 0, "~Right")
			if ((lConstVal + 1) & 0xFFFF) != 0 {
				g.emit(OpADD, uint16((lConstVal+1)&0xFFFF), fmt.Sprintf("~Right + %d (%d - Right)", lConstVal+1, lConstVal))
			}
			return nil
		} else if op == "AND" {
			if err := g.compileExpr(e.Right); err != nil {
				return err
			}
			g.emit(OpAND, uint16(lConstVal&0xFFFF), fmt.Sprintf("Accu AND 0x%04X", lConstVal))
			return nil
		} else if op == "OR" {
			if err := g.compileExpr(e.Right); err != nil {
				return err
			}
			g.emit(OpOR, uint16(lConstVal&0xFFFF), fmt.Sprintf("Accu OR 0x%04X", lConstVal))
			return nil
		}
	}

	// Schnelle Optimierung: Rechter Operand ist eine einfache Variable (+, -, AND, OR)
	// Vermeidet das Allokieren und Neuladen temporärer Zwischenspeicher
	if vRight, ok := e.Right.(*ast.VariableExpr); ok && (op == "+" || op == "AND" || op == "OR") {
		vAddr := g.getOrCreateVar(vRight.Name)
		g.emit(OpLDAM, vAddr, fmt.Sprintf("Lade %s fuer Patch", vRight.Name))
		patchAddr := g.pc + 5
		g.emit(OpSTAM, patchAddr, fmt.Sprintf("Patch %s Operand", op))
		if err := g.compileExpr(e.Left); err != nil {
			return err
		}
		switch op {
		case "+":
			g.emit(OpADD, 0, fmt.Sprintf("A + %s", vRight.Name))
		case "AND":
			g.emit(OpAND, 0, fmt.Sprintf("A AND %s", vRight.Name))
		case "OR":
			g.emit(OpOR, 0, fmt.Sprintf("A OR %s", vRight.Name))
		}
		return nil
	}
	if vRight, ok := e.Right.(*ast.VariableExpr); ok && op == "-" {
		vAddr := g.getOrCreateVar(vRight.Name)
		g.emit(OpLDAM, vAddr, fmt.Sprintf("Lade %s fuer Negation", vRight.Name))
		g.emit(OpNOT, 0, fmt.Sprintf("~%s", vRight.Name))
		g.emit(OpADD, 1, fmt.Sprintf("-%s", vRight.Name))
		patchAddr := g.pc + 5
		g.emit(OpSTAM, patchAddr, "Patch ADD Operand mit -B")
		if err := g.compileExpr(e.Left); err != nil {
			return err
		}
		g.emit(OpADD, 0, fmt.Sprintf("A - %s", vRight.Name))
		return nil
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
		tempA := g.allocTemp()
		g.emit(OpSTAM, tempA, "Temp A sichern")
		g.emit(OpLDAM, tempRight, "Lade B")
		patchAddr := g.pc + 5
		g.emit(OpSTAM, patchAddr, "Patch AND Operand mit B")
		g.emit(OpLDAM, tempA, "Lade A zurueck")
		g.emit(OpAND, 0, "A AND B")

	case "OR":
		tempA := g.allocTemp()
		g.emit(OpSTAM, tempA, "Temp A sichern")
		g.emit(OpLDAM, tempRight, "Lade B")
		patchAddr := g.pc + 5
		g.emit(OpSTAM, patchAddr, "Patch OR Operand mit B")
		g.emit(OpLDAM, tempA, "Lade A zurueck")
		g.emit(OpOR, 0, "A OR B")

	case "XOR":
		// A XOR B = (A OR B) AND NOT (A AND B)
		tempA := g.allocTemp()
		g.emit(OpSTAM, tempA, "Temp A sichern")
		// A OR B
		g.emit(OpLDAM, tempRight, "Lade B")
		patchOr := g.pc + 5
		g.emit(OpSTAM, patchOr, "Patch OR mit B")
		g.emit(OpLDAM, tempA, "Lade A")
		g.emit(OpOR, 0, "A OR B")
		tempOr := g.allocTemp()
		g.emit(OpSTAM, tempOr, "Temp (A OR B)")
		// NOT (A AND B)
		g.emit(OpLDAM, tempRight, "Lade B")
		patchAnd1 := g.pc + 5
		g.emit(OpSTAM, patchAnd1, "Patch AND mit B")
		g.emit(OpLDAM, tempA, "Lade A")
		g.emit(OpAND, 0, "A AND B")
		g.emit(OpNOT, 0, "NOT (A AND B)")
		// (A OR B) AND NOT (A AND B)
		patchAnd2 := g.pc + 5
		g.emit(OpSTAM, patchAnd2, "Patch AND mit NOT(A AND B)")
		g.emit(OpLDAM, tempOr, "Lade (A OR B)")
		g.emit(OpAND, 0, "(A OR B) AND NOT (A AND B)")

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

		g.emit(OpLDAM, r1Addr, "__mul: Lade Faktor 2")
		g.emit(OpBZ, 0, "__mul: Wenn Faktor 2 == 0 -> Fertig")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__mul_done"})

		loopPC := g.pc
		g.labels["__mul_loop"] = loopPC

		// Accu hält bereits r1: Prüfe LSB(r1) mit BE (Even)
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
		// Shift r0 links, danach r1 rechts (damit Accu beim Rücksprung bereits r1 hält!)
		g.emit(OpLDAM, r0Addr, "Shift r0 links")
		g.emit(OpSHL, 0, "r0 << 1")
		g.emit(OpSTAM, r0Addr, "Speichere r0")

		g.emit(OpLDAM, r1Addr, "Shift r1 rechts")
		g.emit(OpSHR, 0, "r1 >> 1")
		g.emit(OpBZ, 0, "__mul: Wenn r1 == 0 -> Fertig")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__mul_done"})
		g.emit(OpSTAM, r1Addr, "Speichere r1 (Accu bleibt r1)")

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

		// Vorab-Berechnung: Berechne -Divisor EINMALIG vor Eintritt in die Schleife
		g.emit(OpLDAM, r1Addr, "__div: Lade Divisor")
		g.emit(OpNOT, 0, "~Divisor")
		g.emit(OpADD, 1, "-Divisor")
		patchDiv := g.pc + 5
		g.emit(OpSTAM, patchDiv, "Patch Divisor-Subtraktion")

		divLoopPC := g.pc
		g.labels["__div_loop"] = divLoopPC

		g.emit(OpLDAM, remAddr, "Lade rem")
		g.emit(OpADD, 0, "rem - Divisor (vorab gepatcht)")

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

	// 2c. Q8.8 Festkomma-Laufzeitroutinen (__runtime_fmul_q88, __runtime_fdiv_q88, __runtime_ftoi_q88)
	g.emitFixedQ88Routines()

	// 2d. CLACK-F16 Gleitkomma-Laufzeitroutinen (__f16_add, __f16_sub, __f16_mul, __f16_div, __f16_itof, __f16_ftoi)
	g.emitFloatF16Routines()

	// 3. Display Löschen __cls (von __cls_ptr bis __cls_end_addr)
	if g.needCls {
		g.labels["__runtime_cls"] = g.pc
		ptrCls := g.getOrCreateVar("__cls_ptr")
		endCls := g.getOrCreateVar("__cls_end_addr")

		clsLoop := g.pc
		g.labels["__cls_loop"] = clsLoop

		// Schnell: Indirekter Speicherzugriff ueber STAMM
		g.emit(OpLDA, 0, "Pixeldaten = 0")
		g.emit(OpSTAMM, ptrCls, "Schreibe in Display via STAMM")

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

	// 4. Pixel setzen/löschen __pset (PSET X, Y, Color) mit O(1) Bitmasken-Tabelle & STAMM
	if g.needPset {
		g.labels["__runtime_pset"] = g.pc
		xAddr := g.getOrCreateVar("__pset_x")
		yAddr := g.getOrCreateVar("__pset_y")
		cAddr := g.getOrCreateVar("__pset_c")
		wordAddr := g.getOrCreateVar("__pset_word_addr")
		maskAddr := g.getOrCreateVar("__pset_mask")
		currWordAddr := g.getOrCreateVar("__pset_curr_word")
		maskTablePtr := g.getOrCreateVar("__pset_mask_ptr")

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

		// 2. Bitmaske aus vorab erstellter 16-Wort-Tabelle per LDAMM laden (O(1) direkt statt Shift-Schleife!)
		g.emit(OpLDAM, xAddr, "Lade X")
		g.emit(OpAND, 0x000F, "X & 15 (Bit-Index)")
		g.emit(OpADD, 0, "Addiere Tabellenbasis")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__pset_mask_table"})
		g.emit(OpSTAM, maskTablePtr, "Speichere Zeiger auf Maske")
		g.emit(OpLDAMM, maskTablePtr, "Lade Maske via LDAMM")
		g.emit(OpSTAM, maskAddr, "Speichere Maske")

		// 3. Lade aktuellen Bildschirminhalt via LDAMM
		g.emit(OpLDAMM, wordAddr, "Lade aktuellen Wortwert via LDAMM")
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

		// 5. Schreibe modifiziertes Wort zurück via STAMM
		g.labels["__pset_write"] = g.pc
		g.emit(OpLDAM, currWordAddr, "Lade geändertes Wort")
		g.emit(OpSTAMM, wordAddr, "Schreibe Pixelwort in Display via STAMM")
		g.emitSingle(OpRET, "RETURN __pset")
	}

	// 5. Bresenham Linie zeichnen __line (LINE X1, Y1 TO X2, Y2, Color) mit O(1) Bitmasken-Tabelle & STAMM
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
		maskPtrAddr := g.getOrCreateVar("__pset_mask_ptr")
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

		// Bitmaske aus 16-Wort-Tabelle per LDAMM laden (O(1) statt langsamer Shift-Schleife!)
		g.emit(OpLDAM, x1Addr, "Lade x1")
		g.emit(OpAND, 0x000F, "x1 & 15 (Bit-Index)")
		g.emit(OpADD, 0, "Addiere Tabellenbasis")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__pset_mask_table"})
		g.emit(OpSTAM, maskPtrAddr, "Speichere Zeiger auf Maske")
		g.emit(OpLDAMM, maskPtrAddr, "Lade Maske via LDAMM")
		g.emit(OpSTAM, maskAddr, "Speichere Maske")

		// Lese aktuellen Wert an wordAddr via LDAMM
		g.emit(OpLDAMM, wordAddr, "Lade aktuellen Wortwert via LDAMM")
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

		// Write back via STAMM
		g.labels[lineWriteLabel] = g.pc
		g.emit(OpLDAM, currWordAddr, "Lade geändertes Wort")
		g.emit(OpSTAMM, wordAddr, "Schreibe Pixelwort in Display via STAMM")

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

	// 6. Gemeinsame Masken-Tabelle einbetten (16 Worte für Bits 15 bis 0), falls PSET oder LINE genutzt wird
	if g.needPset || g.needLine {
		g.labels["__pset_mask_table"] = g.pc
		masks := []uint16{
			0x8000, 0x4000, 0x2000, 0x1000,
			0x0800, 0x0400, 0x0200, 0x0100,
			0x0080, 0x0040, 0x0020, 0x0010,
			0x0008, 0x0004, 0x0002, 0x0001,
		}
		for i, m := range masks {
			g.memory[g.pc] = m
			g.comments[g.pc] = fmt.Sprintf("Maske fuer Bit %d", 15-i)
			g.pc++
		}
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
	if g.nextTempAddr >= g.displayBase {
		g.nextTempAddr = g.tempBaseAddr // Wrap around
	}
	return addr
}

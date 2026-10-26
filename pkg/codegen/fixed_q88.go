package codegen

/*
 ==============================================================================
 🔢 Q8.8 FESTKOMMA-ARITHMETIK & LAUFZEITROUTINEN (SCHRITT 1 FLOATING-POINT)
 ==============================================================================
 Implementiert das vorzeichenbehaftete 16-Bit Q8.8 Festkommaformat (FIXED8):
  - Bit 15: Vorzeichen (Zweierkomplement)
  - Bits 14..8: Ganzzahlanteil (7 Bit + Vorzeichen, -128 .. +127)
  - Bits 7..0: Nachkommaanteil (8 Bit, Auflösung 1/256 = 0.00390625)
  - Wert = int16(W) / 256.0
 ==============================================================================
*/

import (
	"fmt"
	"math"
	"strings"

	"clack-basic/pkg/ast"
)

// Float64ToQ88 konvertiert einen Gleitkommawert mit Rundung zur nächstgelegenen
// darstellbaren Zahl und Sättigung in ein 16-Bit Q8.8 Wort.
func Float64ToQ88(f float64) uint16 {
	scaled := math.Round(f * 256.0)
	if scaled > 32767 {
		scaled = 32767
	} else if scaled < -32768 {
		scaled = -32768
	}
	return uint16(int16(scaled))
}

// Q88ToFloat64 konvertiert ein 16-Bit Q8.8 Wort zurück in float64.
func Q88ToFloat64(w uint16) float64 {
	return float64(int16(w)) / 256.0
}

// IntToQ88 skaliert eine Ganzzahl (int) in das Q8.8 Format (Wert * 256) mit Sättigung.
func IntToQ88(v int) uint16 {
	scaled := v * 256
	if scaled > 32767 {
		scaled = 32767
	} else if scaled < -32768 {
		scaled = -32768
	}
	return uint16(int16(scaled))
}

// Q88ToInt konvertiert ein Q8.8 Wort per Vorzeichen-erhaltender Division durch 256
// (Truncation Richtung Null) in eine 16-Bit Ganzzahl.
func Q88ToInt(w uint16) int16 {
	return int16(w) / 256
}

// Q88Add führt eine Q8.8 Addition im Referenzmodell aus.
func Q88Add(a, b uint16) uint16 {
	return uint16(int16(a) + int16(b))
}

// Q88Sub führt eine Q8.8 Subtraktion im Referenzmodell aus.
func Q88Sub(a, b uint16) uint16 {
	return uint16(int16(a) - int16(b))
}

// Q88Mul berechnet das Q8.8 Produkt (a * b) / 256 im Referenzmodell
// exakt passend zur Relais-Laufzeitroutine __runtime_fmul_q88.
func Q88Mul(a, b uint16) uint16 {
	sa := int32(int16(a))
	sb := int32(int16(b))
	if sa == 0 || sb == 0 {
		return 0
	}
	sign := false
	if sa < 0 {
		sa = -sa
		sign = !sign
	}
	if sb < 0 {
		sb = -sb
		sign = !sign
	}
	prod := (uint32(sa) * uint32(sb)) >> 8
	if prod >= 0x8000 {
		if sign {
			return 0x8000
		}
		return 0x7FFF
	}
	res := uint16(prod)
	if sign {
		res = uint16(-int16(res))
	}
	return res
}

// Q88Div berechnet den Q8.8 Quotienten (a * 256) / b im Referenzmodell
// exakt passend zur Relais-Laufzeitroutine __runtime_fdiv_q88.
func Q88Div(a, b uint16) uint16 {
	sa := int32(int16(a))
	sb := int32(int16(b))
	if sa == 0 {
		return 0
	}
	sign := false
	if sa < 0 {
		sa = -sa
		sign = !sign
	}
	if sb < 0 {
		sb = -sb
		sign = !sign
	}
	if sb == 0 {
		if sign {
			return 0x8000
		}
		return 0x7FFF
	}
	quot := (uint32(sa) << 8) / uint32(sb)
	if quot >= 0x8000 {
		if sign {
			return 0x8000
		}
		return 0x7FFF
	}
	res := uint16(quot)
	if sign {
		res = uint16(-int16(res))
	}
	return res
}

// ============================================================================
// TYPPRÜFUNG & KONVERTIERUNG IM COMPILER
// ============================================================================

// getVarType ermittelt den Datentyp einer Variable (TypeInt16, TypeFixed8 oder TypeFloat16).
func (g *CodeGenerator) getVarType(name string) ast.DataType {
	upper := strings.ToUpper(name)
	if t, exists := g.varTypes[upper]; exists {
		return t
	}
	if strings.HasSuffix(upper, "#") {
		return ast.TypeFloat16
	}
	return ast.TypeInt16
}

// exprType bestimmt den Ergebnistyp eines AST-Ausdrucks.
func (g *CodeGenerator) exprType(expr ast.Expr) ast.DataType {
	if expr == nil {
		return ast.TypeInt16
	}
	switch e := expr.(type) {
	case *ast.NumberLiteral:
		if e.IsFixed {
			return ast.TypeFixed8
		}
		return ast.TypeInt16
	case *ast.VariableExpr:
		return g.getVarType(e.Name)
	case *ast.UnaryExpr:
		return g.exprType(e.Right)
	case *ast.BinaryExpr:
		op := strings.ToUpper(e.Op)
		switch op {
		case "+", "-", "*", "/":
			lType := g.exprType(e.Left)
			rType := g.exprType(e.Right)
			if lType == ast.TypeFloat16 || rType == ast.TypeFloat16 {
				return ast.TypeFloat16
			}
			if lType == ast.TypeFixed8 || rType == ast.TypeFixed8 {
				return ast.TypeFixed8
			}
			return ast.TypeInt16
		case "SHL", "SHR", "MOD":
			return g.exprType(e.Left)
		default:
			// Vergleiche (=, <>, <, <=, >, >=) und Bitlogik liefern 0/1 bzw. Int16
			return ast.TypeInt16
		}
	case *ast.FixedFuncExpr:
		switch strings.ToUpper(e.Name) {
		case "FTOI", "F16TOI", "INT", "FIX", "FCMP", "F16CMP":
			return ast.TypeInt16
		case "F16ADD", "F16SUB", "F16MUL", "F16DIV", "ITOF16", "FLOAT16", "F16", "F16ABS", "F16NEG":
			return ast.TypeFloat16
		case "FIXED8":
			return ast.TypeFixed8
		case "FADD", "FSUB", "FMUL", "FDIV":
			for _, arg := range e.Args {
				if g.exprType(arg) == ast.TypeFloat16 {
					return ast.TypeFloat16
				}
			}
			return ast.TypeFixed8
		case "ITOF", "FABS", "FNEG":
			if len(e.Args) > 0 && g.exprType(e.Args[0]) == ast.TypeFloat16 {
				return ast.TypeFloat16
			}
			return ast.TypeFixed8
		case "ABS":
			if len(e.Args) > 0 {
				return g.exprType(e.Args[0])
			}
			return ast.TypeInt16
		default:
			return ast.TypeFixed8
		}
	default:
		return ast.TypeInt16
	}
}

// evalConstFixed8Expr wertet konstante Ausdrücke als Q8.8 Festkommawert (int16 in int) aus.
// Ganze Zahlen (z.B. 2) werden dabei automatisch mit 256 skaliert (512), Kommazahlen (z.B. 1.5 -> 384) direkt übernommen.
func evalConstFixed8Expr(expr ast.Expr) (int, bool) {
	if expr == nil {
		return 0, false
	}
	switch e := expr.(type) {
	case *ast.NumberLiteral:
		if e.IsFixed {
			return e.Value, true
		}
		return int(int16(IntToQ88(e.Value))), true
	case *ast.UnaryExpr:
		if e.Op == "-" {
			val, ok := evalConstFixed8Expr(e.Right)
			if !ok {
				return 0, false
			}
			return -val, true
		}
		return 0, false
	default:
		return 0, false
	}
}

// compileExprAsFixed8 übersetzt einen Ausdruck in den Accu und stellt sicher,
// dass das Ergebnis im Q8.8 Format vorliegt (automatische ITOF-Skalierung bei Integer-Ausdrücken).
func (g *CodeGenerator) compileExprAsFixed8(expr ast.Expr) error {
	if val, ok := evalConstFixed8Expr(expr); ok {
		g.emit(OpLDA, uint16(val&0xFFFF), fmt.Sprintf("Q8.8 Konstante %d", val))
		return nil
	}
	if g.exprType(expr) == ast.TypeInt16 {
		if err := g.compileExpr(expr); err != nil {
			return err
		}
		g.emitIntToQ88()
		return nil
	}
	return g.compileExpr(expr)
}

// emitIntToQ88 skaliert den Integer-Wert im Accu um Faktor 256 (<< 8) nach Q8.8.
func (g *CodeGenerator) emitIntToQ88() {
	for s := 0; s < 8; s++ {
		g.emit(OpSHL, 0, fmt.Sprintf("ITOF Q8.8 Shift (%d/8)", s+1))
	}
}

// emitQ88ToInt konvertiert den Q8.8-Wert im Accu vorzeichenrichtig nach Integer (/ 256).
func (g *CodeGenerator) emitQ88ToInt() {
	g.needFtoiQ88 = true
	g.emit(OpCALL, 0, "CALL __ftoi_q88")
	g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__runtime_ftoi_q88"})
}

// compileFixedFunc übersetzt explizite Festkomma-, Gleitkomma- und Konvertierungsfunktionen:
// FADD, FSUB, FMUL, FDIV, FCMP, F16ADD, F16SUB, F16MUL, F16DIV, F16CMP,
// ITOF, FTOI, ITOF16, F16TOI, FIXED8, FLOAT16, F16, INT, FIX, ABS, FABS, FNEG, F16ABS, F16NEG.
func (g *CodeGenerator) compileFixedFunc(e *ast.FixedFuncExpr) error {
	name := strings.ToUpper(e.Name)
	switch name {
	case "F16ADD", "F16SUB", "F16MUL", "F16DIV":
		if len(e.Args) != 2 {
			return fmt.Errorf("%s erwartet genau 2 Argumente, erhalten: %d", name, len(e.Args))
		}
		var op string
		switch name {
		case "F16ADD":
			op = "+"
		case "F16SUB":
			op = "-"
		case "F16MUL":
			op = "*"
		case "F16DIV":
			op = "/"
		}
		return g.compileBinaryFloat16WithArgMode(e.Args[0], op, e.Args[1], true)

	case "FADD", "FSUB", "FMUL", "FDIV":
		if len(e.Args) != 2 {
			return fmt.Errorf("%s erwartet genau 2 Argumente, erhalten: %d", name, len(e.Args))
		}
		var op string
		switch name {
		case "FADD":
			op = "+"
		case "FSUB":
			op = "-"
		case "FMUL":
			op = "*"
		case "FDIV":
			op = "/"
		}
		if g.exprType(e.Args[0]) == ast.TypeFloat16 || g.exprType(e.Args[1]) == ast.TypeFloat16 {
			return g.compileBinaryFloat16WithArgMode(e.Args[0], op, e.Args[1], true)
		}
		return g.compileBinaryFixed8WithArgMode(e.Args[0], op, e.Args[1], true)

	case "F16CMP", "FCMP":
		if len(e.Args) != 2 {
			return fmt.Errorf("%s erwartet genau 2 Argumente, erhalten: %d", name, len(e.Args))
		}
		if name == "F16CMP" || g.exprType(e.Args[0]) == ast.TypeFloat16 || g.exprType(e.Args[1]) == ast.TypeFloat16 {
			g.needFSubF16 = true
			g.needFAddF16 = true
			return g.emitF16BinaryCall(e.Args[0], e.Args[1], true, "__f16_sub")
		}
		return g.compileBinaryFixed8WithArgMode(e.Args[0], "-", e.Args[1], true)

	case "ITOF16", "FLOAT16", "F16":
		if len(e.Args) != 1 {
			return fmt.Errorf("%s erwartet genau 1 Argument", name)
		}
		return g.compileExprAsFloat16(e.Args[0])

	case "ITOF", "FIXED8":
		if len(e.Args) != 1 {
			return fmt.Errorf("%s erwartet genau 1 Argument", name)
		}
		arg := e.Args[0]
		if name == "ITOF" && g.exprType(arg) == ast.TypeFloat16 {
			return g.compileExprAsFloat16(arg)
		}
		if val, ok := evalConstFixed8Expr(arg); ok {
			g.emit(OpLDA, uint16(val&0xFFFF), fmt.Sprintf("%s Konstante %d", name, val))
			return nil
		}
		if g.exprType(arg) == ast.TypeFixed8 {
			return g.compileExpr(arg)
		}
		if err := g.compileExpr(arg); err != nil {
			return err
		}
		g.emitIntToQ88()
		return nil

	case "F16TOI":
		if len(e.Args) != 1 {
			return fmt.Errorf("F16TOI erwartet genau 1 Argument")
		}
		arg := e.Args[0]
		if err := g.compileArgForFloat16Op(arg, true); err != nil {
			return err
		}
		g.emitF16ToInt()
		return nil

	case "FTOI", "INT", "FIX":
		if len(e.Args) != 1 {
			return fmt.Errorf("%s erwartet genau 1 Argument", name)
		}
		arg := e.Args[0]
		if g.exprType(arg) == ast.TypeFloat16 {
			if err := g.compileExpr(arg); err != nil {
				return err
			}
			g.emitF16ToInt()
			return nil
		}
		if val, ok := evalConstFixed8Expr(arg); ok && g.exprType(arg) == ast.TypeFixed8 {
			intVal := int(Q88ToInt(uint16(val & 0xFFFF)))
			g.emit(OpLDA, uint16(intVal&0xFFFF), fmt.Sprintf("%s Konstante %d", name, intVal))
			return nil
		}
		if err := g.compileExpr(arg); err != nil {
			return err
		}
		// Auch wenn eine ungetypte Variable einen rohen Q8.8-Wert hält, führt FTOI/INT die Skalierung /256 aus
		if name == "FTOI" || g.exprType(arg) == ast.TypeFixed8 {
			g.emitQ88ToInt()
		}
		return nil

	case "F16ABS":
		if len(e.Args) != 1 {
			return fmt.Errorf("F16ABS erwartet genau 1 Argument")
		}
		if err := g.compileArgForFloat16Op(e.Args[0], true); err != nil {
			return err
		}
		g.emit(OpAND, 0x7FFF, "F16ABS: Vorzeichenbit loeschen")
		return nil

	case "ABS", "FABS":
		if len(e.Args) != 1 {
			return fmt.Errorf("%s erwartet genau 1 Argument", name)
		}
		if g.exprType(e.Args[0]) == ast.TypeFloat16 {
			if err := g.compileExpr(e.Args[0]); err != nil {
				return err
			}
			g.emit(OpAND, 0x7FFF, "F16 ABS: Vorzeichenbit loeschen")
			return nil
		}
		if err := g.compileExpr(e.Args[0]); err != nil {
			return err
		}
		doneLabel := fmt.Sprintf("__abs_done_%d", g.pc)
		g.emit(OpBN, 0, "ABS: Wenn negativ -> negieren")
		negLabel := fmt.Sprintf("__abs_neg_%d", g.pc)
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: negLabel})
		g.emit(OpBA, 0, "ABS: Positiv -> fertig")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: doneLabel})
		g.labels[negLabel] = g.pc
		g.emit(OpNOT, 0, "ABS: ~Accu")
		g.emit(OpADD, 1, "ABS: + 1")
		g.labels[doneLabel] = g.pc
		return nil

	case "F16NEG":
		if len(e.Args) != 1 {
			return fmt.Errorf("F16NEG erwartet genau 1 Argument")
		}
		if err := g.compileArgForFloat16Op(e.Args[0], true); err != nil {
			return err
		}
		g.emitF16Neg()
		return nil

	case "FNEG":
		if len(e.Args) != 1 {
			return fmt.Errorf("FNEG erwartet genau 1 Argument")
		}
		if g.exprType(e.Args[0]) == ast.TypeFloat16 {
			if err := g.compileExprAsFloat16(e.Args[0]); err != nil {
				return err
			}
			g.emitF16Neg()
			return nil
		}
		if err := g.compileExprAsFixed8(e.Args[0]); err != nil {
			return err
		}
		g.emit(OpNOT, 0, "FNEG: ~Accu")
		g.emit(OpADD, 1, "FNEG: + 1")
		return nil

	default:
		return fmt.Errorf("Unbekannte Funktion: %s", name)
	}
}

// compileArgForFixedFunc übersetzt ein Argument für FADD/FSUB/FMUL/FDIV.
// Wurde eine Variable explizit als INTEGER deklariert, wird sie nach Q8.8 skaliert;
// wurde sie nicht mit DIM AS INTEGER deklariert, wird ihr gespeicherter 16-Bit-Wert direkt als Q8.8 genutzt.
func (g *CodeGenerator) compileArgForFixedOp(expr ast.Expr, explicitFunc bool) error {
	if !explicitFunc {
		return g.compileExprAsFixed8(expr)
	}
	if v, ok := expr.(*ast.VariableExpr); ok {
		upper := strings.ToUpper(v.Name)
		if g.explicitVarTypes[upper] && g.varTypes[upper] == ast.TypeInt16 {
			if err := g.compileExpr(expr); err != nil {
				return err
			}
			g.emitIntToQ88()
			return nil
		}
		return g.compileExpr(expr)
	}
	if n, ok := expr.(*ast.NumberLiteral); ok && !n.IsFixed {
		// In expliziten Aufrufen wie FMUL(A, 2.0) ist 2.0 IsFixed=true.
		// Wenn jemand ein Hex-Wort wie FMUL(0x0180, 0x0200) übergibt oder Integer,
		// prüfen wir: kleine Dezimalzahlen oder Q8.8? In compileExprAsFixed8 werden Literale skaliert.
		return g.compileExprAsFixed8(expr)
	}
	return g.compileExprAsFixed8(expr)
}

// compileBinaryFixed8 übersetzt +, -, *, / sowie Vergleiche auf Q8.8 Festkommawerten.
func (g *CodeGenerator) compileBinaryFixed8(e *ast.BinaryExpr) error {
	return g.compileBinaryFixed8WithArgMode(e.Left, e.Op, e.Right, false)
}

func (g *CodeGenerator) compileBinaryFixed8WithArgMode(left ast.Expr, rawOp string, right ast.Expr, explicitFunc bool) error {
	op := strings.ToUpper(rawOp)

	rConstVal, rIsConst := evalConstFixed8Expr(right)
	lConstVal, lIsConst := evalConstFixed8Expr(left)

	// 1. Q8.8 Multiplikation (*)
	if op == "*" {
		// Optimierung: X * 0.0 -> 0
		if (rIsConst && rConstVal == 0) || (lIsConst && lConstVal == 0) {
			g.emit(OpLDA, 0, "Q8.8 Accu = 0 (* 0.0)")
			return nil
		}
		// Optimierung: X * 1.0 (0x0100 = 256) -> X
		if rIsConst && rConstVal == 256 {
			return g.compileArgForFixedOp(left, explicitFunc)
		}
		if lIsConst && lConstVal == 256 {
			return g.compileArgForFixedOp(right, explicitFunc)
		}

		g.needFMulQ88 = true
		if err := g.compileArgForFixedOp(left, explicitFunc); err != nil {
			return err
		}
		g.emit(OpSTAM, g.getOrCreateVar("__r0"), "Q8.8 Multiplikand __r0")

		// Quadrat-Optimierung: X * X
		if vL, okL := left.(*ast.VariableExpr); okL {
			if vR, okR := right.(*ast.VariableExpr); okR && strings.EqualFold(vL.Name, vR.Name) {
				g.emit(OpSTAM, g.getOrCreateVar("__r1"), "Q8.8 Multiplikator __r1 (Quadrat)")
				g.emit(OpCALL, 0, "CALL __fmul_q88")
				g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__runtime_fmul_q88"})
				return nil
			}
		}

		if err := g.compileArgForFixedOp(right, explicitFunc); err != nil {
			return err
		}
		g.emit(OpSTAM, g.getOrCreateVar("__r1"), "Q8.8 Multiplikator __r1")

		g.emit(OpCALL, 0, "CALL __fmul_q88")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__runtime_fmul_q88"})
		return nil
	}

	// 2. Q8.8 Division (/)
	if op == "/" {
		// Optimierung: X / 1.0 (0x0100 = 256) -> X
		if rIsConst && rConstVal == 256 {
			return g.compileArgForFixedOp(left, explicitFunc)
		}

		g.needFDivQ88 = true
		if err := g.compileArgForFixedOp(left, explicitFunc); err != nil {
			return err
		}
		g.emit(OpSTAM, g.getOrCreateVar("__r0"), "Q8.8 Dividend __r0")

		if err := g.compileArgForFixedOp(right, explicitFunc); err != nil {
			return err
		}
		g.emit(OpSTAM, g.getOrCreateVar("__r1"), "Q8.8 Divisor __r1")

		g.emit(OpCALL, 0, "CALL __fdiv_q88")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__runtime_fdiv_q88"})
		return nil
	}

	// 3. Q8.8 Addition (+) und Subtraktion (-) mit Konstante rechts
	if rIsConst && (op == "+" || op == "-") {
		if err := g.compileArgForFixedOp(left, explicitFunc); err != nil {
			return err
		}
		if (rConstVal & 0xFFFF) != 0 {
			if op == "+" {
				g.emit(OpADD, uint16(rConstVal&0xFFFF), fmt.Sprintf("Q8.8 Accu + %d", rConstVal))
			} else {
				g.emit(OpADD, uint16((-rConstVal)&0xFFFF), fmt.Sprintf("Q8.8 Accu - %d", rConstVal))
			}
		}
		return nil
	}

	// 4. Q8.8 Addition (+) und Subtraktion (-) mit Konstante links
	if lIsConst && (op == "+" || op == "-") {
		if err := g.compileArgForFixedOp(right, explicitFunc); err != nil {
			return err
		}
		if op == "+" {
			if (lConstVal & 0xFFFF) != 0 {
				g.emit(OpADD, uint16(lConstVal&0xFFFF), fmt.Sprintf("Q8.8 Accu + %d", lConstVal))
			}
		} else {
			g.emit(OpNOT, 0, "~Right")
			if ((lConstVal + 1) & 0xFFFF) != 0 {
				g.emit(OpADD, uint16((lConstVal+1)&0xFFFF), fmt.Sprintf("Q8.8 %d - Right", lConstVal))
			}
		}
		return nil
	}

	// 5. Schnelle Patch-Optimierung wenn rechter Operand eine bereits als Q8.8 vorliegende Variable ist
	if vRight, ok := right.(*ast.VariableExpr); ok && (op == "+" || op == "-") {
		rightNeedsScale := g.exprType(vRight) == ast.TypeInt16 && (!explicitFunc || g.explicitVarTypes[strings.ToUpper(vRight.Name)])
		if !rightNeedsScale {
			vAddr := g.getOrCreateVar(vRight.Name)
			if op == "+" {
				g.emit(OpLDAM, vAddr, fmt.Sprintf("Lade %s fuer Q8.8 ADD Patch", vRight.Name))
				patchAddr := g.pc + 5
				g.emit(OpSTAM, patchAddr, "Patch ADD Operand")
				if err := g.compileArgForFixedOp(left, explicitFunc); err != nil {
					return err
				}
				g.emit(OpADD, 0, fmt.Sprintf("Q8.8 A + %s", vRight.Name))
				return nil
			}
			g.emit(OpLDAM, vAddr, fmt.Sprintf("Lade %s fuer Q8.8 SUB Negation", vRight.Name))
			g.emit(OpNOT, 0, fmt.Sprintf("~%s", vRight.Name))
			g.emit(OpADD, 1, fmt.Sprintf("-%s", vRight.Name))
			patchAddr := g.pc + 5
			g.emit(OpSTAM, patchAddr, "Patch ADD Operand mit -B")
			if err := g.compileArgForFixedOp(left, explicitFunc); err != nil {
				return err
			}
			g.emit(OpADD, 0, fmt.Sprintf("Q8.8 A - %s", vRight.Name))
			return nil
		}
	}

	// 6. Allgemeine Auswertung für Q8.8 +, - und Vergleiche
	if err := g.compileArgForFixedOp(right, explicitFunc); err != nil {
		return err
	}
	tempRight := g.allocTemp()
	g.emit(OpSTAM, tempRight, "Q8.8 Rechter Operand")

	if err := g.compileArgForFixedOp(left, explicitFunc); err != nil {
		return err
	}

	switch op {
	case "+":
		tempA := g.allocTemp()
		g.emit(OpSTAM, tempA, "Temp A sichern")
		g.emit(OpLDAM, tempRight, "Lade B")
		patchAddr := g.pc + 5
		g.emit(OpSTAM, patchAddr, "Patch ADD Operand")
		g.emit(OpLDAM, tempA, "Lade A zurueck")
		g.emit(OpADD, 0, "Q8.8 A + B")
	case "-":
		tempA := g.allocTemp()
		g.emit(OpSTAM, tempA, "Temp A sichern")
		g.emit(OpLDAM, tempRight, "Lade B")
		g.emit(OpNOT, 0, "~B")
		g.emit(OpADD, 1, "-B")
		patchAddr := g.pc + 5
		g.emit(OpSTAM, patchAddr, "Patch ADD Operand mit -B")
		g.emit(OpLDAM, tempA, "Lade A zurueck")
		g.emit(OpADD, 0, "Q8.8 A - B")
	case "=", "==":
		return g.compileComparison(tempRight, OpBZ)
	case "<>", "!=":
		return g.compileComparison(tempRight, OpBZ, true)
	case "<":
		return g.compileComparison(tempRight, OpBN)
	case ">=":
		return g.compileComparison(tempRight, OpBN, true)
	case ">":
		return g.compileComparisonReverse(tempRight, OpBN)
	case "<=":
		return g.compileComparisonReverse(tempRight, OpBN, true)
	}
	return nil
}

// ============================================================================
// MASCHINEN-LAUFZEITROUTINEN FÜR Q8.8 (__fmul_q88, __fdiv_q88, __ftoi_q88)
// ============================================================================

func (g *CodeGenerator) emitFixedQ88Routines() {
	// 1. Q8.8 Multiplikation: __runtime_fmul_q88 (__r0 * __r1 >> 8 -> Accu)
	// Verwendet ein exaktes 32-Bit Zwischenprodukt (prod_hi:prod_lo), Vorzeichenbehandlung
	// und Overflow-Sättigung auf 0x7FFF (+127.996) bzw. 0x8000 (-128.0).
	// Kein verschachteltes CALL (1-Level Hardware-Return-Stack sicher!).
	if g.needFMulQ88 {
		g.labels["__runtime_fmul_q88"] = g.pc
		r0Addr := g.getOrCreateVar("__r0")
		r1Addr := g.getOrCreateVar("__r1")
		prodLoAddr := g.getOrCreateVar("__prod")
		aHiAddr := g.getOrCreateVar("__quot")
		prodHiAddr := g.getOrCreateVar("__rem")
		signAddr := g.getOrCreateVar("__fp_sign")

		g.emit(OpLDA, 0, "__fmul_q88: Init 0")
		g.emit(OpSTAM, prodLoAddr, "prod_lo = 0")
		g.emit(OpSTAM, prodHiAddr, "prod_hi = 0")
		g.emit(OpSTAM, aHiAddr, "a_hi = 0")
		g.emit(OpSTAM, signAddr, "sign = 0")

		// Prüfe Operand A (__r0)
		g.emit(OpLDAM, r0Addr, "__fmul_q88: Lade A")
		g.emit(OpBZ, 0, "Wenn A == 0 -> Return 0")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fmul_q88_ret_zero"})
		g.emit(OpBN, 0, "Wenn A < 0 -> Negiere A")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fmul_q88_a_neg"})
		g.emit(OpBA, 0, "Weiter zu B")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fmul_q88_check_b"})

		g.labels["__fmul_q88_a_neg"] = g.pc
		g.emit(OpNOT, 0, "~A")
		g.emit(OpADD, 1, "|A| = -A")
		g.emit(OpSTAM, r0Addr, "Speichere |A|")
		g.emit(OpLDA, 1, "sign = 1")
		g.emit(OpSTAM, signAddr, "Speichere sign")

		// Prüfe Operand B (__r1)
		g.labels["__fmul_q88_check_b"] = g.pc
		g.emit(OpLDAM, r1Addr, "__fmul_q88: Lade B")
		g.emit(OpBZ, 0, "Wenn B == 0 -> Return 0")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fmul_q88_ret_zero"})
		g.emit(OpBN, 0, "Wenn B < 0 -> Negiere B")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fmul_q88_b_neg"})
		g.emit(OpBA, 0, "Starte 32-Bit Shift-Add Loop")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fmul_q88_loop"})

		g.labels["__fmul_q88_b_neg"] = g.pc
		g.emit(OpNOT, 0, "~B")
		g.emit(OpADD, 1, "|B| = -B")
		g.emit(OpSTAM, r1Addr, "Speichere |B|")
		g.emit(OpLDAM, signAddr, "Lade sign")
		g.emit(OpADD, 1, "sign + 1")
		g.emit(OpAND, 1, "sign XOR 1")
		g.emit(OpSTAM, signAddr, "Speichere sign")
		g.emit(OpLDAM, r1Addr, "Lade |B| fuer Loop")

		// 32-Bit Shift-and-Add Schleife
		loopPC := g.pc
		g.labels["__fmul_q88_loop"] = loopPC

		g.emit(OpBE, 0, "Wenn LSB(B) == 0 -> Skip Add")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fmul_q88_skip_add"})

		// prod_lo = prod_lo + a_lo
		g.emit(OpLDAM, r0Addr, "Lade a_lo")
		patchAddLo := g.pc + 5
		g.emit(OpSTAM, patchAddLo, "Patch ADD a_lo")
		g.emit(OpLDAM, prodLoAddr, "Lade prod_lo")
		g.emit(OpADD, 0, "prod_lo + a_lo")
		g.emit(OpSTAM, prodLoAddr, "Speichere prod_lo")
		g.emit(OpBC, 0, "Wenn Carry -> prod_hi++")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fmul_q88_carry"})
		g.emit(OpBA, 0, "Weiter zu prod_hi + a_hi")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fmul_q88_add_hi"})

		g.labels["__fmul_q88_carry"] = g.pc
		g.emit(OpLDAM, prodHiAddr, "Lade prod_hi")
		g.emit(OpADD, 1, "prod_hi + 1")
		g.emit(OpSTAM, prodHiAddr, "Speichere prod_hi")

		g.labels["__fmul_q88_add_hi"] = g.pc
		g.emit(OpLDAM, aHiAddr, "Lade a_hi")
		patchAddHi := g.pc + 5
		g.emit(OpSTAM, patchAddHi, "Patch ADD a_hi")
		g.emit(OpLDAM, prodHiAddr, "Lade prod_hi")
		g.emit(OpADD, 0, "prod_hi + a_hi")
		g.emit(OpSTAM, prodHiAddr, "Speichere prod_hi")

		g.labels["__fmul_q88_skip_add"] = g.pc
		g.emit(OpLDAM, r1Addr, "Lade B")
		g.emit(OpSHR, 0, "B >> 1")
		g.emit(OpBZ, 0, "Wenn B == 0 -> Skalierung")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fmul_q88_scale"})
		g.emit(OpSTAM, r1Addr, "Speichere B")

		// Shift (a_hi:a_lo) um 1 Bit nach links
		g.emit(OpLDAM, aHiAddr, "Lade a_hi")
		g.emit(OpSHL, 0, "a_hi << 1")
		g.emit(OpSTAM, aHiAddr, "Speichere a_hi")

		g.emit(OpLDAM, r0Addr, "Lade a_lo")
		g.emit(OpSHL, 0, "a_lo << 1")
		g.emit(OpSTAM, r0Addr, "Speichere a_lo")
		g.emit(OpBC, 0, "Wenn Carry aus a_lo -> a_hi |= 1")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fmul_q88_shl_carry"})
		g.emit(OpLDAM, r1Addr, "Lade B fuer naechste Iteration")
		g.emit(OpBA, loopPC, "Loop __fmul_q88")

		g.labels["__fmul_q88_shl_carry"] = g.pc
		g.emit(OpLDAM, aHiAddr, "Lade a_hi")
		g.emit(OpOR, 1, "a_hi | 1")
		g.emit(OpSTAM, aHiAddr, "Speichere a_hi")
		g.emit(OpLDAM, r1Addr, "Lade B fuer naechste Iteration")
		g.emit(OpBA, loopPC, "Loop __fmul_q88")

		// Skalierung >> 8 und Overflow-Prüfung
		g.labels["__fmul_q88_scale"] = g.pc
		g.emit(OpLDAM, prodHiAddr, "Lade prod_hi")
		g.emit(OpAND, 0xFF80, "Pruefe Overflow (prod_hi >= 128)")
		g.emit(OpBZ, 0, "Kein Overflow -> Kombiniere Bytes")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fmul_q88_no_ovf"})

		// Overflow-Sättigung
		g.emit(OpLDAM, signAddr, "Lade sign bei Overflow")
		g.emit(OpBZ, 0, "Wenn positiv -> 0x7FFF")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fmul_q88_ovf_pos"})
		g.emit(OpLDA, 0x8000, "Overflow negativ -> 0x8000 (-128.0)")
		g.emitSingle(OpRET, "RETURN __fmul_q88 (ovf neg)")

		g.labels["__fmul_q88_ovf_pos"] = g.pc
		g.emit(OpLDA, 0x7FFF, "Overflow positiv -> 0x7FFF (+127.996)")
		g.emitSingle(OpRET, "RETURN __fmul_q88 (ovf pos)")

		g.labels["__fmul_q88_no_ovf"] = g.pc
		g.emit(OpLDAM, prodHiAddr, "Lade prod_hi")
		for s := 0; s < 8; s++ {
			g.emit(OpSHL, 0, "prod_hi << 8")
		}
		g.emit(OpSTAM, prodHiAddr, "Speichere prod_hi << 8")

		g.emit(OpLDAM, prodLoAddr, "Lade prod_lo")
		for s := 0; s < 8; s++ {
			g.emit(OpSHR, 0, "prod_lo >> 8")
		}
		patchCombine := g.pc + 5
		g.emit(OpSTAM, patchCombine, "Patch ADD (prod_lo >> 8)")
		g.emit(OpLDAM, prodHiAddr, "Lade (prod_hi << 8)")
		g.emit(OpADD, 0, "(prod_hi << 8) + (prod_lo >> 8)")
		g.emit(OpSTAM, prodLoAddr, "Speichere Betrag")

		g.emit(OpLDAM, signAddr, "Pruefe Vorzeichen")
		g.emit(OpBZ, 0, "Wenn positiv -> Fertig")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fmul_q88_done"})
		g.emit(OpLDAM, prodLoAddr, "Lade Betrag")
		g.emit(OpNOT, 0, "~Betrag")
		g.emit(OpADD, 1, "-Betrag")
		g.emitSingle(OpRET, "RETURN __fmul_q88 (negativ)")

		g.labels["__fmul_q88_done"] = g.pc
		g.emit(OpLDAM, prodLoAddr, "Lade Ergebnis")
		g.emitSingle(OpRET, "RETURN __fmul_q88")

		g.labels["__fmul_q88_ret_zero"] = g.pc
		g.emit(OpLDA, 0, "Ergebnis = 0")
		g.emitSingle(OpRET, "RETURN __fmul_q88 (0)")
	}

	// 2. Q8.8 Division: __runtime_fdiv_q88 ((__r0 << 8) / __r1 -> Accu)
	// 16-Schritt Restoring-Division über den 24-Bit Dividenden (rem = |A|>>8, num_lo = (|A|&0xFF)<<8).
	if g.needFDivQ88 {
		g.labels["__runtime_fdiv_q88"] = g.pc
		r0Addr := g.getOrCreateVar("__r0")
		r1Addr := g.getOrCreateVar("__r1")
		quotAddr := g.getOrCreateVar("__quot")
		remAddr := g.getOrCreateVar("__rem")
		cntAddr := g.getOrCreateVar("__prod")
		signAddr := g.getOrCreateVar("__fp_sign")

		g.emit(OpLDA, 0, "__fdiv_q88: Init sign = 0")
		g.emit(OpSTAM, signAddr, "sign = 0")
		g.emit(OpSTAM, quotAddr, "quot = 0")

		// Prüfe Dividend A (__r0)
		g.emit(OpLDAM, r0Addr, "__fdiv_q88: Lade Dividend A")
		g.emit(OpBZ, 0, "Wenn A == 0 -> Return 0")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fdiv_q88_ret_zero"})
		g.emit(OpBN, 0, "Wenn A < 0 -> Negiere A")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fdiv_q88_a_neg"})
		g.emit(OpBA, 0, "Weiter zu Divisor B")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fdiv_q88_check_b"})

		g.labels["__fdiv_q88_a_neg"] = g.pc
		g.emit(OpNOT, 0, "~A")
		g.emit(OpADD, 1, "|A| = -A")
		g.emit(OpSTAM, r0Addr, "Speichere |A|")
		g.emit(OpLDA, 1, "sign = 1")
		g.emit(OpSTAM, signAddr, "Speichere sign")

		// Prüfe Divisor B (__r1)
		g.labels["__fdiv_q88_check_b"] = g.pc
		g.emit(OpLDAM, r1Addr, "__fdiv_q88: Lade Divisor B")
		g.emit(OpBZ, 0, "Wenn B == 0 -> Division durch 0 (Sättigung)")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fdiv_q88_ovf"})
		g.emit(OpBN, 0, "Wenn B < 0 -> Negiere B")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fdiv_q88_b_neg"})
		g.emit(OpBA, 0, "Vorbereitung 24-Bit Dividend")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fdiv_q88_prep"})

		g.labels["__fdiv_q88_b_neg"] = g.pc
		g.emit(OpNOT, 0, "~B")
		g.emit(OpADD, 1, "|B| = -B")
		g.emit(OpSTAM, r1Addr, "Speichere |B|")
		g.emit(OpLDAM, signAddr, "Lade sign")
		g.emit(OpADD, 1, "sign + 1")
		g.emit(OpAND, 1, "sign XOR 1")
		g.emit(OpSTAM, signAddr, "Speichere sign")

		// 24-Bit Dividend aufteilen: rem = |A| >> 8, num_lo = (|A| & 0xFF) << 8
		g.labels["__fdiv_q88_prep"] = g.pc
		g.emit(OpLDAM, r0Addr, "Lade |A| fuer rem")
		for s := 0; s < 8; s++ {
			g.emit(OpSHR, 0, "|A| >> 8")
		}
		g.emit(OpSTAM, remAddr, "rem = |A| >> 8")

		g.emit(OpLDAM, r0Addr, "Lade |A| fuer num_lo")
		g.emit(OpAND, 0x00FF, "|A| & 0xFF")
		for s := 0; s < 8; s++ {
			g.emit(OpSHL, 0, "(|A| & 0xFF) << 8")
		}
		g.emit(OpSTAM, r0Addr, "num_lo = (|A| & 0xFF) << 8")

		// Berechne negB = -|B| und patche beide Subtraktionsstellen
		g.emit(OpLDAM, r1Addr, "Lade |B|")
		g.emit(OpNOT, 0, "~|B|")
		g.emit(OpADD, 1, "-|B|")
		g.emit(OpSTAM, 0, "Patch Vorab-Overflow-Check")
		fixPreOvf := g.pc - 1
		g.emit(OpSTAM, 0, "Patch Loop-Subtraktion")
		fixLoopSub := g.pc - 1

		// Vorab-Overflow-Check: Wenn rem >= |B| (d.h. Quotient >= 65536) -> Overflow!
		g.emit(OpLDAM, remAddr, "Lade initiales rem")
		g.emit(OpADD, 0, "rem - |B| (Vorab-Check)")
		g.memory[fixPreOvf] = g.pc - 1
		g.emit(OpBC, 0, "Wenn rem >= |B| -> Overflow")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fdiv_q88_ovf"})

		g.emit(OpLDA, 16, "Init Schleifenzaehler = 16")
		g.emit(OpSTAM, cntAddr, "cnt = 16")

		divLoopPC := g.pc
		g.labels["__fdiv_q88_loop"] = divLoopPC

		// quot <<= 1
		g.emit(OpLDAM, quotAddr, "Lade quot")
		g.emit(OpSHL, 0, "quot << 1")
		g.emit(OpSTAM, quotAddr, "Speichere quot")

		// rem <<= 1
		g.emit(OpLDAM, remAddr, "Lade rem")
		g.emit(OpSHL, 0, "rem << 1")
		g.emit(OpSTAM, remAddr, "Speichere rem")

		// num_lo <<= 1; Carry in Bit 0 von rem übernehmen
		g.emit(OpLDAM, r0Addr, "Lade num_lo")
		g.emit(OpSHL, 0, "num_lo << 1")
		g.emit(OpSTAM, r0Addr, "Speichere num_lo")
		g.emit(OpBC, 0, "Wenn Bit 15 von num_lo == 1 -> rem |= 1")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fdiv_q88_bit_in"})
		g.emit(OpBA, 0, "Weiter zu Subtraktion")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fdiv_q88_try_sub"})

		g.labels["__fdiv_q88_bit_in"] = g.pc
		g.emit(OpLDAM, remAddr, "Lade rem")
		g.emit(OpOR, 1, "rem | 1")
		g.emit(OpSTAM, remAddr, "Speichere rem")

		g.labels["__fdiv_q88_try_sub"] = g.pc
		g.emit(OpLDAM, remAddr, "Lade rem")
		g.emit(OpADD, 0, "rem + (-|B|)")
		g.memory[fixLoopSub] = g.pc - 1
		g.emit(OpBC, 0, "Wenn Carry == 1 (rem >= |B|) -> Subtraktion ok")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fdiv_q88_sub_ok"})
		g.emit(OpBA, 0, "rem < |B| -> Naechster Schritt")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fdiv_q88_next"})

		g.labels["__fdiv_q88_sub_ok"] = g.pc
		g.emit(OpSTAM, remAddr, "rem = rem - |B|")
		g.emit(OpLDAM, quotAddr, "Lade quot")
		g.emit(OpOR, 1, "quot |= 1")
		g.emit(OpSTAM, quotAddr, "Speichere quot")

		g.labels["__fdiv_q88_next"] = g.pc
		g.emit(OpLDAM, cntAddr, "Lade cnt")
		g.emit(OpADD, 0xFFFF, "cnt - 1")
		g.emit(OpSTAM, cntAddr, "Speichere cnt")
		g.emit(OpBZ, 0, "Wenn cnt == 0 -> Fertig")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fdiv_q88_after_loop"})
		g.emit(OpBA, divLoopPC, "Loop __fdiv_q88")

		g.labels["__fdiv_q88_after_loop"] = g.pc
		g.emit(OpLDAM, quotAddr, "Lade quot")
		g.emit(OpBN, 0, "Wenn quot >= 0x8000 -> Overflow")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fdiv_q88_ovf"})

		g.emit(OpLDAM, signAddr, "Pruefe Vorzeichen")
		g.emit(OpBZ, 0, "Wenn positiv -> Fertig")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fdiv_q88_done"})
		g.emit(OpLDAM, quotAddr, "Lade quot")
		g.emit(OpNOT, 0, "~quot")
		g.emit(OpADD, 1, "-quot")
		g.emitSingle(OpRET, "RETURN __fdiv_q88 (negativ)")

		g.labels["__fdiv_q88_done"] = g.pc
		g.emit(OpLDAM, quotAddr, "Lade Ergebnis quot")
		g.emitSingle(OpRET, "RETURN __fdiv_q88")

		g.labels["__fdiv_q88_ovf"] = g.pc
		g.emit(OpLDAM, signAddr, "Lade sign bei Overflow")
		g.emit(OpBZ, 0, "Wenn positiv -> 0x7FFF")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__fdiv_q88_ovf_pos"})
		g.emit(OpLDA, 0x8000, "Overflow negativ -> 0x8000 (-128.0)")
		g.emitSingle(OpRET, "RETURN __fdiv_q88 (ovf neg)")

		g.labels["__fdiv_q88_ovf_pos"] = g.pc
		g.emit(OpLDA, 0x7FFF, "Overflow positiv -> 0x7FFF (+127.996)")
		g.emitSingle(OpRET, "RETURN __fdiv_q88 (ovf pos)")

		g.labels["__fdiv_q88_ret_zero"] = g.pc
		g.emit(OpLDA, 0, "Ergebnis = 0")
		g.emitSingle(OpRET, "RETURN __fdiv_q88 (0)")
	}

	// 3. Q8.8 nach Integer Konvertierung: __runtime_ftoi_q88 (Accu / 256 Richtung 0 -> Accu)
	if g.needFtoiQ88 {
		g.labels["__runtime_ftoi_q88"] = g.pc
		g.emit(OpBN, 0, "__ftoi_q88: Wenn negativ -> Betrag schieben und negieren")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__ftoi_q88_neg"})
		for s := 0; s < 8; s++ {
			g.emit(OpSHR, 0, "Positiv >> 8")
		}
		g.emitSingle(OpRET, "RETURN __ftoi_q88 (positiv)")

		g.labels["__ftoi_q88_neg"] = g.pc
		g.emit(OpNOT, 0, "~Accu")
		g.emit(OpADD, 1, "|Accu|")
		for s := 0; s < 8; s++ {
			g.emit(OpSHR, 0, "|Accu| >> 8")
		}
		g.emit(OpNOT, 0, "~(|Accu| >> 8)")
		g.emit(OpADD, 1, "-(|Accu| >> 8)")
		g.emitSingle(OpRET, "RETURN __ftoi_q88 (negativ)")
	}
}

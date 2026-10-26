package codegen

/*
 ==============================================================================
 🧮 CLACK-F16 GLEITKOMMA-ARITHMETIK & LAUFZEITROUTINEN (SCHRITT 2 FLOATING-POINT)
 ==============================================================================
 Implementiert das 16-Bit CLACK-F16 Gleitkommaformat (S1-E7-M8 Zuse-Style):
  - Bit 15:    Vorzeichen S (1 Bit: 0 = positiv, 1 = negativ)
  - Bits 14..8: Exponent E (7 Bit mit Bias 63, E = 1..127 normalisiert, E = 0 -> 0.0)
  - Bits 7..0:  Mantisse M (8 Bit Bruchanteil mit impliziter führender 1 bei 0x0100)
  - Wert = (-1)^S * (1 + M / 256) * 2^(E - 63)
  - Kanonische Null: 0x0000
 ==============================================================================
*/

import (
	"fmt"
	"math"
	"strings"

	"clack-basic/pkg/ast"
)

const (
	F16Bias    = 63
	F16Zero    = uint16(0x0000)
	F16One     = uint16(0x3F00) // (1 + 0/256) * 2^(63-63) = 1.0
	F16NegOne  = uint16(0xBF00) // -1.0
	F16MaxPos  = uint16(0x7FFF) // S=0, E=127, M=255
	F16MaxNeg  = uint16(0xFFFF) // S=1, E=127, M=255
)

// Float64ToF16 konvertiert einen float64-Wert in ein 16-Bit CLACK-F16 Wort
// (S1-E7-M8, Bias 63, implizite 1).
func Float64ToF16(f float64) uint16 {
	if f == 0 || math.IsNaN(f) {
		return F16Zero
	}
	var sign uint16
	if f < 0 {
		sign = 0x8000
		f = -f
	}
	if math.IsInf(f, 1) {
		return sign | F16MaxPos
	}

	frac, exp := math.Frexp(f) // f = frac * 2^exp mit 0.5 <= frac < 1.0
	norm := frac * 2.0         // 1.0 <= norm < 2.0
	e := exp - 1 + F16Bias

	m := int(math.Round((norm - 1.0) * 256.0))
	if m >= 256 {
		m = 0
		e++
	}

	if e <= 0 {
		return F16Zero
	}
	if e >= 128 {
		return sign | F16MaxPos
	}

	return sign | (uint16(e&0x7F) << 8) | uint16(m&0xFF)
}

// F16ToFloat64 konvertiert ein 16-Bit CLACK-F16 Wort zurück nach float64.
func F16ToFloat64(w uint16) float64 {
	if (w & 0x7FFF) == 0 {
		return 0.0
	}
	sign := (w & 0x8000) != 0
	exp := int((w>>8)&0x7F) - F16Bias
	mant := 1.0 + float64(w&0x00FF)/256.0
	val := math.Ldexp(mant, exp)
	if sign {
		return -val
	}
	return val
}

// IntToF16 konvertiert eine 16-Bit Ganzzahl exakt passend zur Relais-Routine __f16_itof
// in ein CLACK-F16 Wort.
func IntToF16(v int) uint16 {
	sv := int32(int16(v))
	if sv == 0 {
		return F16Zero
	}
	var sign uint16
	var mag uint16
	if sv < 0 {
		sign = 0x8000
		mag = uint16(-sv)
	} else {
		mag = uint16(sv)
	}

	exp := uint16(F16Bias + 8) // 71: Position für Bit 8 (0x0100)
	if (mag & 0xFE00) != 0 {
		for (mag & 0xFE00) != 0 {
			mag >>= 1
			exp++
		}
	} else {
		for (mag & 0x0100) == 0 {
			mag <<= 1
			exp--
		}
	}
	return sign | ((exp & 0x7F) << 8) | (mag & 0x00FF)
}

// F16ToInt konvertiert ein CLACK-F16 Wort per Truncation Richtung 0 in eine 16-Bit Ganzzahl,
// exakt passend zur Relais-Laufzeitroutine __f16_ftoi.
func F16ToInt(w uint16) int16 {
	if (w & 0x7FFF) == 0 {
		return 0
	}
	sign := (w & 0x8000) != 0
	exp := int((w >> 8) & 0x7F)
	if exp < F16Bias {
		return 0
	}
	if exp >= F16Bias+15 {
		if sign {
			return -32768
		}
		return 32767
	}
	mant := uint16((w & 0x00FF) | 0x0100)
	targetExp := F16Bias + 8 // 71
	if exp < targetExp {
		mant >>= uint(targetExp - exp)
	} else if exp > targetExp {
		mant <<= uint(exp - targetExp)
	}
	if sign {
		return -int16(mant)
	}
	return int16(mant)
}

// F16Add führt eine CLACK-F16 Addition im Referenzmodell aus,
// exakt passend zur Relais-Laufzeitroutine __f16_add.
func F16Add(a, b uint16) uint16 {
	magA := a & 0x7FFF
	magB := b & 0x7FFF
	if magA == 0 {
		if magB == 0 {
			return F16Zero
		}
		return b
	}
	if magB == 0 {
		return a
	}

	// Stelle sicher, dass |a| >= |b|
	if magA < magB {
		a, b = b, a
	}

	signA := a & 0x8000
	signB := b & 0x8000
	expA := int((a >> 8) & 0x7F)
	expB := int((b >> 8) & 0x7F)
	mantA := uint16((a & 0x00FF) | 0x0100)
	mantB := uint16((b & 0x00FF) | 0x0100)

	dE := expA - expB
	if dE >= 9 {
		return a
	}
	if dE > 0 {
		mantB >>= uint(dE)
	}

	if signA == signB {
		mr := mantA + mantB
		if (mr & 0x0200) != 0 {
			mr >>= 1
			expA++
			if expA >= 128 {
				return signA | F16MaxPos
			}
		}
		return signA | (uint16(expA) << 8) | (mr & 0x00FF)
	}

	// Unterschiedliche Vorzeichen: Subtraktion der Beträge (mantA >= mantB garantiert)
	mr := mantA - mantB
	if mr == 0 {
		return F16Zero
	}
	for (mr & 0x0100) == 0 {
		expA--
		if expA == 0 {
			return F16Zero
		}
		mr <<= 1
	}
	return signA | (uint16(expA) << 8) | (mr & 0x00FF)
}

// F16Sub führt eine CLACK-F16 Subtraktion (a - b) im Referenzmodell aus,
// exakt passend zur Relais-Laufzeitroutine __f16_sub.
func F16Sub(a, b uint16) uint16 {
	if (b & 0x7FFF) == 0 {
		if (a & 0x7FFF) == 0 {
			return F16Zero
		}
		return a
	}
	return F16Add(a, b^0x8000)
}

// F16Mul führt eine CLACK-F16 Multiplikation (a * b) im Referenzmodell aus,
// exakt passend zur Relais-Laufzeitroutine __f16_mul.
func F16Mul(a, b uint16) uint16 {
	if (a&0x7FFF) == 0 || (b&0x7FFF) == 0 {
		return F16Zero
	}
	signR := (a ^ b) & 0x8000
	expA := int((a >> 8) & 0x7F)
	expB := int((b >> 8) & 0x7F)
	expR := expA + expB - F16Bias

	fA := uint16(a & 0x00FF)
	fB := uint16(b & 0x00FF)
	prod := fA * fB // 8x8 Bit in 16 Bit (max 255*255 = 65025)
	mr := uint16(256) + fA + fB + (prod >> 8)

	if (mr & 0x0200) != 0 {
		mr >>= 1
		expR++
	}
	if expR <= 0 {
		return F16Zero
	}
	if expR >= 128 {
		return signR | F16MaxPos
	}
	return signR | (uint16(expR) << 8) | (mr & 0x00FF)
}

// F16Div führt eine CLACK-F16 Division (a / b) im Referenzmodell aus,
// exakt passend zur Relais-Laufzeitroutine __f16_div.
func F16Div(a, b uint16) uint16 {
	signR := (a ^ b) & 0x8000
	if (a & 0x7FFF) == 0 {
		return F16Zero
	}
	if (b & 0x7FFF) == 0 {
		return signR | F16MaxPos
	}
	expA := int((a >> 8) & 0x7F)
	expB := int((b >> 8) & 0x7F)
	expR := expA - expB + F16Bias

	mantA := uint16((a & 0x00FF) | 0x0100)
	mantB := uint16((b & 0x00FF) | 0x0100)

	var rem uint16
	if mantA < mantB {
		rem = (mantA << 1) - mantB
		expR--
	} else {
		rem = mantA - mantB
	}

	quot := uint16(1)
	for i := 0; i < 8; i++ {
		quot <<= 1
		rem <<= 1
		if rem >= mantB {
			rem -= mantB
			quot |= 1
		}
	}

	if expR <= 0 {
		return F16Zero
	}
	if expR >= 128 {
		return signR | F16MaxPos
	}
	return signR | (uint16(expR) << 8) | (quot & 0x00FF)
}

// ============================================================================
// COMPILER-CODEGENERIERUNG FÜR CLACK-F16 (TypeFloat16)
// ============================================================================

// evalConstFloat16Expr wertet konstante Literale (sowie negierte Literale) als CLACK-F16 Wort aus.
func evalConstFloat16Expr(expr ast.Expr) (uint16, bool) {
	if expr == nil {
		return 0, false
	}
	switch e := expr.(type) {
	case *ast.NumberLiteral:
		if e.IsFixed {
			return Float64ToF16(e.FloatVal), true
		}
		if e.IsHex {
			return uint16(e.Value & 0xFFFF), true
		}
		return IntToF16(e.Value), true
	case *ast.UnaryExpr:
		if e.Op == "-" {
			if n, ok := e.Right.(*ast.NumberLiteral); ok && !n.IsFixed && !n.IsHex {
				return IntToF16(-n.Value), true
			}
			val, ok := evalConstFloat16Expr(e.Right)
			if !ok {
				return 0, false
			}
			if (val & 0x7FFF) == 0 {
				return F16Zero, true
			}
			return val ^ 0x8000, true
		}
		return 0, false
	default:
		return 0, false
	}
}

// compileExprAsFloat16 übersetzt einen Ausdruck in den Accu und stellt sicher,
// dass das Ergebnis im CLACK-F16 Format vorliegt.
func (g *CodeGenerator) compileExprAsFloat16(expr ast.Expr) error {
	if val, ok := evalConstFloat16Expr(expr); ok {
		g.emit(OpLDA, val, fmt.Sprintf("CLACK-F16 Konstante 0x%04X (%.4g)", val, F16ToFloat64(val)))
		return nil
	}
	t := g.exprType(expr)
	if err := g.compileExpr(expr); err != nil {
		return err
	}
	if t == ast.TypeInt16 {
		g.emitIntToF16()
	} else if t == ast.TypeFixed8 {
		// Q8.8 ist int16(W) * 2^-8 -> ITOF16 und Exponent - 8
		g.emitIntToF16()
		skipZero := fmt.Sprintf("__q88_to_f16_zero_%d", g.pc)
		g.emit(OpBZ, 0, "Q8.8->F16: 0 bleibt 0")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: skipZero})
		g.emit(OpADD, 0xF800, "Q8.8->F16: Exponent - 8 (/256)")
		g.labels[skipZero] = g.pc
	}
	return nil
}

// emitIntToF16 ruft die Laufzeitroutine __f16_itof auf (Accu Int16 -> Accu CLACK-F16).
func (g *CodeGenerator) emitIntToF16() {
	g.needItofF16 = true
	g.emit(OpCALL, 0, "CALL __f16_itof")
	g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_itof"})
}

// emitF16ToInt ruft die Laufzeitroutine __f16_ftoi auf (Accu CLACK-F16 -> Accu Int16).
func (g *CodeGenerator) emitF16ToInt() {
	g.needFtoiF16 = true
	g.emit(OpCALL, 0, "CALL __f16_ftoi")
	g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_ftoi"})
}

// emitF16Neg negiert den CLACK-F16 Wert im Accu durch Invertieren von Bit 15 (wenn != 0).
func (g *CodeGenerator) emitF16Neg() {
	doneLabel := fmt.Sprintf("__f16_neg_done_%d", g.pc)
	g.emit(OpBZ, 0, "F16 Negation: 0.0 bleibt 0x0000")
	g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: doneLabel})
	g.emit(OpADD, 0x8000, "F16 Negation: Toggle Sign-Bit (XOR 0x8000)")
	g.labels[doneLabel] = g.pc
}

// compileArgForFloat16Op übersetzt ein Argument für CLACK-F16 Operationen.
// Bei expliziten Funktionsaufrufen wie F16ADD(A, B) wird eine ungetypte Variable A direkt
// als rohes 16-Bit CLACK-F16 Wort geladen; wurde A explizit als INTEGER deklariert, erfolgt ITOF16.
func (g *CodeGenerator) compileArgForFloat16Op(expr ast.Expr, explicitFunc bool) error {
	if !explicitFunc {
		return g.compileExprAsFloat16(expr)
	}
	if v, ok := expr.(*ast.VariableExpr); ok {
		upper := strings.ToUpper(v.Name)
		if g.explicitVarTypes[upper] && g.varTypes[upper] == ast.TypeInt16 {
			if err := g.compileExpr(expr); err != nil {
				return err
			}
			g.emitIntToF16()
			return nil
		}
		if g.explicitVarTypes[upper] && g.varTypes[upper] == ast.TypeFixed8 {
			return g.compileExprAsFloat16(expr)
		}
		return g.compileExpr(expr)
	}
	return g.compileExprAsFloat16(expr)
}

func (g *CodeGenerator) isSimpleFloat16Operand(expr ast.Expr, explicitFunc bool) bool {
	if _, ok := evalConstFloat16Expr(expr); ok {
		return true
	}
	if v, ok := expr.(*ast.VariableExpr); ok {
		upper := strings.ToUpper(v.Name)
		if explicitFunc {
			return !g.explicitVarTypes[upper] || g.varTypes[upper] == ast.TypeFloat16
		}
		return g.getVarType(v.Name) == ast.TypeFloat16
	}
	return false
}

// emitF16BinaryCall lädt left nach __r0 und right nach __r1 (kollisionssicher bei verschachtelten Ausdrücken)
// und ruft die angegebene CLACK-F16 Laufzeitroutine auf.
func (g *CodeGenerator) emitF16BinaryCall(left, right ast.Expr, explicitFunc bool, routineLabel string) error {
	r0Addr := g.getOrCreateVar("__r0")
	r1Addr := g.getOrCreateVar("__r1")

	if g.isSimpleFloat16Operand(right, explicitFunc) {
		if err := g.compileArgForFloat16Op(left, explicitFunc); err != nil {
			return err
		}
		g.emit(OpSTAM, r0Addr, fmt.Sprintf("%s Operand A -> __r0", routineLabel))

		if vL, okL := left.(*ast.VariableExpr); okL {
			if vR, okR := right.(*ast.VariableExpr); okR && strings.EqualFold(vL.Name, vR.Name) {
				g.emit(OpSTAM, r1Addr, fmt.Sprintf("%s Operand B -> __r1 (identisch)", routineLabel))
				g.emit(OpCALL, 0, fmt.Sprintf("CALL %s", routineLabel))
				g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: routineLabel})
				return nil
			}
		}

		if err := g.compileArgForFloat16Op(right, explicitFunc); err != nil {
			return err
		}
		g.emit(OpSTAM, r1Addr, fmt.Sprintf("%s Operand B -> __r1", routineLabel))
	} else {
		if err := g.compileArgForFloat16Op(left, explicitFunc); err != nil {
			return err
		}
		tempLeft := g.allocTemp()
		g.emit(OpSTAM, tempLeft, fmt.Sprintf("%s Temp A sichern", routineLabel))

		if err := g.compileArgForFloat16Op(right, explicitFunc); err != nil {
			return err
		}
		g.emit(OpSTAM, r1Addr, fmt.Sprintf("%s Operand B -> __r1", routineLabel))

		g.emit(OpLDAM, tempLeft, fmt.Sprintf("%s Lade Temp A", routineLabel))
		g.emit(OpSTAM, r0Addr, fmt.Sprintf("%s Operand A -> __r0", routineLabel))
	}

	g.emit(OpCALL, 0, fmt.Sprintf("CALL %s", routineLabel))
	g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: routineLabel})
	return nil
}

// compileBinaryFloat16 übersetzt +, -, *, / sowie Vergleiche auf CLACK-F16 Gleitkommawerten.
func (g *CodeGenerator) compileBinaryFloat16(e *ast.BinaryExpr) error {
	return g.compileBinaryFloat16WithArgMode(e.Left, e.Op, e.Right, false)
}

func (g *CodeGenerator) compileBinaryFloat16WithArgMode(left ast.Expr, rawOp string, right ast.Expr, explicitFunc bool) error {
	op := strings.ToUpper(rawOp)

	rConstVal, rIsConst := evalConstFloat16Expr(right)
	lConstVal, lIsConst := evalConstFloat16Expr(left)

	switch op {
	case "+":
		// Optimierung nur wenn genau einer der Operanden eine Variable und der andere 0.0 ist
		if rIsConst && (rConstVal&0x7FFF) == 0 && !lIsConst {
			return g.compileArgForFloat16Op(left, explicitFunc)
		}
		if lIsConst && (lConstVal&0x7FFF) == 0 && !rIsConst {
			return g.compileArgForFloat16Op(right, explicitFunc)
		}
		g.needFAddF16 = true
		return g.emitF16BinaryCall(left, right, explicitFunc, "__f16_add")

	case "-":
		if rIsConst && (rConstVal&0x7FFF) == 0 && !lIsConst {
			return g.compileArgForFloat16Op(left, explicitFunc)
		}
		g.needFSubF16 = true
		g.needFAddF16 = true
		return g.emitF16BinaryCall(left, right, explicitFunc, "__f16_sub")

	case "*":
		if (rIsConst && (rConstVal&0x7FFF) == 0 && !lIsConst) || (lIsConst && (lConstVal&0x7FFF) == 0 && !rIsConst) {
			g.emit(OpLDA, F16Zero, "CLACK-F16 Accu = 0.0 (* 0.0)")
			return nil
		}
		if rIsConst && rConstVal == F16One && !lIsConst {
			return g.compileArgForFloat16Op(left, explicitFunc)
		}
		if lIsConst && lConstVal == F16One && !rIsConst {
			return g.compileArgForFloat16Op(right, explicitFunc)
		}
		g.needFMulF16 = true
		return g.emitF16BinaryCall(left, right, explicitFunc, "__f16_mul")

	case "/":
		if rIsConst && rConstVal == F16One && !lIsConst {
			return g.compileArgForFloat16Op(left, explicitFunc)
		}
		g.needFDivF16 = true
		return g.emitF16BinaryCall(left, right, explicitFunc, "__f16_div")

	case "=", "==", "<>", "!=", "<", ">=", ">", "<=":
		var branchOp uint16
		invert := false
		reverse := false
		switch op {
		case "=", "==":
			branchOp = OpBZ
		case "<>", "!=":
			branchOp = OpBZ
			invert = true
		case "<":
			branchOp = OpBN
		case ">=":
			branchOp = OpBN
			invert = true
		case ">":
			branchOp = OpBN
			reverse = true
		case "<=":
			branchOp = OpBN
			invert = true
			reverse = true
		}

		g.needFSubF16 = true
		g.needFAddF16 = true
		if !reverse {
			if err := g.emitF16BinaryCall(left, right, explicitFunc, "__f16_sub"); err != nil {
				return err
			}
		} else {
			if err := g.emitF16BinaryCall(right, left, explicitFunc, "__f16_sub"); err != nil {
				return err
			}
		}

		trueLabel := fmt.Sprintf("__f16_comp_true_%d", g.pc)
		endLabel := fmt.Sprintf("__f16_comp_end_%d", g.pc)
		if !invert {
			g.emit(branchOp, 0, "F16 Vergleich erfuellt")
			g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: trueLabel})
			g.emit(OpLDA, 0, "Ergebnis = 0 (False)")
			g.emit(OpBA, 0, "Sprung zu Comp-Ende")
			g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: endLabel})
			g.labels[trueLabel] = g.pc
			g.emit(OpLDA, 1, "Ergebnis = 1 (True)")
			g.labels[endLabel] = g.pc
		} else {
			g.emit(branchOp, 0, "F16 Vergleich invertiert")
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

	return nil
}

// ============================================================================
// MASCHINEN-LAUFZEITROUTINEN FÜR CLACK-F16 (__f16_add, __f16_sub, __f16_mul, __f16_div)
// ============================================================================

func (g *CodeGenerator) emitFloatF16Routines() {
	if g.needFSubF16 {
		g.needFAddF16 = true
	}
	needCommonPack := g.needFAddF16 || g.needFMulF16 || g.needFDivF16 || g.needItofF16

	// 1. CLACK-F16 Subtraktion: __f16_sub (__r0 - __r1 -> Accu)
	// Invertiert das Vorzeichenbit von __r1 (falls __r1 != 0) per ADD 0x8000 und springt
	// direkt per BA in __f16_add (kein verschachteltes CALL -> 1-Level Return-Register sicher!).
	if g.needFSubF16 {
		g.labels["__f16_sub"] = g.pc
		g.labels["__runtime_f16_sub"] = g.pc
		r0Addr := g.getOrCreateVar("__r0")
		r1Addr := g.getOrCreateVar("__r1")

		g.emit(OpLDAM, r1Addr, "__f16_sub: Lade Subtrahend B")
		g.emit(OpAND, 0x7FFF, "Pruefe |B| == 0")
		g.emit(OpBZ, 0, "Wenn B == 0 -> Return A")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_sub_ret_a"})

		g.emit(OpLDAM, r1Addr, "Lade B fuer Vorzeichenwechsel")
		g.emit(OpADD, 0x8000, "B = -B (Toggle Bit 15)")
		g.emit(OpSTAM, r1Addr, "Speichere -B in __r1")
		g.emit(OpBA, 0, "Weiter zu __f16_add")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_add"})

		g.labels["__f16_sub_ret_a"] = g.pc
		g.emit(OpLDAM, r0Addr, "__f16_sub: Lade A")
		g.emitSingle(OpRET, "RETURN __f16_sub (B == 0)")
	}

	// 2. CLACK-F16 Addition: __f16_add (__r0 + __r1 -> Accu)
	if g.needFAddF16 {
		g.labels["__f16_add"] = g.pc
		g.labels["__runtime_f16_add"] = g.pc
		r0Addr := g.getOrCreateVar("__r0")
		r1Addr := g.getOrCreateVar("__r1")
		signAAddr := g.getOrCreateVar("__fp_sign")
		signBAddr := g.getOrCreateVar("__prod")
		expAAddr := g.getOrCreateVar("__quot")
		expBAddr := g.getOrCreateVar("__rem")
		mantAAddr := g.getOrCreateVar("__f16_ma")
		mantBAddr := g.getOrCreateVar("__f16_mb")

		// Prüfe ob A == 0
		g.emit(OpLDAM, r0Addr, "__f16_add: Lade Operand A")
		g.emit(OpAND, 0x7FFF, "|A| = A & 0x7FFF")
		g.emit(OpBZ, 0, "Wenn |A| == 0 -> Return B")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_add_ret_b"})
		g.emit(OpSTAM, mantAAddr, "Speichere |A|")

		// Prüfe ob B == 0
		g.emit(OpLDAM, r1Addr, "__f16_add: Lade Operand B")
		g.emit(OpAND, 0x7FFF, "|B| = B & 0x7FFF")
		g.emit(OpBZ, 0, "Wenn |B| == 0 -> Return A")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_add_ret_a"})
		g.emit(OpSTAM, mantBAddr, "Speichere |B|")

		// Vergleiche |A| und |B|: wenn |A| < |B|, tausche __r0 und __r1
		g.emit(OpNOT, 0, "~|B|")
		g.emit(OpADD, 1, "-|B|")
		patchMag := g.pc + 5
		g.emit(OpSTAM, patchMag, "Patch -|B|")
		g.emit(OpLDAM, mantAAddr, "Lade |A|")
		g.emit(OpADD, 0, "|A| - |B|")
		g.emit(OpBN, 0, "Wenn |A| < |B| -> Swap A und B")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_add_swap"})
		g.emit(OpBA, 0, "Weiter zum Entpacken")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_add_unpack"})

		g.labels["__f16_add_swap"] = g.pc
		g.emit(OpLDAM, r0Addr, "Swap: Lade A")
		g.emit(OpSTAM, mantAAddr, "Temp = A")
		g.emit(OpLDAM, r1Addr, "Lade B")
		g.emit(OpSTAM, r0Addr, "__r0 = B")
		g.emit(OpLDAM, mantAAddr, "Lade Temp")
		g.emit(OpSTAM, r1Addr, "__r1 = A")

		// Entpacken von Sign, Exponent und Mantisse (mit impliziter 1 bei 0x0100)
		g.labels["__f16_add_unpack"] = g.pc
		g.emit(OpLDAM, r0Addr, "Lade A")
		g.emit(OpAND, 0x8000, "signA = A & 0x8000")
		g.emit(OpSTAM, signAAddr, "Speichere signA")

		g.emit(OpLDAM, r1Addr, "Lade B")
		g.emit(OpAND, 0x8000, "signB = B & 0x8000")
		g.emit(OpSTAM, signBAddr, "Speichere signB")

		g.emit(OpLDAM, r0Addr, "Lade A fuer EA")
		g.emit(OpAND, 0x7F00, "A & 0x7F00")
		for s := 0; s < 8; s++ {
			g.emit(OpSHR, 0, "EA >> 8")
		}
		g.emit(OpSTAM, expAAddr, "Speichere EA")

		g.emit(OpLDAM, r1Addr, "Lade B fuer EB")
		g.emit(OpAND, 0x7F00, "B & 0x7F00")
		for s := 0; s < 8; s++ {
			g.emit(OpSHR, 0, "EB >> 8")
		}
		g.emit(OpSTAM, expBAddr, "Speichere EB")

		g.emit(OpLDAM, r0Addr, "Lade A fuer MA")
		g.emit(OpAND, 0x00FF, "A & 0xFF")
		g.emit(OpOR, 0x0100, "MA = (A & 0xFF) | 0x0100")
		g.emit(OpSTAM, mantAAddr, "Speichere MA")

		g.emit(OpLDAM, r1Addr, "Lade B fuer MB")
		g.emit(OpAND, 0x00FF, "B & 0xFF")
		g.emit(OpOR, 0x0100, "MB = (B & 0xFF) | 0x0100")
		g.emit(OpSTAM, mantBAddr, "Speichere MB")

		// Berechne dE = EA - EB (>= 0 da |A| >= |B|)
		g.emit(OpLDAM, expBAddr, "Lade EB")
		g.emit(OpNOT, 0, "~EB")
		g.emit(OpADD, 1, "-EB")
		patchDE := g.pc + 5
		g.emit(OpSTAM, patchDE, "Patch -EB")
		g.emit(OpLDAM, expAAddr, "Lade EA")
		g.emit(OpADD, 0, "dE = EA - EB")
		g.emit(OpBZ, 0, "Wenn dE == 0 -> Keine Ausrichtung noetig")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_add_aligned"})
		g.emit(OpSTAM, expBAddr, "Speichere dE")

		// Wenn dE >= 9 -> B fällt aus der 9-Bit Mantisse heraus -> Return A
		g.emit(OpADD, 0xFFF7, "dE - 9")
		g.emit(OpBN, 0, "Wenn dE < 9 -> Mantissen-Shift-Schleife")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_add_shift_loop"})
		g.emit(OpBA, 0, "dE >= 9 -> Return A")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_add_ret_a"})

		shiftLoopPC := g.pc
		g.labels["__f16_add_shift_loop"] = shiftLoopPC
		g.emit(OpLDAM, mantBAddr, "Lade MB")
		g.emit(OpSHR, 0, "MB >> 1")
		g.emit(OpSTAM, mantBAddr, "Speichere MB")
		g.emit(OpLDAM, expBAddr, "Lade dE")
		g.emit(OpADD, 0xFFFF, "dE - 1")
		g.emit(OpSTAM, expBAddr, "Speichere dE")
		g.emit(OpBZ, 0, "Wenn dE == 0 -> Ausrichtung fertig")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_add_aligned"})
		g.emit(OpBA, shiftLoopPC, "Loop Mantissen-Shift")

		// Prüfe ob signA == signB: (signA + signB) == 0 genau dann, wenn signA == signB
		g.labels["__f16_add_aligned"] = g.pc
		g.emit(OpLDAM, signBAddr, "Lade signB")
		patchSign := g.pc + 5
		g.emit(OpSTAM, patchSign, "Patch signB")
		g.emit(OpLDAM, signAAddr, "Lade signA")
		g.emit(OpADD, 0, "signA + signB (XOR auf Bit 15)")
		g.emit(OpBZ, 0, "Wenn gleiche Vorzeichen -> Addiere Mantissen")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_add_same_sign"})
		g.emit(OpBA, 0, "Unterschiedliche Vorzeichen -> Subtrahiere Mantissen")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_add_diff_sign"})

		// Gleiche Vorzeichen: MR = MA + MB -> Gemeinsamer Normierungs- und Pack-Block
		g.labels["__f16_add_same_sign"] = g.pc
		g.emit(OpLDAM, mantBAddr, "Lade MB")
		patchAddM := g.pc + 5
		g.emit(OpSTAM, patchAddM, "Patch +MB")
		g.emit(OpLDAM, mantAAddr, "Lade MA")
		g.emit(OpADD, 0, "MR = MA + MB")
		g.emit(OpSTAM, mantAAddr, "Speichere MR")
		g.emit(OpBA, 0, "Sprung zu gemeinsamem Norm+Pack")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_common_norm_pack"})

		// Unterschiedliche Vorzeichen: MR = MA - MB (>= 0 garantiert)
		g.labels["__f16_add_diff_sign"] = g.pc
		g.emit(OpLDAM, mantBAddr, "Lade MB")
		g.emit(OpNOT, 0, "~MB")
		g.emit(OpADD, 1, "-MB")
		patchSubM := g.pc + 5
		g.emit(OpSTAM, patchSubM, "Patch -MB")
		g.emit(OpLDAM, mantAAddr, "Lade MA")
		g.emit(OpADD, 0, "MR = MA - MB")
		g.emit(OpBZ, 0, "Wenn MR == 0 -> Return 0.0")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_common_ret_zero"})
		g.emit(OpSTAM, mantAAddr, "Speichere MR")

		normLoopPC := g.pc
		g.labels["__f16_add_norm_loop"] = normLoopPC
		g.emit(OpLDAM, mantAAddr, "Lade MR fuer Normierung")
		g.emit(OpAND, 0x0100, "Pruefe implizite 1 (Bit 8)")
		g.emit(OpBZ, 0, "Wenn Bit 8 == 0 -> Links-Shift")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_add_norm_shift"})
		g.emit(OpBA, 0, "Bit 8 == 1 -> Packen")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_common_pack"})

		g.labels["__f16_add_norm_shift"] = g.pc
		g.emit(OpLDAM, expAAddr, "Lade EA")
		g.emit(OpADD, 0xFFFF, "EA - 1")
		g.emit(OpBZ, 0, "Wenn EA == 0 -> Underflow 0.0")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_common_ret_zero"})
		g.emit(OpSTAM, expAAddr, "Speichere EA")
		g.emit(OpLDAM, mantAAddr, "Lade MR")
		g.emit(OpSHL, 0, "MR << 1")
		g.emit(OpSTAM, mantAAddr, "Speichere MR")
		g.emit(OpBA, normLoopPC, "Loop Normierung")

		g.labels["__f16_add_ret_a"] = g.pc
		g.emit(OpLDAM, r0Addr, "Return A")
		g.emitSingle(OpRET, "RETURN __f16_add (A)")

		g.labels["__f16_add_ret_b"] = g.pc
		g.emit(OpLDAM, r1Addr, "Return B")
		g.emitSingle(OpRET, "RETURN __f16_add (B)")
	}

	// 3. CLACK-F16 Multiplikation: __f16_mul (__r0 * __r1 -> Accu)
	// Nutzt die exakte Zerlegung ((256 + fA) * (256 + fB)) >> 8 = 256 + fA + fB + ((fA * fB) >> 8),
	// wodurch fA * fB (max 255*255 = 65025) komplett überlauffrei in ein 16-Bit Register passt!
	if g.needFMulF16 {
		g.labels["__f16_mul"] = g.pc
		g.labels["__runtime_f16_mul"] = g.pc
		r0Addr := g.getOrCreateVar("__r0")
		r1Addr := g.getOrCreateVar("__r1")
		signRAddr := g.getOrCreateVar("__fp_sign")
		prodAddr := g.getOrCreateVar("__prod")
		expRAddr := g.getOrCreateVar("__quot")
		mantAAddr := g.getOrCreateVar("__f16_ma")
		mantBAddr := g.getOrCreateVar("__f16_mb")

		// Prüfe A == 0 und B == 0
		g.emit(OpLDAM, r0Addr, "__f16_mul: Lade A")
		g.emit(OpAND, 0x7FFF, "|A|")
		g.emit(OpBZ, 0, "Wenn |A| == 0 -> Return 0.0")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_common_ret_zero"})

		g.emit(OpLDAM, r1Addr, "__f16_mul: Lade B")
		g.emit(OpAND, 0x7FFF, "|B|")
		g.emit(OpBZ, 0, "Wenn |B| == 0 -> Return 0.0")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_common_ret_zero"})

		// Berechne signR = (A ^ B) & 0x8000
		g.emit(OpLDAM, r0Addr, "Lade A fuer Sign")
		g.emit(OpAND, 0x8000, "A & 0x8000")
		g.emit(OpSTAM, signRAddr, "signR = signA")
		g.emit(OpLDAM, r1Addr, "Lade B fuer Sign")
		g.emit(OpBN, 0, "Wenn B < 0 -> Toggle signR")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_mul_b_neg"})
		g.emit(OpBA, 0, "Weiter zu Exponenten")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_mul_exp"})

		g.labels["__f16_mul_b_neg"] = g.pc
		g.emit(OpLDAM, signRAddr, "Lade signR")
		g.emit(OpADD, 0x8000, "signR ^ 0x8000")
		g.emit(OpSTAM, signRAddr, "Speichere signR")

		// Berechne ER = EA + EB - 63
		g.labels["__f16_mul_exp"] = g.pc
		g.emit(OpLDAM, r0Addr, "Lade A fuer EA")
		g.emit(OpAND, 0x7F00, "A & 0x7F00")
		for s := 0; s < 8; s++ {
			g.emit(OpSHR, 0, "EA >> 8")
		}
		g.emit(OpSTAM, expRAddr, "Speichere EA")

		g.emit(OpLDAM, r1Addr, "Lade B fuer EB")
		g.emit(OpAND, 0x7F00, "B & 0x7F00")
		for s := 0; s < 8; s++ {
			g.emit(OpSHR, 0, "EB >> 8")
		}
		patchExp := g.pc + 5
		g.emit(OpSTAM, patchExp, "Patch +EB")
		g.emit(OpLDAM, expRAddr, "Lade EA")
		g.emit(OpADD, 0, "EA + EB")
		g.emit(OpADD, 0xFFC1, "EA + EB - 63 (Bias)")
		g.emit(OpSTAM, expRAddr, "Speichere ER")

		// Extrahiere Bruch-Mantissen fA = A & 0xFF und fB = B & 0xFF
		g.emit(OpLDAM, r0Addr, "Lade A fuer fA")
		g.emit(OpAND, 0x00FF, "fA = A & 0xFF")
		g.emit(OpSTAM, mantAAddr, "Speichere fA")
		g.emit(OpSTAM, r0Addr, "fA_shift = fA")

		g.emit(OpLDAM, r1Addr, "Lade B fuer fB")
		g.emit(OpAND, 0x00FF, "fB = B & 0xFF")
		g.emit(OpSTAM, mantBAddr, "Speichere fB")
		g.emit(OpSTAM, r1Addr, "fB_shift = fB")

		// 8x8-Bit Shift-and-Add Multiplikation: prod = fA * fB
		g.emit(OpLDA, 0, "prod = 0")
		g.emit(OpSTAM, prodAddr, "Init prod = 0")
		g.emit(OpLDAM, r1Addr, "Lade fB_shift")
		g.emit(OpBZ, 0, "Wenn fB == 0 -> Skip Loop")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_mul_combine"})

		mulLoopPC := g.pc
		g.labels["__f16_mul_loop"] = mulLoopPC
		g.emit(OpBE, 0, "Wenn LSB(fB) == 0 -> Skip Add")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_mul_skip_add"})

		g.emit(OpLDAM, r0Addr, "Lade fA_shift")
		patchMulAdd := g.pc + 5
		g.emit(OpSTAM, patchMulAdd, "Patch +fA_shift")
		g.emit(OpLDAM, prodAddr, "Lade prod")
		g.emit(OpADD, 0, "prod + fA_shift")
		g.emit(OpSTAM, prodAddr, "Speichere prod")

		g.labels["__f16_mul_skip_add"] = g.pc
		g.emit(OpLDAM, r0Addr, "Lade fA_shift")
		g.emit(OpSHL, 0, "fA_shift << 1")
		g.emit(OpSTAM, r0Addr, "Speichere fA_shift")

		g.emit(OpLDAM, r1Addr, "Lade fB_shift")
		g.emit(OpSHR, 0, "fB_shift >> 1")
		g.emit(OpBZ, 0, "Wenn fB_shift == 0 -> Kombiniere")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_mul_combine"})
		g.emit(OpSTAM, r1Addr, "Speichere fB_shift")
		g.emit(OpBA, mulLoopPC, "Loop __f16_mul")

		// Kombiniere MR = 256 + fA + fB + (prod >> 8)
		g.labels["__f16_mul_combine"] = g.pc
		g.emit(OpLDAM, prodAddr, "Lade prod")
		for s := 0; s < 8; s++ {
			g.emit(OpSHR, 0, "prod >> 8")
		}
		g.emit(OpADD, 0x0100, "256 + (prod >> 8)")
		g.emit(OpSTAM, prodAddr, "Speichere Zwischenwert")

		g.emit(OpLDAM, mantAAddr, "Lade fA")
		patchFA := g.pc + 5
		g.emit(OpSTAM, patchFA, "Patch +fA")
		g.emit(OpLDAM, prodAddr, "Lade Zwischenwert")
		g.emit(OpADD, 0, "+ fA")
		g.emit(OpSTAM, prodAddr, "Speichere Zwischenwert")

		g.emit(OpLDAM, mantBAddr, "Lade fB")
		patchFB := g.pc + 5
		g.emit(OpSTAM, patchFB, "Patch +fB")
		g.emit(OpLDAM, prodAddr, "Lade Zwischenwert")
		g.emit(OpADD, 0, "MR = 256 + fA + fB + (prod >> 8)")
		g.emit(OpSTAM, mantAAddr, "Speichere MR")
		g.emit(OpBA, 0, "Sprung zu gemeinsamem Norm+Pack")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_common_norm_pack"})
	}

	// 4. CLACK-F16 Division: __f16_div (__r0 / __r1 -> Accu)
	// Normiert vorab MA >= MB (sodass das führende Bit 1 bereits feststeht) und
	// berechnet die 8 Nachkommabits in exakt 8 Restoring-Division-Schritten.
	if g.needFDivF16 {
		g.labels["__f16_div"] = g.pc
		g.labels["__runtime_f16_div"] = g.pc
		r0Addr := g.getOrCreateVar("__r0")
		r1Addr := g.getOrCreateVar("__r1")
		signRAddr := g.getOrCreateVar("__fp_sign")
		cntAddr := g.getOrCreateVar("__prod")
		expRAddr := g.getOrCreateVar("__quot")
		remAddr := g.getOrCreateVar("__rem")
		mantAAddr := g.getOrCreateVar("__f16_ma")
		mantBAddr := g.getOrCreateVar("__f16_mb")

		// Berechne signR = (A ^ B) & 0x8000
		g.emit(OpLDAM, r0Addr, "__f16_div: Lade A fuer Sign")
		g.emit(OpAND, 0x8000, "A & 0x8000")
		g.emit(OpSTAM, signRAddr, "signR = signA")
		g.emit(OpLDAM, r1Addr, "Lade B fuer Sign")
		g.emit(OpBN, 0, "Wenn B < 0 -> Toggle signR")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_div_b_neg"})
		g.emit(OpBA, 0, "Weiter zu Zero-Check")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_div_check_zero"})

		g.labels["__f16_div_b_neg"] = g.pc
		g.emit(OpLDAM, signRAddr, "Lade signR")
		g.emit(OpADD, 0x8000, "signR ^ 0x8000")
		g.emit(OpSTAM, signRAddr, "Speichere signR")

		g.labels["__f16_div_check_zero"] = g.pc
		g.emit(OpLDAM, r0Addr, "Lade A")
		g.emit(OpAND, 0x7FFF, "|A|")
		g.emit(OpBZ, 0, "Wenn |A| == 0 -> Return 0.0")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_common_ret_zero"})

		g.emit(OpLDAM, r1Addr, "Lade B")
		g.emit(OpAND, 0x7FFF, "|B|")
		g.emit(OpBZ, 0, "Wenn |B| == 0 -> Division durch 0 (Saettigung)")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_common_ovf"})

		// Berechne ER = EA - EB + 63
		g.emit(OpLDAM, r0Addr, "Lade A fuer EA")
		g.emit(OpAND, 0x7F00, "A & 0x7F00")
		for s := 0; s < 8; s++ {
			g.emit(OpSHR, 0, "EA >> 8")
		}
		g.emit(OpSTAM, expRAddr, "Speichere EA")

		g.emit(OpLDAM, r1Addr, "Lade B fuer EB")
		g.emit(OpAND, 0x7F00, "B & 0x7F00")
		for s := 0; s < 8; s++ {
			g.emit(OpSHR, 0, "EB >> 8")
		}
		g.emit(OpNOT, 0, "~EB")
		g.emit(OpADD, 1, "-EB")
		patchEB := g.pc + 5
		g.emit(OpSTAM, patchEB, "Patch -EB")
		g.emit(OpLDAM, expRAddr, "Lade EA")
		g.emit(OpADD, 0, "EA - EB")
		g.emit(OpADD, F16Bias, "EA - EB + 63")
		g.emit(OpSTAM, expRAddr, "Speichere ER")

		// Entpacke MA und MB
		g.emit(OpLDAM, r0Addr, "Lade A fuer MA")
		g.emit(OpAND, 0x00FF, "A & 0xFF")
		g.emit(OpOR, 0x0100, "MA = (A & 0xFF) | 0x0100")
		g.emit(OpSTAM, mantAAddr, "Speichere MA")

		g.emit(OpLDAM, r1Addr, "Lade B fuer MB")
		g.emit(OpAND, 0x00FF, "B & 0xFF")
		g.emit(OpOR, 0x0100, "MB = (B & 0xFF) | 0x0100")
		g.emit(OpSTAM, mantBAddr, "Speichere MB")

		// Berechne -MB und patche alle 3 Subtraktionsstellen
		g.emit(OpNOT, 0, "~MB")
		g.emit(OpADD, 1, "-MB")
		g.emit(OpSTAM, 0, "Patch MA - MB")
		fixInitSub := g.pc - 1
		g.emit(OpSTAM, 0, "Patch 2*MA - MB")
		fixLtSub := g.pc - 1
		g.emit(OpSTAM, 0, "Patch Loop rem - MB")
		fixLoopSub := g.pc - 1

		// Prüfe ob MA < MB
		g.emit(OpLDAM, mantAAddr, "Lade MA")
		g.emit(OpADD, 0, "MA - MB")
		g.memory[fixInitSub] = g.pc - 1
		g.emit(OpBN, 0, "Wenn MA < MB -> Vorab-Shift MA << 1 und ER--")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_div_ma_lt_mb"})
		g.emit(OpSTAM, remAddr, "rem = MA - MB")
		g.emit(OpBA, 0, "Starte 8-Bit Divisionsschleife")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_div_loop_init"})

		g.labels["__f16_div_ma_lt_mb"] = g.pc
		g.emit(OpLDAM, mantAAddr, "Lade MA")
		g.emit(OpSHL, 0, "2 * MA")
		g.emit(OpADD, 0, "2 * MA - MB")
		g.memory[fixLtSub] = g.pc - 1
		g.emit(OpSTAM, remAddr, "rem = 2*MA - MB")
		g.emit(OpLDAM, expRAddr, "Lade ER")
		g.emit(OpADD, 0xFFFF, "ER - 1")
		g.emit(OpSTAM, expRAddr, "Speichere ER")

		g.labels["__f16_div_loop_init"] = g.pc
		g.emit(OpLDA, 1, "quot = 1 (implizite 1)")
		g.emit(OpSTAM, mantAAddr, "Speichere quot")
		g.emit(OpLDA, 8, "cnt = 8")
		g.emit(OpSTAM, cntAddr, "Speichere cnt")

		divLoopPC := g.pc
		g.labels["__f16_div_loop"] = divLoopPC
		g.emit(OpLDAM, mantAAddr, "Lade quot")
		g.emit(OpSHL, 0, "quot << 1")
		g.emit(OpSTAM, mantAAddr, "Speichere quot")

		g.emit(OpLDAM, remAddr, "Lade rem")
		g.emit(OpSHL, 0, "rem << 1")
		g.emit(OpSTAM, remAddr, "Speichere rem << 1")
		g.emit(OpADD, 0, "(rem << 1) - MB")
		g.memory[fixLoopSub] = g.pc - 1
		g.emit(OpBN, 0, "Wenn rem < MB -> Bit 0 bleibt 0")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_div_next"})

		g.emit(OpSTAM, remAddr, "rem = (rem << 1) - MB")
		g.emit(OpLDAM, mantAAddr, "Lade quot")
		g.emit(OpOR, 1, "quot |= 1")
		g.emit(OpSTAM, mantAAddr, "Speichere quot")

		g.labels["__f16_div_next"] = g.pc
		g.emit(OpLDAM, cntAddr, "Lade cnt")
		g.emit(OpADD, 0xFFFF, "cnt - 1")
		g.emit(OpSTAM, cntAddr, "Speichere cnt")
		g.emit(OpBZ, 0, "Wenn cnt == 0 -> Pruefe Exponent und Packen")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_common_check_exp"})
		g.emit(OpBA, divLoopPC, "Loop __f16_div")
	}

	// 5. Integer nach CLACK-F16 Konvertierung: __f16_itof (Accu Int16 -> Accu CLACK-F16)
	if g.needItofF16 {
		g.labels["__f16_itof"] = g.pc
		g.labels["__runtime_f16_itof"] = g.pc
		signAddr := g.getOrCreateVar("__fp_sign")
		expAddr := g.getOrCreateVar("__quot")
		valAddr := g.getOrCreateVar("__f16_ma")

		g.emit(OpBZ, 0, "__f16_itof: Wenn 0 -> Return 0.0")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_common_ret_zero"})
		g.emit(OpSTAM, valAddr, "Speichere Eingabe")
		g.emit(OpLDA, 0, "Init sign = 0")
		g.emit(OpSTAM, signAddr, "sign = 0")
		g.emit(OpLDAM, valAddr, "Lade Eingabe")
		g.emit(OpBN, 0, "Wenn negativ -> Negiere")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_itof_neg"})
		g.emit(OpBA, 0, "Weiter zu Normierung")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_itof_norm"})

		g.labels["__f16_itof_neg"] = g.pc
		g.emit(OpNOT, 0, "~val")
		g.emit(OpADD, 1, "|val| = -val")
		g.emit(OpSTAM, valAddr, "Speichere |val|")
		g.emit(OpLDA, 0x8000, "sign = 0x8000")
		g.emit(OpSTAM, signAddr, "Speichere sign")

		g.labels["__f16_itof_norm"] = g.pc
		g.emit(OpLDA, F16Bias+8, "Init exp = 71 (63 + 8)")
		g.emit(OpSTAM, expAddr, "Speichere exp")

		shrCheckPC := g.pc
		g.labels["__f16_itof_shr_check"] = shrCheckPC
		g.emit(OpLDAM, valAddr, "Lade |val|")
		g.emit(OpAND, 0xFE00, "Pruefe |val| >= 512")
		g.emit(OpBZ, 0, "Wenn < 512 -> Pruefe Links-Shift")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_itof_shl_check"})
		g.emit(OpLDAM, valAddr, "Lade |val|")
		g.emit(OpSHR, 0, "|val| >> 1")
		g.emit(OpSTAM, valAddr, "Speichere |val|")
		g.emit(OpLDAM, expAddr, "Lade exp")
		g.emit(OpADD, 1, "exp + 1")
		g.emit(OpSTAM, expAddr, "Speichere exp")
		g.emit(OpBA, shrCheckPC, "Loop Rechts-Shift")

		shlCheckPC := g.pc
		g.labels["__f16_itof_shl_check"] = shlCheckPC
		g.emit(OpLDAM, valAddr, "Lade |val|")
		g.emit(OpAND, 0x0100, "Pruefe Bit 8 (0x0100)")
		g.emit(OpBZ, 0, "Wenn Bit 8 == 0 -> Links-Shift")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_itof_shl_step"})
		g.emit(OpBA, 0, "Bit 8 == 1 -> Packen")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_common_pack"})

		g.labels["__f16_itof_shl_step"] = g.pc
		g.emit(OpLDAM, valAddr, "Lade |val|")
		g.emit(OpSHL, 0, "|val| << 1")
		g.emit(OpSTAM, valAddr, "Speichere |val|")
		g.emit(OpLDAM, expAddr, "Lade exp")
		g.emit(OpADD, 0xFFFF, "exp - 1")
		g.emit(OpSTAM, expAddr, "Speichere exp")
		g.emit(OpBA, shlCheckPC, "Loop Links-Shift")
	}

	// Gemeinsamer Normierungs-, Exponentenprüf- und Pack-Block für __f16_add, __f16_mul, __f16_div, __f16_itof
	if needCommonPack {
		signAddr := g.getOrCreateVar("__fp_sign")
		expAddr := g.getOrCreateVar("__quot")
		mantAddr := g.getOrCreateVar("__f16_ma")

		g.labels["__f16_common_norm_pack"] = g.pc
		g.emit(OpLDAM, mantAddr, "Lade MR fuer Bit-9 Check")
		g.emit(OpAND, 0x0200, "Pruefe Carry in Bit 9 (MR >= 512)")
		g.emit(OpBZ, 0, "Wenn < 512 -> Pruefe Exponent")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_common_check_exp"})
		g.emit(OpLDAM, mantAddr, "Lade MR")
		g.emit(OpSHR, 0, "MR >> 1")
		g.emit(OpSTAM, mantAddr, "Speichere normiertes MR")
		g.emit(OpLDAM, expAddr, "Lade ER")
		g.emit(OpADD, 1, "ER + 1")
		g.emit(OpSTAM, expAddr, "Speichere ER")

		g.labels["__f16_common_check_exp"] = g.pc
		g.emit(OpLDAM, expAddr, "Lade ER")
		g.emit(OpBZ, 0, "Wenn ER == 0 -> Underflow 0.0")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_common_ret_zero"})
		g.emit(OpBN, 0, "Wenn ER < 0 -> Underflow 0.0")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_common_ret_zero"})
		g.emit(OpAND, 0xFF80, "Pruefe ER >= 128")
		g.emit(OpBZ, 0, "Kein Overflow -> Packen")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_common_pack"})

		g.labels["__f16_common_ovf"] = g.pc
		g.emit(OpLDAM, signAddr, "Lade signR bei Overflow")
		g.emit(OpOR, F16MaxPos, "Saettigung signR | 0x7FFF")
		g.emitSingle(OpRET, "RETURN __f16 (ovf)")

		g.labels["__f16_common_pack"] = g.pc
		g.emit(OpLDAM, expAddr, "Lade ER")
		for s := 0; s < 8; s++ {
			g.emit(OpSHL, 0, "ER << 8")
		}
		g.emit(OpSTAM, expAddr, "Speichere ER << 8")
		g.emit(OpLDAM, mantAddr, "Lade MR")
		g.emit(OpAND, 0x00FF, "MR & 0xFF")
		patchPackM := g.pc + 5
		g.emit(OpSTAM, patchPackM, "Patch Mantisse")
		g.emit(OpLDAM, expAddr, "Lade ER << 8")
		g.emit(OpADD, 0, "(ER << 8) | (MR & 0xFF)")
		g.emit(OpSTAM, expAddr, "Speichere Betrag")
		g.emit(OpLDAM, signAddr, "Lade signR")
		patchPackS := g.pc + 5
		g.emit(OpSTAM, patchPackS, "Patch signR")
		g.emit(OpLDAM, expAddr, "Lade Betrag")
		g.emit(OpADD, 0, "signR | (ER << 8) | (MR & 0xFF)")
		g.emitSingle(OpRET, "RETURN __f16")

		g.labels["__f16_common_ret_zero"] = g.pc
		g.emit(OpLDA, F16Zero, "Return 0.0")
		g.emitSingle(OpRET, "RETURN __f16 (0.0)")
	}

	// 6. CLACK-F16 nach Integer Konvertierung: __f16_ftoi (Accu CLACK-F16 -> Accu Int16)
	if g.needFtoiF16 {
		g.labels["__f16_ftoi"] = g.pc
		g.labels["__runtime_f16_ftoi"] = g.pc
		signAddr := g.getOrCreateVar("__fp_sign")
		expAddr := g.getOrCreateVar("__quot")
		valAddr := g.getOrCreateVar("__f16_ma")

		g.emit(OpSTAM, valAddr, "__f16_ftoi: Speichere F16-Wort")
		g.emit(OpAND, 0x8000, "Extrahiere Sign")
		g.emit(OpSTAM, signAddr, "Speichere sign")

		g.emit(OpLDAM, valAddr, "Lade F16-Wort fuer E")
		g.emit(OpAND, 0x7F00, "Wort & 0x7F00")
		for s := 0; s < 8; s++ {
			g.emit(OpSHR, 0, "E >> 8")
		}
		g.emit(OpSTAM, expAddr, "Speichere E")

		// Wenn E < 63 -> |x| < 1.0 -> Return 0
		g.emit(OpADD, 0xFFC1, "E - 63")
		g.emit(OpBN, 0, "Wenn E < 63 -> Return 0")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_ftoi_ret_zero"})

		// Wenn E >= 78 (63 + 15) -> Overflow Sättigung
		g.emit(OpLDAM, expAddr, "Lade E")
		g.emit(OpADD, 0xFFB2, "E - 78")
		g.emit(OpBN, 0, "Wenn E < 78 -> Normale Konvertierung")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_ftoi_extract_m"})
		g.emit(OpLDAM, signAddr, "Overflow: Pruefe Sign")
		g.emit(OpBZ, 0, "Wenn positiv -> 32767")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_ftoi_ovf_pos"})
		g.emit(OpLDA, 0x8000, "Negativ -> -32768")
		g.emitSingle(OpRET, "RETURN __f16_ftoi (ovf neg)")

		g.labels["__f16_ftoi_ovf_pos"] = g.pc
		g.emit(OpLDA, 0x7FFF, "Positiv -> 32767")
		g.emitSingle(OpRET, "RETURN __f16_ftoi (ovf pos)")

		g.labels["__f16_ftoi_extract_m"] = g.pc
		g.emit(OpLDAM, valAddr, "Lade F16-Wort")
		g.emit(OpAND, 0x00FF, "M & 0xFF")
		g.emit(OpOR, 0x0100, "M = (M & 0xFF) | 0x0100")
		g.emit(OpSTAM, valAddr, "Speichere M")

		// Vergleiche E mit 71 (63 + 8):
		g.emit(OpLDAM, expAddr, "Lade E")
		g.emit(OpADD, 0xFFB9, "E - 71")
		g.emit(OpBZ, 0, "Wenn E == 71 -> Kein Shift noetig")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_ftoi_apply_sign"})
		g.emit(OpBN, 0, "Wenn E < 71 -> Rechts-Shift")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_ftoi_shr_init"})

		// E > 71: Links-Shift um (E - 71)
		g.emit(OpSTAM, expAddr, "cnt = E - 71")
		shlLoopPC := g.pc
		g.labels["__f16_ftoi_shl_loop"] = shlLoopPC
		g.emit(OpLDAM, valAddr, "Lade M")
		g.emit(OpSHL, 0, "M << 1")
		g.emit(OpSTAM, valAddr, "Speichere M")
		g.emit(OpLDAM, expAddr, "Lade cnt")
		g.emit(OpADD, 0xFFFF, "cnt - 1")
		g.emit(OpSTAM, expAddr, "Speichere cnt")
		g.emit(OpBZ, 0, "Wenn cnt == 0 -> Vorzeichen anwenden")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_ftoi_apply_sign"})
		g.emit(OpBA, shlLoopPC, "Loop Links-Shift")

		// E < 71: Rechts-Shift um (71 - E) = -(E - 71)
		g.labels["__f16_ftoi_shr_init"] = g.pc
		g.emit(OpNOT, 0, "~(E - 71)")
		g.emit(OpADD, 1, "cnt = 71 - E")
		g.emit(OpSTAM, expAddr, "Speichere cnt")
		shrLoopPC := g.pc
		g.labels["__f16_ftoi_shr_loop"] = shrLoopPC
		g.emit(OpLDAM, valAddr, "Lade M")
		g.emit(OpSHR, 0, "M >> 1")
		g.emit(OpSTAM, valAddr, "Speichere M")
		g.emit(OpLDAM, expAddr, "Lade cnt")
		g.emit(OpADD, 0xFFFF, "cnt - 1")
		g.emit(OpSTAM, expAddr, "Speichere cnt")
		g.emit(OpBZ, 0, "Wenn cnt == 0 -> Vorzeichen anwenden")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_ftoi_apply_sign"})
		g.emit(OpBA, shrLoopPC, "Loop Rechts-Shift")

		g.labels["__f16_ftoi_apply_sign"] = g.pc
		g.emit(OpLDAM, signAddr, "Pruefe Sign")
		g.emit(OpBZ, 0, "Wenn positiv -> Fertig")
		g.fixups = append(g.fixups, labelFixup{patchPC: g.pc - 1, label: "__f16_ftoi_done"})
		g.emit(OpLDAM, valAddr, "Lade Betrag")
		g.emit(OpNOT, 0, "~Betrag")
		g.emit(OpADD, 1, "-Betrag")
		g.emitSingle(OpRET, "RETURN __f16_ftoi (negativ)")

		g.labels["__f16_ftoi_done"] = g.pc
		g.emit(OpLDAM, valAddr, "Lade Betrag")
		g.emitSingle(OpRET, "RETURN __f16_ftoi (positiv)")

		g.labels["__f16_ftoi_ret_zero"] = g.pc
		g.emit(OpLDA, 0, "Return 0")
		g.emitSingle(OpRET, "RETURN __f16_ftoi (0)")
	}
}

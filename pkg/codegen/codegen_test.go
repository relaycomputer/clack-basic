package codegen

import (
	"fmt"
	"strings"
	"testing"

	"clack-basic/pkg/lexer"
	"clack-basic/pkg/parser"
)

func TestCodeGen_GraphicsStatements(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		contains []string
	}{
		{
			name: "PSET and PLOT",
			source: `10 CLS
20 PLOT 5, 5, 1
30 PSET 10, 10, 0
40 END`,
			contains: []string{"__runtime_cls", "__runtime_pset"},
		},
		{
			name: "PIXELMOVE Absolute and Relative",
			source: `10 PIXELMOVE 5, 5 TO 6, 6, 1
20 X = 10
30 Y = 10
40 PIXELMOVE X, Y BY 1, -1, 1
50 END`,
			contains: []string{"__runtime_pset"},
		},
		{
			name: "LINE Drawing",
			source: `10 CLS
20 LINE 0, 0 TO 31, 31, 1
30 LINE 10, 20, 5, 0, 1
40 END`,
			contains: []string{"__runtime_cls", "__runtime_line"},
		},
		{
			name: "Multiplication and Division",
			source: `10 A = 12 * 34
20 B = 100 / 5
30 C = B MOD 7
40 END`,
			contains: []string{"__runtime_mul", "__runtime_div"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.NewLexer(tt.source)
			tokens := l.Tokenize()
			p := parser.NewParser(tokens)
			prog, err := p.ParseProgram()
			if err != nil {
				t.Fatalf("Parser errors: %v", err)
			}

			gen := NewCodeGenerator()
			mem, comments, err := gen.Generate(prog)
			if err != nil {
				t.Fatalf("Generate error: %v", err)
			}

			if mem[0] != OpBA {
				t.Errorf("Expected initial jump at mem[0], got 0x%04X", mem[0])
			}

			var commentDump strings.Builder
			for i := 0; i < len(mem); i++ {
				if c, ok := comments[uint16(i)]; ok {
					commentDump.WriteString(c + "\n")
				}
			}
			commentStr := commentDump.String()

			for _, req := range tt.contains {
				found := false
				if _, ok := gen.labels[req]; ok {
					found = true
				}
				if !found && strings.Contains(commentStr, req) {
					found = true
				}
				if !found {
					t.Errorf("Expected runtime/label %s not found in generated output", req)
				}
			}
		})
	}
}

func TestCodeGen_SqrMath(t *testing.T) {
	testCases := []struct {
		name     string
		source   string
		expected uint16
	}{
		{"SQR of 0", "10 R = SQR(0)\n20 END", 0},
		{"SQR of 1", "10 R = SQR(1)\n20 END", 1},
		{"SQR of 4", "10 R = SQR(4)\n20 END", 2},
		{"SQR of 9", "10 R = SQR(9)\n20 END", 3},
		{"SQR of 64", "10 R = SQR(64)\n20 END", 8},
		{"SQR of 100", "10 R = SQR(100)\n20 END", 10},
		{"SQRT alias 25", "10 R = SQRT(25)\n20 END", 5},
		{"SQR(2, 0) int", "10 R = SQR(2, 0)\n20 END", 1},
		{"SQR(2, 1) fixed 1.4", "10 R = SQR(2, 1)\n20 END", 14},
		{"SQR(2, 2) fixed 1.41", "10 R = SQR(2, 2)\n20 END", 141},
		{"SQR(3, 2) fixed 1.73", "10 R = SQR(3, 2)\n20 END", 173},
		{"SQR(10, 2) fixed 3.16", "10 R = SQR(10, 2)\n20 END", 316},
		{"SQR(100, 1) fixed 10.0", "10 R = SQR(100, 1)\n20 END", 100},
		{"SQR(100, 2) fixed 10.00", "10 R = SQR(100, 2)\n20 END", 1000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			l := lexer.NewLexer(tc.source)
			tokens := l.Tokenize()
			p := parser.NewParser(tokens)
			prog, err := p.ParseProgram()
			if err != nil {
				t.Fatalf("Parser error for '%s': %v", tc.source, err)
			}

			gen := NewCodeGenerator()
			mem, _, err := gen.Generate(prog)
			if err != nil {
				t.Fatalf("Codegen error for '%s': %v", tc.source, err)
			}

			_, finalMem, err := runRelayEmulator(mem, 20000)
			if err != nil {
				t.Fatalf("Emulator error for '%s': %v", tc.source, err)
			}

			rAddr, ok := gen.symbols["R"]
			if !ok {
				t.Fatalf("Symbol R not found in symbol table")
			}

			actualVal := finalMem[rAddr]
			if actualVal != tc.expected {
				t.Errorf("Expected %d, got %d for '%s'", tc.expected, actualVal, tc.source)
			}
		})
	}
}

func runRelayEmulator(memory [MaxMemorySize]uint16, maxCycles int) (uint16, [MaxMemorySize]uint16, error) {
	pc := uint16(0)
	var acc uint16
	var stack uint16
	var carry uint16

	for cycles := 0; cycles < maxCycles; cycles++ {
		if pc >= MaxMemorySize {
			break
		}
		cmd := memory[pc]
		var opVal uint16
		if pc+1 < MaxMemorySize {
			opVal = memory[pc+1]
		}

		switch cmd {
		case OpNOP:
			pc = (pc + 1) & 0x03FF
		case OpLDA:
			acc = opVal
			pc = (pc + 2) & 0x03FF
		case OpLDAM:
			acc = memory[opVal&0x03FF]
			pc = (pc + 2) & 0x03FF
		case OpLDAMM:
			ptr := opVal & 0x03FF
			target := memory[ptr] & 0x03FF
			acc = memory[target]
			pc = (pc + 2) & 0x03FF
		case OpSTAM:
			addr := opVal & 0x03FF
			memory[addr] = acc
			pc = (pc + 2) & 0x03FF
		case OpSTAMM:
			ptr := opVal & 0x03FF
			target := memory[ptr] & 0x03FF
			memory[target] = acc
			pc = (pc + 2) & 0x03FF
		case OpADD:
			res := uint32(acc) + uint32(opVal)
			if res > 0xFFFF {
				carry = 1
			} else {
				carry = 0
			}
			acc = uint16(res & 0xFFFF)
			pc = (pc + 2) & 0x03FF
		case OpAND:
			acc = acc & opVal
			pc = (pc + 2) & 0x03FF
		case OpOR:
			acc = acc | opVal
			pc = (pc + 2) & 0x03FF
		case OpNOT:
			acc = ^acc
			pc = (pc + 2) & 0x03FF
		case OpSHR:
			carry = acc & 1
			acc = acc >> 1
			pc = (pc + 2) & 0x03FF
		case OpSHL:
			carry = (acc >> 15) & 1
			acc = (acc << 1) & 0xFFFF
			pc = (pc + 2) & 0x03FF
		case OpBA:
			if (opVal & 0x03FF) == pc {
				// Halt loop (BA to self)
				return acc, memory, nil
			}
			pc = opVal & 0x03FF
		case OpBC:
			if carry == 1 {
				pc = opVal & 0x03FF
			} else {
				pc = (pc + 2) & 0x03FF
			}
		case OpBN:
			if (acc & 0x8000) != 0 {
				pc = opVal & 0x03FF
			} else {
				pc = (pc + 2) & 0x03FF
			}
		case OpBE:
			if (acc & 0x0001) == 0 {
				pc = opVal & 0x03FF
			} else {
				pc = (pc + 2) & 0x03FF
			}
		case OpBZ:
			if acc == 0 {
				pc = opVal & 0x03FF
			} else {
				pc = (pc + 2) & 0x03FF
			}
		case OpCALL:
			stack = (pc + 2) & 0x03FF
			pc = opVal & 0x03FF
		case OpRET:
			pc = stack & 0x03FF
		default:
			return acc, memory, fmt.Errorf("Unknown opcode 0x%04X at PC: 0x%03X", cmd, pc)
		}
	}
	return acc, memory, fmt.Errorf("Exceeded cycle limit %d at PC 0x%03X", maxCycles, pc)
}

func TestCodeGen_Screen64(t *testing.T) {
	source := `10 SCREEN 64
20 A = 42
30 B = 99
40 CLS
50 END`
	l := lexer.NewLexer(source)
	tokens := l.Tokenize()
	p := parser.NewParser(tokens)
	prog, err := p.ParseProgram()
	if err != nil {
		t.Fatalf("Parser error: %v", err)
	}

	cg := NewCodeGenerator()
	_, _, err = cg.Generate(prog)
	if err != nil {
		t.Fatalf("Codegen error: %v", err)
	}

	if cg.resolution != 64 {
		t.Errorf("Expected resolution 64, got %d", cg.resolution)
	}
	if cg.displayBase != 768 {
		t.Errorf("Expected displayBase 768, got %d", cg.displayBase)
	}
	if cg.varBaseAddr != 734 {
		t.Errorf("Expected varBaseAddr 734, got %d", cg.varBaseAddr)
	}
	if cg.symbols["A"] != 734 {
		t.Errorf("Expected variable A at 734, got %d", cg.symbols["A"])
	}
	if cg.symbols["B"] != 735 {
		t.Errorf("Expected variable B at 735, got %d", cg.symbols["B"])
	}
}

func TestCodeGen_BitwiseLogic(t *testing.T) {
	testCases := []struct {
		name     string
		source   string
		expected uint16
	}{
		{"AND with literal", "10 A = 15\n20 R = A AND 6\n30 END", 6},
		{"AND with variable", "10 A = 15\n20 B = 6\n30 R = A AND B\n40 END", 6},
		{"OR with literal", "10 A = 9\n20 R = A OR 6\n30 END", 15},
		{"OR with variable", "10 A = 9\n20 B = 6\n30 R = A OR B\n40 END", 15},
		{"XOR with literal", "10 A = 12\n20 R = A XOR 10\n30 END", 6},
		{"XOR with variable", "10 A = 12\n20 B = 10\n30 R = A XOR B\n40 END", 6},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			l := lexer.NewLexer(tc.source)
			tokens := l.Tokenize()
			p := parser.NewParser(tokens)
			prog, err := p.ParseProgram()
			if err != nil {
				t.Fatalf("Parser error for '%s': %v", tc.source, err)
			}

			gen := NewCodeGenerator()
			mem, _, err := gen.Generate(prog)
			if err != nil {
				t.Fatalf("Codegen error for '%s': %v", tc.source, err)
			}

			_, finalMem, err := runRelayEmulator(mem, 20000)
			if err != nil {
				t.Fatalf("Emulator error for '%s': %v", tc.source, err)
			}

			rAddr, ok := gen.symbols["R"]
			if !ok {
				t.Fatalf("Symbol R not found in symbol table")
			}

			actualVal := finalMem[rAddr]
			if actualVal != tc.expected {
				t.Errorf("Expected %d, got %d for '%s'", tc.expected, actualVal, tc.source)
			}
		})
	}
}

func TestCodeGen_PowerOfTwoOptimizations(t *testing.T) {
	testCases := []struct {
		name     string
		source   string
		expected uint16
		checkDiv bool
		checkMul bool
	}{
		{"Div_by_2", "10 X = 64\n20 R = X / 2\n", 32, false, false},
		{"Div_by_4", "10 X = 64\n20 R = X / 4\n", 16, false, false},
		{"Div_by_16", "10 X = 64\n20 R = X / 16\n", 4, false, false},
		{"Div_by_32", "10 X = 160\n20 R = X / 32\n", 5, false, false},
		{"Mod_by_8", "10 X = 27\n20 R = X MOD 8\n", 3, false, false},
		{"Mod_by_32", "10 X = 35\n20 R = X MOD 32\n", 3, false, false},
		{"Mul_by_4", "10 X = 7\n20 R = X * 4\n", 28, false, false},
		{"Mul_by_8", "10 X = 5\n20 R = 8 * X\n", 40, false, false},
		{"Div_by_3_generic", "10 X = 100\n20 R = X / 3\n", 33, true, false},
		{"Mul_by_3_generic", "10 X = 7\n20 R = X * 3\n", 21, false, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			l := lexer.NewLexer(tc.source)
			tokens := l.Tokenize()
			p := parser.NewParser(tokens)
			prog, err := p.ParseProgram()
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			gen := NewCodeGenerator()
			mem, _, err := gen.Generate(prog)
			if err != nil {
				t.Fatalf("Codegen error: %v", err)
			}

			if tc.checkDiv && !gen.needDiv {
				t.Errorf("Expected needDiv=true for generic division")
			}
			if !tc.checkDiv && gen.needDiv {
				t.Errorf("Expected needDiv=false for optimized power-of-two division")
			}
			if tc.checkMul && !gen.needMul {
				t.Errorf("Expected needMul=true for generic multiplication")
			}
			if !tc.checkMul && gen.needMul {
				t.Errorf("Expected needMul=false for optimized power-of-two multiplication")
			}

			_, finalMem, err := runRelayEmulator(mem, 20000)
			if err != nil {
				t.Fatalf("Emulator error: %v", err)
			}

			rAddr, ok := gen.symbols["R"]
			if !ok {
				t.Fatalf("Symbol R not found in symbols")
			}

			if finalMem[rAddr] != tc.expected {
				t.Errorf("Expected %d, got %d for %s", tc.expected, finalMem[rAddr], tc.source)
			}
		})
	}
}

func TestCodeGen_MaxPerformanceOptimizations(t *testing.T) {
	t.Run("Direct_POKE_and_PEEK_Constants", func(t *testing.T) {
		src := "10 POKE 2, 42\n20 X = PEEK(3)\n30 OUT 2, 99\n40 END\n"
		l := lexer.NewLexer(src)
		prog, err := parser.NewParser(l.Tokenize()).ParseProgram()
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		gen := NewCodeGenerator()
		mem, comments, err := gen.Generate(prog)
		if err != nil {
			t.Fatalf("Codegen error: %v", err)
		}

		// Prüfe ob POKE 2 direkt per OpSTAM 2 abgelegt wird (ohne Patching)
		foundDirectPoke := false
		foundDirectPeek := false
		for pc := 4; pc < int(gen.pc); pc++ {
			if mem[pc] == OpSTAM && mem[pc+1] == 2 {
				foundDirectPoke = true
			}
			if mem[pc] == OpLDAM && mem[pc+1] == 3 {
				foundDirectPeek = true
			}
		}
		if !foundDirectPoke {
			t.Errorf("Expected direct OpSTAM 2 for POKE/OUT")
		}
		if !foundDirectPeek {
			t.Errorf("Expected direct OpLDAM 3 for PEEK(3)")
		}

		// Initialer Zustand mit IN Register (Adresse 3) = 77
		mem[3] = 77
		_, finalMem, err := runRelayEmulator(mem, 1000)
		if err != nil {
			t.Fatalf("Emulator error: %v", err)
		}
		xAddr := gen.symbols["X"]
		if finalMem[xAddr] != 77 {
			t.Errorf("Expected X=77 from PEEK(3), got %d", finalMem[xAddr])
		}
		if finalMem[2] != 99 {
			t.Errorf("Expected OUT (addr 2) = 99, got %d", finalMem[2])
		}
		_ = comments
	})

	t.Run("Optimized_FOR_Loop", func(t *testing.T) {
		src := "10 S = 0\n20 FOR I = 1 TO 10\n30 S = S + I\n40 NEXT I\n50 END\n"
		l := lexer.NewLexer(src)
		prog, err := parser.NewParser(l.Tokenize()).ParseProgram()
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		gen := NewCodeGenerator()
		mem, _, err := gen.Generate(prog)
		if err != nil {
			t.Fatalf("Codegen error: %v", err)
		}

		_, finalMem, err := runRelayEmulator(mem, 5000)
		if err != nil {
			t.Fatalf("Emulator error: %v", err)
		}
		sAddr := gen.symbols["S"]
		if finalMem[sAddr] != 55 {
			t.Errorf("Expected sum 1..10 = 55, got %d", finalMem[sAddr])
		}
	})

	t.Run("Left_Literal_Arithmetic", func(t *testing.T) {
		src := "10 Y = 7\n20 A = 10 + Y\n30 B = 20 - Y\n40 C = 0xFF AND Y\n50 D = 0x80 OR Y\n60 END\n"
		l := lexer.NewLexer(src)
		prog, err := parser.NewParser(l.Tokenize()).ParseProgram()
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		gen := NewCodeGenerator()
		mem, _, err := gen.Generate(prog)
		if err != nil {
			t.Fatalf("Codegen error: %v", err)
		}

		_, finalMem, err := runRelayEmulator(mem, 2000)
		if err != nil {
			t.Fatalf("Emulator error: %v", err)
		}
		if finalMem[gen.symbols["A"]] != 17 {
			t.Errorf("Expected 10+7=17, got %d", finalMem[gen.symbols["A"]])
		}
		if finalMem[gen.symbols["B"]] != 13 {
			t.Errorf("Expected 20-7=13, got %d", finalMem[gen.symbols["B"]])
		}
		if finalMem[gen.symbols["C"]] != 7 {
			t.Errorf("Expected 0xFF AND 7 = 7, got %d", finalMem[gen.symbols["C"]])
		}
		if finalMem[gen.symbols["D"]] != 0x87 {
			t.Errorf("Expected 0x80 OR 7 = 0x87, got %d", finalMem[gen.symbols["D"]])
		}
	})

	t.Run("PSET_Plot_Bitmask_Table", func(t *testing.T) {
		src := "10 CLS\n20 PLOT 0, 0, 1\n30 PLOT 16, 0, 1\n40 END\n"
		l := lexer.NewLexer(src)
		prog, err := parser.NewParser(l.Tokenize()).ParseProgram()
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		gen := NewCodeGenerator()
		mem, _, err := gen.Generate(prog)
		if err != nil {
			t.Fatalf("Codegen error: %v", err)
		}

		_, finalMem, err := runRelayEmulator(mem, 10000)
		if err != nil {
			t.Fatalf("Emulator error: %v", err)
		}
		// Word 1 (Offset 1) = Linke Hälfte (X=0..15), Addr DisplayBase + 1 = 961
		// Word 0 (Offset 0) = Rechte Hälfte (X=16..31), Addr DisplayBase + 0 = 960
		// PLOT 0,0,1 setzt Bit 15 in Word 1 (0x8000 in Addr 961)
		// PLOT 16,0,1 setzt Bit 15 in Word 0 (0x8000 in Addr 960)
		if finalMem[961] != 0x8000 {
			t.Errorf("Expected 0x8000 at display addr 961, got 0x%04X", finalMem[961])
		}
		if finalMem[960] != 0x8000 {
			t.Errorf("Expected 0x8000 at display addr 960, got 0x%04X", finalMem[960])
		}
	})

	t.Run("Direct_IF_And_IF_GOTO_Branching", func(t *testing.T) {
		src := `
LET X = -5
LET Y = 20
LET RES = 0
IF X < 0 THEN
    LET X = -X
END IF
IF X = 5 THEN
    LET RES = RES + 10
END IF
IF Y > 15 THEN
    GOTO JUMP_OK
END IF
LET RES = 999
JUMP_OK:
IF Y >= 20 THEN
    LET RES = RES + X
ELSE
    LET RES = 0
END IF
END
`
		l := lexer.NewLexer(src)
		prog, err := parser.NewParser(l.Tokenize()).ParseProgram()
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		gen := NewCodeGenerator()
		mem, _, err := gen.Generate(prog)
		if err != nil {
			t.Fatalf("Codegen error: %v", err)
		}

		_, finalMem, err := runRelayEmulator(mem, 2000)
		if err != nil {
			t.Fatalf("Emulator error: %v", err)
		}
		if finalMem[gen.symbols["RES"]] != 15 {
			t.Errorf("Expected RES=15, got %d", finalMem[gen.symbols["RES"]])
		}
	})

	t.Run("FOR_Negative_Step_And_Dynamic_Limit", func(t *testing.T) {
		src := `
LET SUM = 0
FOR I = 10 TO 2 STEP -2
    LET SUM = SUM + I
NEXT I
END
`
		l := lexer.NewLexer(src)
		prog, err := parser.NewParser(l.Tokenize()).ParseProgram()
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		gen := NewCodeGenerator()
		mem, _, err := gen.Generate(prog)
		if err != nil {
			t.Fatalf("Codegen error: %v", err)
		}

		_, finalMem, err := runRelayEmulator(mem, 5000)
		if err != nil {
			t.Fatalf("Emulator error: %v", err)
		}
		// 10 + 8 + 6 + 4 + 2 = 30
		if finalMem[gen.symbols["SUM"]] != 30 {
			t.Errorf("Expected SUM=30 for STEP -2 loop, got %d", finalMem[gen.symbols["SUM"]])
		}
	})

	t.Run("LINE_Bitmask_Table_And_STAMM", func(t *testing.T) {
		src := "10 CLS\n20 LINE 0, 0 TO 2, 0, 1\n30 END\n"
		l := lexer.NewLexer(src)
		prog, err := parser.NewParser(l.Tokenize()).ParseProgram()
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		gen := NewCodeGenerator()
		mem, _, err := gen.Generate(prog)
		if err != nil {
			t.Fatalf("Codegen error: %v", err)
		}

		_, finalMem, err := runRelayEmulator(mem, 10000)
		if err != nil {
			t.Fatalf("Emulator error: %v", err)
		}
		// Bits 15, 14, 13 (0x8000 | 0x4000 | 0x2000 = 0xE000) in Word 1 (Addr 961)
		if finalMem[961] != 0xE000 {
			t.Errorf("Expected 0xE000 at display addr 961 after LINE 0,0 TO 2,0, got 0x%04X", finalMem[961])
		}
	})
}

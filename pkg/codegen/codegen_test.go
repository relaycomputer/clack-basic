package codegen

import (
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

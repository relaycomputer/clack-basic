package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"
	"time"

	"clack-basic/pkg/assembler"
	"clack-basic/pkg/codegen"
	"clack-basic/pkg/decompiler"
	"clack-basic/pkg/emitter"
	"clack-basic/pkg/lexer"
	"clack-basic/pkg/parser"
)

func compileBasic(this js.Value, args []js.Value) any {
	if len(args) < 1 {
		return `{"success":false,"error":"Kein Quellcode übergeben"}`
	}
	source := args[0].String()
	format := "arduino"
	if len(args) > 1 && args[1].String() != "" {
		format = args[1].String()
	}

	startTime := time.Now()
	l := lexer.NewLexer(source)
	tokens := l.Tokenize()

	p := parser.NewParser(tokens)
	prog, err := p.ParseProgram()
	if err != nil {
		res, _ := json.Marshal(map[string]any{
			"success": false,
			"error":   fmt.Sprintf("%v", err),
		})
		return string(res)
	}

	cg := codegen.NewCodeGenerator()
	memory, comments, err := cg.Generate(prog)
	if err != nil {
		res, _ := json.Marshal(map[string]any{
			"success": false,
			"error":   fmt.Sprintf("Codegen-Fehler: %v", err),
		})
		return string(res)
	}

	duration := time.Since(startTime)
	usedWords := 0
	for _, val := range memory {
		if val != 0 {
			usedWords++
		}
	}

	fmtReq := emitter.OutputFormat(format)
	if fmtReq == "" {
		fmtReq = emitter.FormatArduino
	}

	code := emitter.Emit(memory, comments, fmtReq)
	listing := emitter.Emit(memory, comments, emitter.FormatListing)
	arduinoCode := emitter.Emit(memory, comments, emitter.FormatArduino)
	carrayCode := emitter.Emit(memory, comments, emitter.FormatCArray)

	res, _ := json.Marshal(map[string]any{
		"success":     true,
		"code":        code,
		"arduinoCode": arduinoCode,
		"carrayCode":  carrayCode,
		"listing":     listing,
		"usedWords":   usedWords,
		"totalWords":  codegen.MaxMemorySize,
		"durationMs":  float64(duration.Microseconds()) / 1000.0,
	})
	return string(res)
}

func assembleCode(this js.Value, args []js.Value) any {
	if len(args) < 1 {
		return `{"success":false,"error":"Kein Assembler-Quellcode übergeben"}`
	}
	source := args[0].String()
	startTime := time.Now()

	asm := assembler.NewAssembler()
	mem, used, err := asm.Assemble(source)
	if err != nil {
		res, _ := json.Marshal(map[string]any{
			"success": false,
			"error":   fmt.Sprintf("Assembler-Fehler: %v", err),
		})
		return string(res)
	}

	usedWords := 0
	for _, u := range used {
		if u {
			usedWords++
		}
	}

	arduinoCode := asm.FormatArduinoOutput()
	carrayCode := asm.FormatCArrayOutput()
	duration := time.Since(startTime)

	res, _ := json.Marshal(map[string]any{
		"success":     true,
		"arduinoCode": arduinoCode,
		"carrayCode":  carrayCode,
		"memory":      mem,
		"used":        used,
		"usedWords":   usedWords,
		"totalWords":  codegen.MaxMemorySize,
		"durationMs":  float64(duration.Microseconds()) / 1000.0,
	})
	return string(res)
}

func decompileCode(this js.Value, args []js.Value) any {
	if len(args) < 1 {
		return `{"success":false,"error":"Keine Quelldaten übergeben"}`
	}
	source := args[0].String()
	decomp := decompiler.NewDecompiler()
	err := decomp.LoadSource(source)
	if err != nil {
		asm := assembler.NewAssembler()
		mem, used, asmErr := asm.Assemble(source)
		if asmErr != nil {
			res, _ := json.Marshal(map[string]any{
				"success": false,
				"error":   fmt.Sprintf("Dekompilier-Fehler: %v", err),
			})
			return string(res)
		}
		decomp.SetMemory(mem, used[:])
	}

	basicCode, err := decomp.Decompile()
	if err != nil {
		res, _ := json.Marshal(map[string]any{
			"success": false,
			"error":   fmt.Sprintf("Dekompilier-Fehler: %v", err),
		})
		return string(res)
	}

	res, _ := json.Marshal(map[string]any{
		"success":   true,
		"basicCode": basicCode,
	})
	return string(res)
}

func disassembleCode(this js.Value, args []js.Value) any {
	if len(args) < 1 {
		return `{"success":false,"error":"Keine Quelldaten übergeben"}`
	}
	source := args[0].String()
	decomp := decompiler.NewDecompiler()
	err := decomp.LoadSource(source)
	if err != nil {
		res, _ := json.Marshal(map[string]any{
			"success": false,
			"error":   fmt.Sprintf("Disassembler-Fehler: %v", err),
		})
		return string(res)
	}

	asmCode, err := decomp.DisassembleToASM()
	if err != nil {
		res, _ := json.Marshal(map[string]any{
			"success": false,
			"error":   fmt.Sprintf("Disassembler-Fehler: %v", err),
		})
		return string(res)
	}

	res, _ := json.Marshal(map[string]any{
		"success": true,
		"asmCode": asmCode,
	})
	return string(res)
}

func main() {
	js.Global().Set("clackCompile", js.FuncOf(compileBasic))
	js.Global().Set("clackAssemble", js.FuncOf(assembleCode))
	js.Global().Set("clackDecompile", js.FuncOf(decompileCode))
	js.Global().Set("clackDisassemble", js.FuncOf(disassembleCode))
	js.Global().Set("clackWasmReady", true)
	println("⚡ Clack WebAssembly Cross-Compiler Engine ready!")
	select {}
}

package emitter

/*
 ==============================================================================
 📤 EMITTER & FORMATIERER FÜR CLACK-BASIC
 ==============================================================================
 Formatiert den generierten Maschinencode in das gewünschte Zielformat:
  1) Arduino-Loader Format (CLR, 0 0x4000, ..., RUN)
  2) C-Array Format (daten[0] = 0x4000;)
  3) Assembler-Listing mit Disassembly & Kommentaren
 ==============================================================================
*/

import (
	"fmt"
	"strings"

	"clack-basic/pkg/codegen"
)

type OutputFormat string

const (
	FormatArduino OutputFormat = "arduino" // Standard: Für upload.py / relay-upload
	FormatCArray  OutputFormat = "c-array" // Für C/C++ Code & Simulation
	FormatListing OutputFormat = "listing" // Menschenlesbares Disassembly
)

// Emit erzeugt die Textausgabe für das übergebene Speicherabbild im gewählten Format.
func Emit(memory [codegen.MaxMemorySize]uint16, comments map[uint16]string, format OutputFormat) string {
	switch format {
	case FormatCArray:
		return emitCArray(memory, comments)
	case FormatListing:
		return emitListing(memory, comments)
	default:
		return emitArduino(memory, comments)
	}
}

func emitArduino(memory [codegen.MaxMemorySize]uint16, comments map[uint16]string) string {
	var sb strings.Builder
	sb.WriteString("// ============================================================\n")
	sb.WriteString("// ⚡ Generiert von Clack-BASIC Cross-Compiler (Arduino Loader Format)\n")
	sb.WriteString("// ============================================================\n")
	sb.WriteString("CLR\n\n")

	for i := 0; i < codegen.MaxMemorySize; i++ {
		val := memory[i]
		if val != 0 {
			comment := ""
			if c, ok := comments[uint16(i)]; ok {
				comment = fmt.Sprintf(" // %s", c)
			}
			sb.WriteString(fmt.Sprintf("%d 0x%04X%s\n", i, val, comment))
		}
	}

	sb.WriteString("\nRUN\n")
	return sb.String()
}

func emitCArray(memory [codegen.MaxMemorySize]uint16, comments map[uint16]string) string {
	var sb strings.Builder
	sb.WriteString("// ============================================================\n")
	sb.WriteString("// ⚡ Generiert von Clack-BASIC Cross-Compiler (C-Array Format)\n")
	sb.WriteString("// ============================================================\n\n")

	for i := 0; i < codegen.MaxMemorySize; i++ {
		val := memory[i]
		if val != 0 {
			comment := ""
			if c, ok := comments[uint16(i)]; ok {
				comment = fmt.Sprintf(" // %s", c)
			}
			sb.WriteString(fmt.Sprintf("daten[%d] = 0x%04X;%s\n", i, val, comment))
		}
	}
	return sb.String()
}

func emitListing(memory [codegen.MaxMemorySize]uint16, comments map[uint16]string) string {
	var sb strings.Builder
	sb.WriteString("==============================================================================\n")
	sb.WriteString(" ADR   HEX    DEZ    BEFEHL / KOMMENTAR\n")
	sb.WriteString("==============================================================================\n")

	for i := 0; i < codegen.MaxMemorySize; i++ {
		val := memory[i]
		if val != 0 {
			comment := ""
			if c, ok := comments[uint16(i)]; ok {
				comment = c
			}
			sb.WriteString(fmt.Sprintf(" [%04d] 0x%04X (%5d)  %s\n", i, val, int16(val), comment))
		}
	}
	sb.WriteString("==============================================================================\n")
	return sb.String()
}

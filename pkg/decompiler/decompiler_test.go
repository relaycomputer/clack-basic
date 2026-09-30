package decompiler

import (
	"strings"
	"testing"
)

func TestDecompiler_UserDivisionSnippet(t *testing.T) {
	snippet := `daten[0] = 0x4000;		// NOP
daten[1] = 400;

// DIV_LOOP
daten[11] = 0x1800;		// LDAM
daten[12] = 202;			// bitposition
daten[13] = 0x4018;		// BZ
daten[14] = 83;         // Finished
daten[15] = 0x3440;		//ADD
daten[16] = 0xFFFF;		// -1
daten[17] = 0x2800;		// STAM
daten[18] = 202;			// Save bitposition -1
daten[85] = 0x6000;    // RETURN

daten[200] = 0;		// Quotient
daten[201] = 0;		// Rest
daten[202] = 16;	// Bitposition

daten[400] = 0x5000;   // CALL to DIV
daten[401] = 11;
`

	d := NewDecompiler()
	err := d.LoadSource(snippet)
	if err != nil {
		t.Fatalf("LoadSource failed: %v", err)
	}

	asm, err := d.DisassembleToASM()
	if err != nil {
		t.Fatalf("DisassembleToASM failed: %v", err)
	}

	t.Logf("Generated ASM:\n%s", asm)

	if !strings.Contains(asm, "CALL SUB_11") {
		t.Errorf("Expected CALL SUB_11 in ASM, got:\n%s", asm)
	}
	if !strings.Contains(asm, "SUB_11:") {
		t.Errorf("Expected SUB_11: in ASM")
	}
	if !strings.Contains(asm, "ORG 200") {
		t.Errorf("Expected ORG 200 in ASM")
	}
}

func TestDecompiler_BlockArray(t *testing.T) {
	cArray := `const uint16_t prog[] PROGMEM = {
		0x1400, 42,
		0x2800, 500,
		0x4000, 4
	};`

	d := NewDecompiler()
	err := d.LoadSource(cArray)
	if err != nil {
		t.Fatalf("LoadSource failed: %v", err)
	}

	asm, err := d.DisassembleToASM()
	if err != nil {
		t.Fatalf("DisassembleToASM failed: %v", err)
	}

	if !strings.Contains(asm, "LDA #42") || !strings.Contains(asm, "STAM 500") {
		t.Errorf("Unexpected ASM output:\n%s", asm)
	}
}

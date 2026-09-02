package codegen

/*
 ==============================================================================
 ⚡ 16-BIT OPCODES & INSTRUCTION MASKS FÜR RELAISRECHNER
 ==============================================================================
 Definitionen der Maschinencode-Befehle gemäß Hardwarespezifikation.
 ==============================================================================
*/

const (
	// Basis-Opcodes
	OpNOP   uint16 = 0xF000
	OpLDA   uint16 = 0x1400 // LDA #Immediate
	OpLDAM  uint16 = 0x1800 // LDAM [DirectAddr]
	OpLDAMM uint16 = 0x1C00 // LDAMM [[IndirectAddr]]
	OpSTAM  uint16 = 0x2800 // STAM [DirectAddr]
	OpSTAMM uint16 = 0x2C00 // STAMM [[IndirectAddr]]

	// ALU-Befehle (Immediate Modus)
	OpADD uint16 = 0x3440 // ADD #Imm
	OpAND uint16 = 0x3480 // AND #Imm
	OpOR  uint16 = 0x34C0 // OR #Imm
	OpNOT uint16 = 0x3540 // NOT A (Bitinversion)
	OpSHR uint16 = 0x3580 // SHR A (Rechtsshift)
	OpSHL uint16 = 0x35C0 // SHL A (Linksshift)

	// Sprungbefehle
	OpBA uint16 = 0x4000 // Branch Always
	OpBC uint16 = 0x4008 // Branch if Carry == 1
	OpBN uint16 = 0x4010 // Branch if Negative (Bit 15 == 1)
	OpBZ uint16 = 0x4018 // Branch if Zero (Accu == 0)
	OpBE uint16 = 0x4020 // Branch if Even (Bit 0 == 0)

	// Unterprogramm-Befehle
	OpCALL uint16 = 0x5000 // CALL Target (SR = PC+2, PC = Target)
	OpRET  uint16 = 0x6000 // RET (PC = SR)
)

// Instruction repräsentiert ein einzelnes Wort oder Befehlspaar im Speicher.
type Instruction struct {
	Address uint16
	Opcode  uint16
	Operand uint16
	HasOp   bool
	Comment string
	Label   string
}

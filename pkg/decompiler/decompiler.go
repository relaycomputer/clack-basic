package decompiler

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"clack-basic/pkg/codegen"
)

type Instruction struct {
	PC       int
	Opcode   uint16
	Operand  uint16
	HasOp    bool
	Mnemonic string
	IsJump   bool
	TargetPC int
	Comment  string
}

type Decompiler struct {
	memory     [1024]uint16
	used       [1024]bool
	jumpLabels map[int]string
	varNames   map[int]string
	comments   map[int]string
}

func NewDecompiler() *Decompiler {
	return &Decompiler{
		jumpLabels: make(map[int]string),
		varNames:   make(map[int]string),
		comments:   make(map[int]string),
	}
}

// LoadSource parses multiple array and machine code representations:
// 1) C / Arduino array assignments: daten[0] = 0x4000; // Comment
// 2) C99 designated initializers: [0] = 0x4000,
// 3) VHDL indexed: 0 => x"4000",
// 4) Arduino loader / simple lines: 0 0x4000 // Comment
// 5) Hex dump / labels: 0: 0x4000 or 0000: 4000
// 6) C/C++ array lists: uint16_t daten[] = { 0x4000, 400, ... };
// 7) VHDL lists: ( x"4000", x"0190", ... )
// 8) Comma/space-separated hex/decimal stream
func (d *Decompiler) LoadSource(source string) error {
	// Clear memory
	for i := range d.memory {
		d.memory[i] = 0
		d.used[i] = false
	}
	d.comments = make(map[int]string)
	d.jumpLabels = make(map[int]string)
	d.varNames = make(map[int]string)

	reBlockComment := regexp.MustCompile(`(?s)/\*.*?\*/`)
	cleanSource := reBlockComment.ReplaceAllString(source, "")

	// Regexes for single line / indexed statements
	reCArray := regexp.MustCompile(`(?i)(?:daten|array|mem|memory|ram|rom|buffer)\s*\[\s*(\d+)\s*\]\s*=\s*(0x[0-9A-Fa-f]+|\d+)\s*;?(?:\s*//\s*(.*))?`)
	reC99Array := regexp.MustCompile(`(?i)\[\s*(\d+)\s*\]\s*=\s*(0x[0-9A-Fa-f]+|\d+)\s*,?(?:\s*//\s*(.*))?`)
	reVhdlIndexed := regexp.MustCompile(`(?i)(\d+)\s*=>\s*(?:x"([0-9A-Fa-f]+)"|(0x[0-9A-Fa-f]+|\d+))\s*,?(?:\s*--\s*(.*))?`)
	reAddrVal := regexp.MustCompile(`^\s*(\d+)[\s:]+(0x[0-9A-Fa-f]+|\d+)\b(?:\s*//\s*(.*))?`)

	lines := strings.Split(cleanSource, "\n")
	hasMachineLines := false

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" || strings.EqualFold(trimmed, "CLR") || strings.EqualFold(trimmed, "RUN") {
			continue
		}

		// 1. C-Array: daten[11] = 0x1800; // Comment
		if match := reCArray.FindStringSubmatch(trimmed); len(match) >= 3 {
			addrStr := match[1]
			valStr := match[2]
			comment := ""
			if len(match) > 3 {
				comment = strings.TrimSpace(match[3])
			}
			if d.setVal(addrStr, valStr, comment) {
				hasMachineLines = true
				continue
			}
		}

		// 2. C99 Designated: [11] = 0x1800,
		if match := reC99Array.FindStringSubmatch(trimmed); len(match) >= 3 {
			addrStr := match[1]
			valStr := match[2]
			comment := ""
			if len(match) > 3 {
				comment = strings.TrimSpace(match[3])
			}
			if d.setVal(addrStr, valStr, comment) {
				hasMachineLines = true
				continue
			}
		}

		// 3. VHDL: 11 => x"1800",
		if match := reVhdlIndexed.FindStringSubmatch(trimmed); len(match) >= 4 {
			addrStr := match[1]
			valStr := match[2]
			if valStr == "" {
				valStr = match[3]
			} else {
				valStr = "0x" + valStr
			}
			comment := ""
			if len(match) > 4 {
				comment = strings.TrimSpace(match[4])
			}
			if d.setVal(addrStr, valStr, comment) {
				hasMachineLines = true
				continue
			}
		}

		// 4. Addr Val: 11 0x1800 // Comment
		if match := reAddrVal.FindStringSubmatch(trimmed); len(match) >= 3 {
			addrStr := match[1]
			valStr := match[2]
			comment := ""
			if len(match) > 3 {
				comment = strings.TrimSpace(match[3])
			}
			if d.setVal(addrStr, valStr, comment) {
				hasMachineLines = true
				continue
			}
		}
	}

	// If no indexed statements were found, try parsing as a sequential array block `{ ... }` or `( ... )`
	if !hasMachineLines {
		hasMachineLines = d.parseArrayBlock(cleanSource)
	}

	if !hasMachineLines {
		return fmt.Errorf("keine gueltigen Maschinencode-Zeilen oder Array-Strukturen (daten[n]=..., { 0x... }, '0 0x1400') gefunden")
	}

	return nil
}

func (d *Decompiler) parseArrayBlock(text string) bool {
	// Find content inside { ... } or ( ... ) or [ ... ]
	startIdx := strings.IndexAny(text, "{([")
	endIdx := strings.LastIndexAny(text, "})]")

	content := text
	if startIdx >= 0 && endIdx > startIdx {
		content = text[startIdx+1 : endIdx]
	}

	// Split by commas or whitespace
	reTokens := regexp.MustCompile(`(?:0x[0-9A-Fa-f]+|x"[0-9A-Fa-f]+|\b\d+\b)`)
	matches := reTokens.FindAllString(content, -1)

	if len(matches) == 0 {
		return false
	}

	for idx, token := range matches {
		if idx >= 1024 {
			break
		}
		var val uint64
		var err error
		if strings.HasPrefix(strings.ToLower(token), "x\"") {
			hexStr := strings.Trim(token[2:], "\"")
			val, err = strconv.ParseUint(hexStr, 16, 16)
		} else if strings.HasPrefix(strings.ToLower(token), "0x") {
			val, err = strconv.ParseUint(token[2:], 16, 16)
		} else {
			val, err = strconv.ParseUint(token, 10, 16)
		}
		if err == nil {
			d.memory[idx] = uint16(val)
			d.used[idx] = true
		}
	}

	return true
}

func (d *Decompiler) setVal(addrStr, valStr, comment string) bool {
	idx64, err := strconv.ParseUint(addrStr, 10, 16)
	if err != nil || idx64 >= 1024 {
		return false
	}
	var val64 uint64
	if strings.HasPrefix(strings.ToLower(valStr), "0x") {
		val64, err = strconv.ParseUint(valStr[2:], 16, 16)
	} else {
		val64, err = strconv.ParseUint(valStr, 10, 16)
	}
	if err != nil {
		return false
	}

	d.memory[idx64] = uint16(val64)
	d.used[idx64] = true
	if comment != "" {
		d.comments[int(idx64)] = comment
	}
	return true
}

func (d *Decompiler) SetMemory(mem [1024]uint16, used []bool) {
	d.memory = mem
	for i := range d.used {
		if i < len(used) {
			d.used[i] = used[i]
		} else {
			d.used[i] = true
		}
	}
}

// DisassembleToASM converts the loaded machine array into clean, labeled 16-bit Assembler source code (.asm)
func (d *Decompiler) DisassembleToASM() (string, error) {
	// 1. First pass: find all branch and call targets to create labels
	targets := make(map[int]bool)
	callTargets := make(map[int]bool)

	for pc := 0; pc < 1024; pc++ {
		if !d.used[pc] && pc > 0 && d.memory[pc] == 0 {
			continue
		}
		cmd := d.memory[pc]
		switch cmd {
		case codegen.OpBA, codegen.OpBC, codegen.OpBN, codegen.OpBZ, codegen.OpBE:
			if pc+1 < 1024 {
				target := int(d.memory[pc+1])
				targets[target] = true
				d.registerLabel(target)
			}
			pc++
		case codegen.OpCALL:
			if pc+1 < 1024 {
				target := int(d.memory[pc+1])
				callTargets[target] = true
				d.registerSubLabel(target)
			}
			pc++
		case codegen.OpLDA, codegen.OpLDAM, codegen.OpLDAMM, codegen.OpSTAM, codegen.OpSTAMM,
			codegen.OpADD, codegen.OpAND, codegen.OpOR, codegen.OpNOT, codegen.OpSHR, codegen.OpSHL:
			pc++
		}
	}

	// 2. Second pass: format into Assembler
	var sb strings.Builder
	sb.WriteString("; ============================================================\n")
	sb.WriteString("; ⚡ 16-Bit Relais-Assembler (Dekompiliert aus C/Array-Maschinencode)\n")
	sb.WriteString("; ============================================================\n\n")

	// Get all used addresses
	var usedAddrs []int
	for i := 0; i < 1024; i++ {
		if d.used[i] {
			usedAddrs = append(usedAddrs, i)
		}
	}
	sort.Ints(usedAddrs)

	if len(usedAddrs) == 0 {
		return "; Leeres Programm", nil
	}

	pc := 0
	currentOrg := -1

	for pc < 1024 {
		if !d.used[pc] {
			// Find next used address
			nextUsed := -1
			for k := pc; k < 1024; k++ {
				if d.used[k] {
					nextUsed = k
					break
				}
			}
			if nextUsed == -1 {
				break
			}
			// Skip gap with ORG
			if nextUsed > pc+1 {
				sb.WriteString(fmt.Sprintf("\nORG %d\n\n", nextUsed))
				currentOrg = nextUsed
			}
			pc = nextUsed
		}

		if currentOrg == -1 && pc == 0 {
			sb.WriteString("ORG 0\n\n")
			currentOrg = 0
		}

		// Print Label if this address is targeted
		if labelName, ok := d.jumpLabels[pc]; ok {
			sb.WriteString(fmt.Sprintf("\n%s:\n", labelName))
		}

		cmd := d.memory[pc]
		c1 := d.comments[pc]
		var c1Str string
		if c1 != "" {
			c1Str = "  ; " + c1
		}

		switch cmd {
		case codegen.OpNOP:
			sb.WriteString(fmt.Sprintf("    NOP%s\n", c1Str))
			pc++
		case codegen.OpLDA:
			op := d.memory[pc+1]
			c2 := d.comments[pc+1]
			comment := d.combineComments(c1, c2)
			if op > 0x8000 && op <= 0xFFFF {
				neg := int(0x10000 - int(op))
				sb.WriteString(fmt.Sprintf("    LDA #-%d%s\n", neg, comment))
			} else if op > 255 {
				sb.WriteString(fmt.Sprintf("    LDA #0x%04X%s\n", op, comment))
			} else {
				sb.WriteString(fmt.Sprintf("    LDA #%d%s\n", op, comment))
			}
			pc += 2
		case codegen.OpLDAM:
			op := d.memory[pc+1]
			c2 := d.comments[pc+1]
			comment := d.combineComments(c1, c2)
			sb.WriteString(fmt.Sprintf("    LDAM %d%s\n", op, comment))
			pc += 2
		case codegen.OpLDAMM:
			op := d.memory[pc+1]
			c2 := d.comments[pc+1]
			comment := d.combineComments(c1, c2)
			sb.WriteString(fmt.Sprintf("    LDAMM %d%s\n", op, comment))
			pc += 2
		case codegen.OpSTAM:
			op := d.memory[pc+1]
			c2 := d.comments[pc+1]
			comment := d.combineComments(c1, c2)
			sb.WriteString(fmt.Sprintf("    STAM %d%s\n", op, comment))
			pc += 2
		case codegen.OpSTAMM:
			op := d.memory[pc+1]
			c2 := d.comments[pc+1]
			comment := d.combineComments(c1, c2)
			sb.WriteString(fmt.Sprintf("    STAMM %d%s\n", op, comment))
			pc += 2
		case codegen.OpADD:
			op := d.memory[pc+1]
			c2 := d.comments[pc+1]
			comment := d.combineComments(c1, c2)
			if op == 0xFFFF {
				sb.WriteString(fmt.Sprintf("    ADD #-1%s\n", comment))
			} else if op > 0x8000 {
				neg := int(0x10000 - int(op))
				sb.WriteString(fmt.Sprintf("    ADD #-%d%s\n", neg, comment))
			} else {
				sb.WriteString(fmt.Sprintf("    ADD #%d%s\n", op, comment))
			}
			pc += 2
		case codegen.OpAND:
			op := d.memory[pc+1]
			c2 := d.comments[pc+1]
			comment := d.combineComments(c1, c2)
			sb.WriteString(fmt.Sprintf("    AND #0x%04X%s\n", op, comment))
			pc += 2
		case codegen.OpOR:
			op := d.memory[pc+1]
			c2 := d.comments[pc+1]
			comment := d.combineComments(c1, c2)
			sb.WriteString(fmt.Sprintf("    OR #0x%04X%s\n", op, comment))
			pc += 2
		case codegen.OpNOT:
			c2 := d.comments[pc+1]
			comment := d.combineComments(c1, c2)
			sb.WriteString(fmt.Sprintf("    NOT%s\n", comment))
			pc += 2
		case codegen.OpSHR:
			c2 := d.comments[pc+1]
			comment := d.combineComments(c1, c2)
			sb.WriteString(fmt.Sprintf("    SHR%s\n", comment))
			pc += 2
		case codegen.OpSHL:
			c2 := d.comments[pc+1]
			comment := d.combineComments(c1, c2)
			sb.WriteString(fmt.Sprintf("    SHL%s\n", comment))
			pc += 2
		case codegen.OpBA:
			target := int(d.memory[pc+1])
			c2 := d.comments[pc+1]
			comment := d.combineComments(c1, c2)
			sb.WriteString(fmt.Sprintf("    BA %s%s\n", d.getLabel(target), comment))
			pc += 2
		case codegen.OpBC:
			target := int(d.memory[pc+1])
			c2 := d.comments[pc+1]
			comment := d.combineComments(c1, c2)
			sb.WriteString(fmt.Sprintf("    BC %s%s\n", d.getLabel(target), comment))
			pc += 2
		case codegen.OpBN:
			target := int(d.memory[pc+1])
			c2 := d.comments[pc+1]
			comment := d.combineComments(c1, c2)
			sb.WriteString(fmt.Sprintf("    BN %s%s\n", d.getLabel(target), comment))
			pc += 2
		case codegen.OpBZ:
			target := int(d.memory[pc+1])
			c2 := d.comments[pc+1]
			comment := d.combineComments(c1, c2)
			sb.WriteString(fmt.Sprintf("    BZ %s%s\n", d.getLabel(target), comment))
			pc += 2
		case codegen.OpBE:
			target := int(d.memory[pc+1])
			c2 := d.comments[pc+1]
			comment := d.combineComments(c1, c2)
			sb.WriteString(fmt.Sprintf("    BE %s%s\n", d.getLabel(target), comment))
			pc += 2
		case codegen.OpCALL:
			target := int(d.memory[pc+1])
			c2 := d.comments[pc+1]
			comment := d.combineComments(c1, c2)
			sb.WriteString(fmt.Sprintf("    CALL %s%s\n", d.getLabel(target), comment))
			pc += 2
		case codegen.OpRET:
			sb.WriteString(fmt.Sprintf("    RET%s\n", c1Str))
			pc++
		default:
			// Raw data word (e.g. variable initializers)
			sb.WriteString(fmt.Sprintf("    DATA %d%s\n", cmd, c1Str))
			pc++
		}
	}

	return sb.String(), nil
}

func (d *Decompiler) combineComments(c1, c2 string) string {
	c1 = strings.TrimSpace(c1)
	c2 = strings.TrimSpace(c2)
	if c1 != "" && c2 != "" {
		if strings.EqualFold(c1, c2) {
			return "  ; " + c1
		}
		return "  ; " + c1 + " | " + c2
	}
	if c1 != "" {
		return "  ; " + c1
	}
	if c2 != "" {
		return "  ; " + c2
	}
	return ""
}

// Decompile decompiles machine code memory into high-level Clack-BASIC code (.bas)
func (d *Decompiler) Decompile() (string, error) {
	// 1. Disassemble instruction stream
	var instructions []Instruction
	pc := 0

	for pc < 1024 {
		if !d.used[pc] && pc > 0 && d.memory[pc] == 0 {
			// Check if rest is unused
			allZero := true
			for k := pc; k < 1024; k++ {
				if d.used[k] || d.memory[k] != 0 {
					allZero = false
					break
				}
			}
			if allZero {
				break
			}
		}

		cmd := d.memory[pc]
		inst := Instruction{PC: pc, Opcode: cmd, Comment: d.comments[pc]}

		switch cmd {
		case codegen.OpNOP:
			inst.Mnemonic = "NOP"
			instructions = append(instructions, inst)
			pc++
		case codegen.OpLDA:
			inst.Mnemonic = "LDA"
			inst.HasOp = true
			inst.Operand = d.memory[pc+1]
			instructions = append(instructions, inst)
			pc += 2
		case codegen.OpLDAM:
			inst.Mnemonic = "LDAM"
			inst.HasOp = true
			inst.Operand = d.memory[pc+1]
			instructions = append(instructions, inst)
			d.registerVar(int(inst.Operand))
			pc += 2
		case codegen.OpLDAMM:
			inst.Mnemonic = "LDAMM"
			inst.HasOp = true
			inst.Operand = d.memory[pc+1]
			instructions = append(instructions, inst)
			d.registerVar(int(inst.Operand))
			pc += 2
		case codegen.OpSTAM:
			inst.Mnemonic = "STAM"
			inst.HasOp = true
			inst.Operand = d.memory[pc+1]
			instructions = append(instructions, inst)
			d.registerVar(int(inst.Operand))
			pc += 2
		case codegen.OpSTAMM:
			inst.Mnemonic = "STAMM"
			inst.HasOp = true
			inst.Operand = d.memory[pc+1]
			instructions = append(instructions, inst)
			d.registerVar(int(inst.Operand))
			pc += 2
		case codegen.OpADD:
			inst.Mnemonic = "ADD"
			inst.HasOp = true
			inst.Operand = d.memory[pc+1]
			instructions = append(instructions, inst)
			pc += 2
		case codegen.OpAND:
			inst.Mnemonic = "AND"
			inst.HasOp = true
			inst.Operand = d.memory[pc+1]
			instructions = append(instructions, inst)
			pc += 2
		case codegen.OpOR:
			inst.Mnemonic = "OR"
			inst.HasOp = true
			inst.Operand = d.memory[pc+1]
			instructions = append(instructions, inst)
			pc += 2
		case codegen.OpNOT:
			inst.Mnemonic = "NOT"
			inst.HasOp = true
			inst.Operand = d.memory[pc+1]
			instructions = append(instructions, inst)
			pc += 2
		case codegen.OpSHR:
			inst.Mnemonic = "SHR"
			inst.HasOp = true
			inst.Operand = d.memory[pc+1]
			instructions = append(instructions, inst)
			pc += 2
		case codegen.OpSHL:
			inst.Mnemonic = "SHL"
			inst.HasOp = true
			inst.Operand = d.memory[pc+1]
			instructions = append(instructions, inst)
			pc += 2
		case codegen.OpBA:
			inst.Mnemonic = "BA"
			inst.HasOp = true
			inst.IsJump = true
			inst.TargetPC = int(d.memory[pc+1])
			inst.Operand = d.memory[pc+1]
			instructions = append(instructions, inst)
			d.registerLabel(inst.TargetPC)
			pc += 2
		case codegen.OpBC:
			inst.Mnemonic = "BC"
			inst.HasOp = true
			inst.IsJump = true
			inst.TargetPC = int(d.memory[pc+1])
			inst.Operand = d.memory[pc+1]
			instructions = append(instructions, inst)
			d.registerLabel(inst.TargetPC)
			pc += 2
		case codegen.OpBN:
			inst.Mnemonic = "BN"
			inst.HasOp = true
			inst.IsJump = true
			inst.TargetPC = int(d.memory[pc+1])
			inst.Operand = d.memory[pc+1]
			instructions = append(instructions, inst)
			d.registerLabel(inst.TargetPC)
			pc += 2
		case codegen.OpBZ:
			inst.Mnemonic = "BZ"
			inst.HasOp = true
			inst.IsJump = true
			inst.TargetPC = int(d.memory[pc+1])
			inst.Operand = d.memory[pc+1]
			instructions = append(instructions, inst)
			d.registerLabel(inst.TargetPC)
			pc += 2
		case codegen.OpBE:
			inst.Mnemonic = "BE"
			inst.HasOp = true
			inst.IsJump = true
			inst.TargetPC = int(d.memory[pc+1])
			inst.Operand = d.memory[pc+1]
			instructions = append(instructions, inst)
			d.registerLabel(inst.TargetPC)
			pc += 2
		case codegen.OpCALL:
			inst.Mnemonic = "CALL"
			inst.HasOp = true
			inst.IsJump = true
			inst.TargetPC = int(d.memory[pc+1])
			inst.Operand = d.memory[pc+1]
			instructions = append(instructions, inst)
			d.registerSubLabel(inst.TargetPC)
			pc += 2
		case codegen.OpRET:
			inst.Mnemonic = "RET"
			instructions = append(instructions, inst)
			pc++
		default:
			// Raw data word
			inst.Mnemonic = "DATA"
			inst.Operand = cmd
			instructions = append(instructions, inst)
			pc++
		}
	}

	// 2. High-Level BASIC Pattern Recognition
	var sb strings.Builder
	sb.WriteString("REM ============================================================\n")
	sb.WriteString("REM ⚡ Clack-BASIC: Automatisch dekompiliert aus Assembler/Maschinencode\n")
	sb.WriteString("REM ============================================================\n\n")

	i := 0
	for i < len(instructions) {
		inst := instructions[i]

		// Print Label if this PC is targeted
		if labelName, ok := d.jumpLabels[inst.PC]; ok {
			sb.WriteString(fmt.Sprintf("\n%s:\n", labelName))
		}

		// Pattern: Infinite Loop HALT (BA to self)
		if inst.Mnemonic == "BA" && inst.TargetPC == inst.PC {
			sb.WriteString("    HALT\n")
			i++
			continue
		}

		// Pattern: LET var = immediate (LDA #imm; STAM var)
		if inst.Mnemonic == "LDA" && i+1 < len(instructions) && instructions[i+1].Mnemonic == "STAM" {
			val := inst.Operand
			dstVar := d.getVarName(int(instructions[i+1].Operand))
			sb.WriteString(fmt.Sprintf("    LET %s = %s\n", dstVar, d.formatNumber(val)))
			i += 2
			continue
		}

		// Pattern: LET dst = src (LDAM src; STAM dst)
		if inst.Mnemonic == "LDAM" && i+1 < len(instructions) && instructions[i+1].Mnemonic == "STAM" {
			srcVar := d.getVarName(int(inst.Operand))
			dstVar := d.getVarName(int(instructions[i+1].Operand))
			sb.WriteString(fmt.Sprintf("    LET %s = %s\n", dstVar, srcVar))
			i += 2
			continue
		}

		// Pattern: LET var = var + imm (LDAM var; ADD #imm; STAM var)
		if inst.Mnemonic == "LDAM" && i+2 < len(instructions) &&
			instructions[i+1].Mnemonic == "ADD" &&
			instructions[i+2].Mnemonic == "STAM" &&
			inst.Operand == instructions[i+2].Operand {
			vName := d.getVarName(int(inst.Operand))
			addVal := instructions[i+1].Operand
			if addVal == 0xFFFF {
				sb.WriteString(fmt.Sprintf("    LET %s = %s - 1\n", vName, vName))
			} else if addVal > 0x8000 {
				neg := int(0x10000 - int(addVal))
				sb.WriteString(fmt.Sprintf("    LET %s = %s - %d\n", vName, vName, neg))
			} else {
				sb.WriteString(fmt.Sprintf("    LET %s = %s + %d\n", vName, vName, addVal))
			}
			i += 3
			continue
		}

		// Pattern: LET dst = src + imm (LDAM src; ADD #imm; STAM dst)
		if inst.Mnemonic == "LDAM" && i+2 < len(instructions) &&
			instructions[i+1].Mnemonic == "ADD" &&
			instructions[i+2].Mnemonic == "STAM" {
			srcVar := d.getVarName(int(inst.Operand))
			dstVar := d.getVarName(int(instructions[i+2].Operand))
			addVal := instructions[i+1].Operand
			if addVal == 0xFFFF {
				sb.WriteString(fmt.Sprintf("    LET %s = %s - 1\n", dstVar, srcVar))
			} else if addVal > 0x8000 {
				neg := int(0x10000 - int(addVal))
				sb.WriteString(fmt.Sprintf("    LET %s = %s - %d\n", dstVar, srcVar, neg))
			} else {
				sb.WriteString(fmt.Sprintf("    LET %s = %s + %d\n", dstVar, srcVar, addVal))
			}
			i += 3
			continue
		}

		// Pattern: LET var = var AND mask (LDAM var; AND #mask; STAM var)
		if inst.Mnemonic == "LDAM" && i+2 < len(instructions) &&
			instructions[i+1].Mnemonic == "AND" &&
			instructions[i+2].Mnemonic == "STAM" &&
			inst.Operand == instructions[i+2].Operand {
			vName := d.getVarName(int(inst.Operand))
			mask := instructions[i+1].Operand
			sb.WriteString(fmt.Sprintf("    LET %s = %s AND 0x%04X\n", vName, vName, mask))
			i += 3
			continue
		}

		// Pattern: LET var = var OR mask (LDAM var; OR #mask; STAM var)
		if inst.Mnemonic == "LDAM" && i+2 < len(instructions) &&
			instructions[i+1].Mnemonic == "OR" &&
			instructions[i+2].Mnemonic == "STAM" &&
			inst.Operand == instructions[i+2].Operand {
			vName := d.getVarName(int(inst.Operand))
			mask := instructions[i+1].Operand
			sb.WriteString(fmt.Sprintf("    LET %s = %s OR 0x%04X\n", vName, vName, mask))
			i += 3
			continue
		}

		// Pattern: LET var = var * 2 (LDAM var; SHL; STAM var)
		if inst.Mnemonic == "LDAM" && i+2 < len(instructions) &&
			instructions[i+1].Mnemonic == "SHL" &&
			instructions[i+2].Mnemonic == "STAM" &&
			inst.Operand == instructions[i+2].Operand {
			vName := d.getVarName(int(inst.Operand))
			sb.WriteString(fmt.Sprintf("    LET %s = %s * 2\n", vName, vName))
			i += 3
			continue
		}

		// Pattern: LET var = var / 2 (LDAM var; SHR; STAM var)
		if inst.Mnemonic == "LDAM" && i+2 < len(instructions) &&
			instructions[i+1].Mnemonic == "SHR" &&
			instructions[i+2].Mnemonic == "STAM" &&
			inst.Operand == instructions[i+2].Operand {
			vName := d.getVarName(int(inst.Operand))
			sb.WriteString(fmt.Sprintf("    LET %s = %s / 2\n", vName, vName))
			i += 3
			continue
		}

		// Pattern: LET var = -var (LDAM var; NOT; ADD #1; STAM var)
		if inst.Mnemonic == "LDAM" && i+3 < len(instructions) &&
			instructions[i+1].Mnemonic == "NOT" &&
			instructions[i+2].Mnemonic == "ADD" && instructions[i+2].Operand == 1 &&
			instructions[i+3].Mnemonic == "STAM" &&
			inst.Operand == instructions[i+3].Operand {
			vName := d.getVarName(int(inst.Operand))
			sb.WriteString(fmt.Sprintf("    LET %s = -%s\n", vName, vName))
			i += 4
			continue
		}

		// Pattern: Conditional branch IF var == K THEN GOTO label (LDAM var; ADD #-K; BZ target)
		if inst.Mnemonic == "LDAM" && i+2 < len(instructions) &&
			instructions[i+1].Mnemonic == "ADD" &&
			instructions[i+2].Mnemonic == "BZ" {
			vName := d.getVarName(int(inst.Operand))
			addVal := instructions[i+1].Operand
			target := d.getLabel(instructions[i+2].TargetPC)
			if addVal >= 0x8000 {
				kVal := int(0x10000 - int(addVal))
				sb.WriteString(fmt.Sprintf("    IF %s == %d THEN GOTO %s\n", vName, kVal, target))
			} else {
				sb.WriteString(fmt.Sprintf("    IF %s == -%d THEN GOTO %s\n", vName, addVal, target))
			}
			i += 3
			continue
		}

		// Pattern: Conditional branch IF var == 0 THEN GOTO label (LDAM var; BZ target)
		if inst.Mnemonic == "LDAM" && i+1 < len(instructions) && instructions[i+1].Mnemonic == "BZ" {
			vName := d.getVarName(int(inst.Operand))
			target := d.getLabel(instructions[i+1].TargetPC)
			sb.WriteString(fmt.Sprintf("    IF %s == 0 THEN GOTO %s\n", vName, target))
			i += 2
			continue
		}

		// Pattern: Conditional branch IF var < 0 THEN GOTO label (LDAM var; BN target)
		if inst.Mnemonic == "LDAM" && i+1 < len(instructions) && instructions[i+1].Mnemonic == "BN" {
			vName := d.getVarName(int(inst.Operand))
			target := d.getLabel(instructions[i+1].TargetPC)
			sb.WriteString(fmt.Sprintf("    IF %s < 0 THEN GOTO %s\n", vName, target))
			i += 2
			continue
		}

		// Pattern: Conditional branch IF (var AND 1) == 0 THEN GOTO label (LDAM var; BE target)
		if inst.Mnemonic == "LDAM" && i+1 < len(instructions) && instructions[i+1].Mnemonic == "BE" {
			vName := d.getVarName(int(inst.Operand))
			target := d.getLabel(instructions[i+1].TargetPC)
			sb.WriteString(fmt.Sprintf("    IF (%s AND 1) == 0 THEN GOTO %s\n", vName, target))
			i += 2
			continue
		}

		// Direct Jumps / Calls / Returns
		if inst.Mnemonic == "BA" {
			sb.WriteString(fmt.Sprintf("    GOTO %s\n", d.getLabel(inst.TargetPC)))
			i++
			continue
		}
		if inst.Mnemonic == "BZ" {
			sb.WriteString(fmt.Sprintf("    IF ACCU == 0 THEN GOTO %s\n", d.getLabel(inst.TargetPC)))
			i++
			continue
		}
		if inst.Mnemonic == "BN" {
			sb.WriteString(fmt.Sprintf("    IF ACCU < 0 THEN GOTO %s\n", d.getLabel(inst.TargetPC)))
			i++
			continue
		}
		if inst.Mnemonic == "BC" {
			sb.WriteString(fmt.Sprintf("    IF CARRY == 1 THEN GOTO %s\n", d.getLabel(inst.TargetPC)))
			i++
			continue
		}
		if inst.Mnemonic == "BE" {
			sb.WriteString(fmt.Sprintf("    IF (ACCU AND 1) == 0 THEN GOTO %s\n", d.getLabel(inst.TargetPC)))
			i++
			continue
		}
		if inst.Mnemonic == "CALL" {
			sb.WriteString(fmt.Sprintf("    GOSUB %s\n", d.getLabel(inst.TargetPC)))
			i++
			continue
		}
		if inst.Mnemonic == "RET" {
			sb.WriteString("    RETURN\n")
			i++
			continue
		}
		if inst.Mnemonic == "NOP" {
			sb.WriteString("    REM NOP\n")
			i++
			continue
		}

		// Single Load/Store/Math fallback
		if inst.Mnemonic == "LDA" {
			sb.WriteString(fmt.Sprintf("    LET ACCU = %s\n", d.formatNumber(inst.Operand)))
		} else if inst.Mnemonic == "LDAM" {
			sb.WriteString(fmt.Sprintf("    LET ACCU = %s\n", d.getVarName(int(inst.Operand))))
		} else if inst.Mnemonic == "LDAMM" {
			sb.WriteString(fmt.Sprintf("    LET ACCU = PEEK(%s)\n", d.getVarName(int(inst.Operand))))
		} else if inst.Mnemonic == "STAM" {
			if inst.Operand >= 960 && inst.Operand <= 1023 {
				sb.WriteString(fmt.Sprintf("    POKE %d, ACCU  REM Display Wort %d\n", inst.Operand, inst.Operand-960))
			} else {
				sb.WriteString(fmt.Sprintf("    LET %s = ACCU\n", d.getVarName(int(inst.Operand))))
			}
		} else if inst.Mnemonic == "STAMM" {
			sb.WriteString(fmt.Sprintf("    POKE %s, ACCU\n", d.getVarName(int(inst.Operand))))
		} else if inst.Mnemonic == "ADD" {
			sb.WriteString(fmt.Sprintf("    LET ACCU = ACCU + %s\n", d.formatNumber(inst.Operand)))
		} else if inst.Mnemonic == "AND" {
			sb.WriteString(fmt.Sprintf("    LET ACCU = ACCU AND 0x%04X\n", inst.Operand))
		} else if inst.Mnemonic == "OR" {
			sb.WriteString(fmt.Sprintf("    LET ACCU = ACCU OR 0x%04X\n", inst.Operand))
		} else if inst.Mnemonic == "NOT" {
			sb.WriteString("    LET ACCU = NOT ACCU\n")
		} else if inst.Mnemonic == "SHR" {
			sb.WriteString("    LET ACCU = ACCU / 2\n")
		} else if inst.Mnemonic == "SHL" {
			sb.WriteString("    LET ACCU = ACCU * 2\n")
		} else if inst.Mnemonic == "DATA" {
			if inst.Operand == 0 {
				zeroCount := 0
				j := i
				for j < len(instructions) && instructions[j].Mnemonic == "DATA" && instructions[j].Operand == 0 {
					if _, hasLabel := d.jumpLabels[instructions[j].PC]; hasLabel && j > i {
						break
					}
					zeroCount++
					j++
				}
				if zeroCount > 3 {
					sb.WriteString(fmt.Sprintf("    REM [%d freie Null-Worte]\n", zeroCount))
					i += zeroCount
					continue
				}
			}
			sb.WriteString(fmt.Sprintf("    DATA %s\n", d.formatNumber(inst.Operand)))
		}

		i++
	}

	return sb.String(), nil
}

func (d *Decompiler) registerLabel(targetPC int) {
	if _, ok := d.jumpLabels[targetPC]; !ok {
		d.jumpLabels[targetPC] = fmt.Sprintf("LABEL_%d", targetPC)
	}
}

func (d *Decompiler) registerSubLabel(targetPC int) {
	if _, ok := d.jumpLabels[targetPC]; !ok {
		d.jumpLabels[targetPC] = fmt.Sprintf("SUB_%d", targetPC)
	}
}

func (d *Decompiler) getLabel(targetPC int) string {
	if name, ok := d.jumpLabels[targetPC]; ok {
		return name
	}
	return fmt.Sprintf("LABEL_%d", targetPC)
}

func (d *Decompiler) registerVar(addr int) {
	if addr >= 960 && addr <= 1023 {
		return // Framebuffer
	}
	if _, ok := d.varNames[addr]; !ok {
		// Common known variables or generic V_addr
		d.varNames[addr] = fmt.Sprintf("V_%d", addr)
	}
}

func (d *Decompiler) getVarName(addr int) string {
	if addr >= 960 && addr <= 1023 {
		return fmt.Sprintf("DISPLAY_%d", addr-960)
	}
	if name, ok := d.varNames[addr]; ok {
		return name
	}
	return fmt.Sprintf("V_%d", addr)
}

func (d *Decompiler) formatNumber(val uint16) string {
	if val > 0x8000 && val <= 0xFFFF {
		neg := int(0x10000 - int(val))
		if neg <= 100 {
			return fmt.Sprintf("-%d", neg)
		}
	}
	if val > 255 {
		return fmt.Sprintf("0x%04X", val)
	}
	return fmt.Sprintf("%d", val)
}

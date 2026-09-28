package decompiler

import (
	"fmt"
	"regexp"
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
}

type Decompiler struct {
	memory     [1024]uint16
	used       [1024]bool
	jumpLabels map[int]string
	varNames   map[int]string
}

func NewDecompiler() *Decompiler {
	return &Decompiler{
		jumpLabels: make(map[int]string),
		varNames:   make(map[int]string),
	}
}

func (d *Decompiler) LoadSource(source string) error {
	// Clear memory
	for i := range d.memory {
		d.memory[i] = 0
		d.used[i] = false
	}

	reBlockComment := regexp.MustCompile(`(?s)/\*.*?\*/`)
	cleanSource := reBlockComment.ReplaceAllString(source, "")

	reCArray := regexp.MustCompile(`(?i)daten\[\s*(\d+)\s*\]\s*=\s*(0x[0-9A-Fa-f]+|\d+)\s*;`)
	reAddrVal := regexp.MustCompile(`^\s*(\d+)\s+(0x[0-9A-Fa-f]+|\d+)\b`)

	lines := strings.Split(cleanSource, "\n")
	hasMachineLines := false

	for _, rawLine := range lines {
		line := rawLine
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}
		if idx := strings.Index(line, ";"); idx >= 0 && !strings.Contains(line, "daten[") {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.EqualFold(line, "CLR") || strings.EqualFold(line, "RUN") {
			continue
		}

		var addrStr, valStr string
		if match := reCArray.FindStringSubmatch(line); len(match) == 3 {
			addrStr = match[1]
			valStr = match[2]
		} else if match := reAddrVal.FindStringSubmatch(line); len(match) == 3 {
			addrStr = match[1]
			valStr = match[2]
		}

		if addrStr != "" && valStr != "" {
			idx64, err := strconv.ParseUint(addrStr, 10, 16)
			if err != nil || idx64 >= 1024 {
				continue
			}
			var val64 uint64
			if strings.HasPrefix(strings.ToLower(valStr), "0x") {
				val64, err = strconv.ParseUint(valStr[2:], 16, 16)
			} else {
				val64, err = strconv.ParseUint(valStr, 10, 16)
			}
			if err == nil {
				d.memory[idx64] = uint16(val64)
				d.used[idx64] = true
				hasMachineLines = true
			}
		}
	}

	if !hasMachineLines {
		return fmt.Errorf("keine gueltigen Maschinencode-Zeilen (daten[n]=... oder '0 0x1400') gefunden")
	}

	return nil
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
		inst := Instruction{PC: pc, Opcode: cmd}

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

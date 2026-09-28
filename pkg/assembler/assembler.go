package assembler

import (
	"fmt"
	"strconv"
	"strings"

	"clack-basic/pkg/codegen"
)

type InstructionDef struct {
	Opcode  uint16
	HasOp   bool
	IsJump  bool
	IsMemOp bool
}

var Mnemonics = map[string]InstructionDef{
	"NOP":   {Opcode: codegen.OpNOP, HasOp: false},
	"LDA":   {Opcode: codegen.OpLDA, HasOp: true},
	"LDAM":  {Opcode: codegen.OpLDAM, HasOp: true, IsMemOp: true},
	"LDAMM": {Opcode: codegen.OpLDAMM, HasOp: true, IsMemOp: true},
	"STAM":  {Opcode: codegen.OpSTAM, HasOp: true, IsMemOp: true},
	"STAMM": {Opcode: codegen.OpSTAMM, HasOp: true, IsMemOp: true},
	"ADD":   {Opcode: codegen.OpADD, HasOp: true},
	"AND":   {Opcode: codegen.OpAND, HasOp: true},
	"OR":    {Opcode: codegen.OpOR, HasOp: true},
	"NOT":   {Opcode: codegen.OpNOT, HasOp: true},
	"SHR":   {Opcode: codegen.OpSHR, HasOp: true},
	"SHL":   {Opcode: codegen.OpSHL, HasOp: true},
	"BA":    {Opcode: codegen.OpBA, HasOp: true, IsJump: true},
	"BC":    {Opcode: codegen.OpBC, HasOp: true, IsJump: true},
	"BN":    {Opcode: codegen.OpBN, HasOp: true, IsJump: true},
	"BZ":    {Opcode: codegen.OpBZ, HasOp: true, IsJump: true},
	"BE":    {Opcode: codegen.OpBE, HasOp: true, IsJump: true},
	"CALL":  {Opcode: codegen.OpCALL, HasOp: true, IsJump: true},
	"RET":   {Opcode: codegen.OpRET, HasOp: false},
	"HALT":  {Opcode: codegen.OpBA, HasOp: true, IsJump: true},
}

type Assembler struct {
	symbols map[string]int
	memory  [1024]uint16
	used    [1024]bool
	maxAddr int
}

func NewAssembler() *Assembler {
	return &Assembler{
		symbols: make(map[string]int),
	}
}

type ParsedLine struct {
	LineNum  int
	Label    string
	Mnemonic string
	Operand  string
	Raw      string
	IsData   bool
	DataVals []string
	IsOrg    bool
	OrgVal   string
	IsEqu    bool
	EquName  string
	EquVal   string
}

func (a *Assembler) Assemble(source string) ([1024]uint16, [1024]bool, error) {
	lines := strings.Split(source, "\n")
	var parsed []ParsedLine

	for idx, rawLine := range lines {
		lineNum := idx + 1
		line := rawLine

		if pos := strings.Index(line, ";"); pos >= 0 {
			line = line[:pos]
		}
		if pos := strings.Index(line, "//"); pos >= 0 {
			line = line[:pos]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		p := ParsedLine{LineNum: lineNum, Raw: rawLine}

		if strings.HasPrefix(strings.ToLower(line), "daten[") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				addrStr := strings.Trim(parts[0][6:], " ]\t")
				valStr := strings.Trim(parts[1], " ;\t")
				p.IsData = true
				p.OrgVal = addrStr
				p.DataVals = []string{valStr}
				parsed = append(parsed, p)
				continue
			}
		}

		if colon := strings.Index(line, ":"); colon >= 0 {
			p.Label = strings.TrimSpace(line[:colon])
			line = strings.TrimSpace(line[colon+1:])
			if line == "" {
				parsed = append(parsed, p)
				continue
			}
		}

		tokens := strings.Fields(line)
		if len(tokens) == 0 {
			parsed = append(parsed, p)
			continue
		}

		first := strings.ToUpper(tokens[0])

		if first == "ORG" {
			p.IsOrg = true
			if len(tokens) > 1 {
				p.OrgVal = tokens[1]
			}
		} else if first == "EQU" {
			p.IsEqu = true
			if len(tokens) >= 3 {
				p.EquName = tokens[1]
				p.EquVal = tokens[2]
			}
		} else if first == "DATA" || first == "DW" || first == "DB" {
			p.IsData = true
			dataPart := strings.TrimSpace(line[len(tokens[0]):])
			vals := strings.Split(dataPart, ",")
			for _, v := range vals {
				v = strings.TrimSpace(v)
				if v != "" {
					p.DataVals = append(p.DataVals, v)
				}
			}
		} else {
			p.Mnemonic = first
			if len(tokens) > 1 {
				p.Operand = strings.TrimSpace(strings.Join(tokens[1:], " "))
			}
		}

		parsed = append(parsed, p)
	}

	// Pass 1: Collect symbols
	pc := 0
	for _, p := range parsed {
		if p.IsOrg {
			addr, err := a.parseValue(p.OrgVal, pc)
			if err != nil {
				return a.memory, a.used, fmt.Errorf("Zeile %d: Ungueltige ORG-Adresse '%s': %w", p.LineNum, p.OrgVal, err)
			}
			pc = addr
			continue
		}

		if p.IsEqu {
			val, err := a.parseValue(p.EquVal, pc)
			if err != nil {
				return a.memory, a.used, fmt.Errorf("Zeile %d: Ungueltiger EQU-Wert '%s': %w", p.LineNum, p.EquVal, err)
			}
			a.symbols[strings.ToUpper(p.EquName)] = val
			continue
		}

		if p.Label != "" {
			a.symbols[strings.ToUpper(p.Label)] = pc
		}

		if p.IsData {
			pc += len(p.DataVals)
			continue
		}

		if p.Mnemonic == "" {
			continue
		}

		def, ok := Mnemonics[p.Mnemonic]
		if !ok {
			return a.memory, a.used, fmt.Errorf("Zeile %d: Unbekannter Befehl '%s'", p.LineNum, p.Mnemonic)
		}

		if p.Mnemonic == "NOP" || p.Mnemonic == "RET" {
			pc++
		} else if def.HasOp {
			pc += 2
		} else {
			pc++
		}
	}

	// Pass 2: Generate words
	pc = 0
	for _, p := range parsed {
		if p.IsOrg {
			addr, _ := a.parseValue(p.OrgVal, pc)
			pc = addr
			continue
		}
		if p.IsEqu {
			continue
		}

		if p.IsData {
			for _, dv := range p.DataVals {
				val, err := a.parseValue(dv, pc)
				if err != nil {
					return a.memory, a.used, fmt.Errorf("Zeile %d: Ungueltiger Datenwert '%s': %w", p.LineNum, dv, err)
				}
				if pc >= 1024 {
					return a.memory, a.used, fmt.Errorf("Zeile %d: Speicherueberlauf (>1023 Worte)", p.LineNum)
				}
				a.memory[pc] = uint16(val & 0xFFFF)
				a.used[pc] = true
				if pc > a.maxAddr {
					a.maxAddr = pc
				}
				pc++
			}
			continue
		}

		if p.Mnemonic == "" {
			continue
		}

		def := Mnemonics[p.Mnemonic]

		if p.Mnemonic == "HALT" {
			a.emitWord(pc, codegen.OpBA)
			a.emitWord(pc+1, uint16(pc))
			pc += 2
			continue
		}

		if p.Mnemonic == "NOP" {
			a.emitWord(pc, codegen.OpNOP)
			pc++
			continue
		}

		if p.Mnemonic == "RET" {
			a.emitWord(pc, codegen.OpRET)
			pc++
			continue
		}

		if p.Mnemonic == "NOT" || p.Mnemonic == "SHR" || p.Mnemonic == "SHL" {
			a.emitWord(pc, def.Opcode)
			a.emitWord(pc+1, 0)
			pc += 2
			continue
		}

		a.emitWord(pc, def.Opcode)

		opStr := p.Operand
		opStr = strings.TrimPrefix(opStr, "#")
		opStr = strings.TrimPrefix(opStr, "[")
		opStr = strings.TrimSuffix(opStr, "]")
		opStr = strings.TrimSpace(opStr)

		if opStr == "" {
			return a.memory, a.used, fmt.Errorf("Zeile %d: Fehlender Operand fuer '%s'", p.LineNum, p.Mnemonic)
		}

		opVal, err := a.parseValue(opStr, pc)
		if err != nil {
			return a.memory, a.used, fmt.Errorf("Zeile %d: Ungueltiger Operand '%s': %w", p.LineNum, opStr, err)
		}

		a.emitWord(pc+1, uint16(opVal&0xFFFF))
		pc += 2
	}

	return a.memory, a.used, nil
}

func (a *Assembler) emitWord(addr int, val uint16) {
	if addr >= 0 && addr < 1024 {
		a.memory[addr] = val
		a.used[addr] = true
		if addr > a.maxAddr {
			a.maxAddr = addr
		}
	}
}

func (a *Assembler) parseValue(s string, currentPC int) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("leerer Wert")
	}

	if s == "$" || s == "*" {
		return currentPC, nil
	}

	if val, ok := a.symbols[strings.ToUpper(s)]; ok {
		return val, nil
	}

	if strings.HasPrefix(strings.ToLower(s), "0x") {
		v, err := strconv.ParseUint(s[2:], 16, 16)
		return int(v), err
	}
	if strings.HasPrefix(s, "$") {
		v, err := strconv.ParseUint(s[1:], 16, 16)
		return int(v), err
	}

	if strings.HasPrefix(s, "%") {
		v, err := strconv.ParseUint(s[1:], 2, 16)
		return int(v), err
	}
	if strings.HasPrefix(strings.ToLower(s), "0b") {
		v, err := strconv.ParseUint(s[2:], 2, 16)
		return int(v), err
	}

	if strings.HasPrefix(s, "-") {
		v, err := strconv.ParseInt(s, 10, 32)
		return int(uint16(v & 0xFFFF)), err
	}

	v, err := strconv.ParseUint(s, 10, 16)
	return int(v), err
}

func (a *Assembler) FormatArduinoOutput() string {
	var sb strings.Builder
	for i := 0; i <= a.maxAddr; i++ {
		if a.used[i] {
			sb.WriteString(fmt.Sprintf("%d 0x%04X\n", i, a.memory[i]))
		}
	}
	return sb.String()
}

func (a *Assembler) FormatCArrayOutput() string {
	var sb strings.Builder
	for i := 0; i <= a.maxAddr; i++ {
		if a.used[i] {
			sb.WriteString(fmt.Sprintf("daten[%d] = 0x%04X;\n", i, a.memory[i]))
		}
	}
	return sb.String()
}

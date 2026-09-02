# 📖 Befehlssatz & Opcode-Referenz

Der Relaisrechner besitzt eine 16-Bit Befehlsarchitektur mit einem einzigen Programmierer-Register (**Accu**).

---

## 1. Opcode-Bitaufbau (16 Bit)

$$\text{Opcode} = [\text{Typ: 4 Bit}] \, [\text{Adr: 2 Bit}] \, [\text{MOP: 4 Bit}] \, [\text{B: 3 Bit}] \, [\text{Res: 3 Bit}]$$

| Bitbereich | Feld | Beschreibung | Werte |
| :---: | :---: | :--- | :--- |
| `15 .. 12` | **Typ** | Grundbefehlstyp | `0001` (LD), `0010` (ST), `0011` (MOP/ALU), `0100` (Branch), `0101` (CALL), `0110` (RET), `1111` (NOP = `0xF000`) |
| `11 .. 10` | **Adr** | Adressierungsart | `00` (---), `01` (Immediate `X`), `10` (Direct `M(X)`), `11` (Indirect `M(M(X))`) |
| `9 .. 6` | **MOP** | ALU-Rechenoperation | `0001` (ADD), `0010` (AND), `0011` (OR), `0101` (NOT), `0110` (SHR), `0111` (SHL) |
| `5 .. 3` | **B** | Sprungbedingung | `000` (BA - Always), `001` (BC - Carry), `010` (BN - Negativ), `011` (BZ - Zero), `100` (BE - Even) |
| `2 .. 0` | **Res** | Reserviert | `000` |

---

## 2. Befehlstabelle

| Assembler-Befehl | Hex-Opcode | Operand | Zyklen | Beschreibung |
| :--- | :--- | :---: | :---: | :--- |
| `NOP` | `0xF000` | — | 2 | Keine Operation (Load next command) |
| `LDA #X` | `0x1400` | `X` | 4 | Konstante `X` in Accu laden |
| `LDAM [M]` | `0x1800` | `M` | 6 | Wert aus Speicherzelle `M` in Accu laden |
| `LDAMM [[M]]` | `0x1C00` | `M` | 8 | Indirekter Speicherzugriff über Zeiger `[M]` |
| `STAM [M]` | `0x2800` | `M` | 6 | Accu in Speicherzelle `M` schreiben |
| `STAMM [[M]]` | `0x2C00` | `M` | 8 | Accu indirekt in Zeigeradresse `[[M]]` schreiben |
| `ADD #X` | `0x3440` | `X` | 4 | `Accu = Accu + X` (setzt Carry) |
| `AND #X` | `0x3480` | `X` | 4 | `Accu = Accu & X` |
| `OR #X` | `0x34C0` | `X` | 4 | `Accu = Accu \| X` |
| `NOT A` | `0x3540` | 0 | 4 | `Accu = ~Accu` (Bitweise Invertierung) |
| `SHR A` | `0x3580` | 0 | 4 | `Accu = Accu >> 1` (Logischer Rechtsshift) |
| `SHL A` | `0x35C0` | 0 | 4 | `Accu = Accu << 1` (Linksshift) |
| `BA Ziel` | `0x4000` | `Ziel` | 4 | Unbedingter Sprung |
| `BC Ziel` | `0x4008` | `Ziel` | 4 | Sprung wenn Carry == 1 |
| `BN Ziel` | `0x4010` | `Ziel` | 4 | Sprung wenn Accu < 0 (Bit 15 == 1) |
| `BZ Ziel` | `0x4018` | `Ziel` | 4 | Sprung wenn Accu == 0 |
| `BE Ziel` | `0x4020` | `Ziel` | 4 | Sprung wenn Accu gerade (Bit 0 == 0) |
| `CALL Ziel` | `0x5000` | `Ziel` | 4 | Unterprogrammaufruf (`SR = PC+2, PC = Ziel`) |
| `RET` | `0x6000` | — | 5 | Rücksprung aus Unterprogramm (`PC = SR`) |

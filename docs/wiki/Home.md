# ⚡ Clack-BASIC: 16-Bit Relais-Computer Cross-Compiler

Willkommen im offiziellen Wiki des **Clack-BASIC Cross-Compilers** für den 16-Bit Relais-Computer!

---

## 📖 Inhaltsverzeichnis

1. [🏛️ Compiler-Architektur & Pipeline](Compiler-Architektur)
2. [⚙️ Befehlssatz, Opcodes & Dynamic Patching](Befehlssatz-und-Opcodes)
3. [📦 Integrierte Runtime-Routinen (Mul, Div, Pset, Cls, Data)](Runtime-Routinen)
4. [🏓 Code-Beispiele (Ping-Pong, Grafik, Mathe, DATA)](Code-Beispiele)

---

## 🚀 Schnellstart

```bash
# BASIC-Quellcode kompilieren (Standard: Arduino Loader Format):
./bin/clack compile examples/ping_pong.bas -o ping_pong.txt

# Als Assembler-Listing mit Disassembly anzeigen:
./bin/clack compile examples/math_test.bas --format=listing

# Direkt auf den echten Relais-Computer flashen:
./bin/clack flash examples/ping_pong.bas -m
```

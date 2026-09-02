# ⚡ Clack-BASIC

**Clack-BASIC** (benannt nach dem unverkennbaren Klacken der Relaiskontakte ⚡) ist der Go-basierte Cross-Compiler für den 16-Bit Relaisrechner.

## 📁 Projektstruktur

* `docs/` — Dokumentation, Befehlssatz-Spezifikationen, Excel-Tabellen und Notizen
* `cmd/` — CLI-Einstiegspunkt (`clack` / `cbc`)
* `pkg/` — Compiler-Module (Lexer, Parser, AST, Codegen, Runtime-Routinen, Emitter)
* `examples/` — BASIC-Beispielprogramme (`.bas`)

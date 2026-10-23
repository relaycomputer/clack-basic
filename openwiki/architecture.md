---
type: architecture
title: Clack-Basic Compiler Architecture
description: Modular Go compiler architecture translating Clack-BASIC into 16-bit relay machine code.
tags: [compiler, architecture, relay-computing, go]
---

# Clack-Basic Compiler Architecture

Clack-Basic ist ein modularer Cross-Compiler und Decompiler, geschrieben in Go, der BASIC-Quellcode direkt in optimierten 16-Bit-Maschinencode für die physische Relais-CPU und den Emulator übersetzt.

## 1. Compiler-Pipeline

```
BASIC Quellcode (*.bas)
         │
         ▼
[Lexer (pkg/lexer)] ──> Token Stream (Keywords, Literale, Operatoren)
         │
         ▼
[Pratt-Parser (pkg/parser)] ──> Abstract Syntax Tree (AST mit Operator-Präzedenz)
         │
         ▼
[Codegen (pkg/codegen)] ──> 16-Bit Maschinencode + Label-Fixups + Software-Runtime (__runtime_*)
         │
         ▼
[Emitter] ──> Maschinencode-Arrays, Binary ROM, Arduino RAM-Loader oder WebAssembly
```

## 2. Kernkomponenten

- **Lexer (`pkg/lexer`):** Zerlegt den Quelltext deterministisch in Tokens.
- **Pratt-Parser (`pkg/parser`):** Parst arithmetische und logische Ausdrücke unter strikter Einhaltung von Präzedenzen (`*` vor `+`).
- **Codegenerator (`pkg/codegen`):** Übersetzt AST-Knoten in Relais-Mikroinstruktionen, reserviert Variablen im RAM und bettet bei Bedarf Software-Routinen ein.
- **Decompiler (`pkg/decompiler`):** Ermöglicht die Rückübersetzung von 16-Bit Maschinencode in lesbaren Assembler- oder BASIC-Code.
- **WebAssembly Engine (`wasm/`):** Exponiert den Compiler für Browser-Umgebungen und das Clack-Studio IDE.

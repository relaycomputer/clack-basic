---
type: guide
title: Clack-Basic Quickstart & Compilation Guide
description: Quickstart guide for compiling BASIC programs to 16-bit relay machine code.
tags: [quickstart, guide, compiler, basic]
---

# Clack-Basic Quickstart & Compilation Guide

## 1. Voraussetzungen

- **Go 1.21+** (`go version`)
- **Make** (optional für Build-Automatisierung)

## 2. Bauen des Compilers

```bash
cd /home/veitscherbub/work/relay-computing/clack-basic
go build -o bin/clack-basic main.go
```

## 3. Kompilieren eines BASIC-Programms

```bash
# Beispiel: Mandelbrot-Fraktal kompilieren
./bin/clack-basic compile ../mandelbrot_clack_v3.bas -o dist/mandelbrot.bin

# Kompilieren mit Ausgabe als Arduino-C++-Array für Hardware-Upload
./bin/clack-basic compile ../mandelbrot_clack_v3.bas --format arduino
```

## 4. Unterstützte Grafikbefehle

- `CLS`: Löscht das 32×32 Relais-Display (Adressen 960..1023).
- `PLOT X, Y, Color`: Setzt ein Pixel bei Koordinate $(X, Y)$ ($0 \le X, Y \le 31$).
- `LINE X1, Y1 TO X2, Y2`: Zeichnet eine Linie per Hardware-Bresenham.

---
type: concept
title: Clack 16-Bit Relay Architecture & Computational Constraints
description: Hardware execution model, cycle timing, fixed-point math, and memory mapping for the 16-bit relay CPU.
tags: [concept, architecture, relay-cpu, hardware, timing, fixed-point]
---

# Clack 16-Bit Relay Architecture & Computational Constraints

Die Clack-Architektur beschreibt einen diskret aufgebauten 16-Bit-Computer auf Basis elektromechanischer Relais. Aufgrund der physikalischen Trägheit der Relaiszungen und des Kontaktprellens unterliegt das System besonderen Hardware- und Timing-Restriktionen, die direkt in den Compilerbau und die mathematischen Bibliotheken von `clack-basic` einfließen.

---

## 1. Hardware-Taktung & Elektromechanik

* **Taktperiode pro T-State:** `70 ms` (entspricht einer Hardware-Basisfrequenz von ca. `14.28 Hz`).
* **Kontaktprellen:** Vor jedem Latch-Signal ist ein Sicherheitsfenster von mindestens `500 µs` für stabile Buspegel zwingend erforderlich.
* **Zykluskosten:**
  * Einfache Instruktionen (`NOP`): 2 T-States (140 ms).
  * Register- und ALU-Instruktionen (`LDA`, `ADD`, `AND`, `OR`, `SHL`, `SHR`): 4 T-States (280 ms).
  * Direkte RAM-Zugriffe (`LDAM`, `STAM`): 6 T-States (420 ms).
  * Indirekte Zeiger-Zugriffe (`LDAMM`, `STAMM`): 8 T-States (560 ms).

---

## 2. Speicherabbild (Memory Map)

Der 16-Bit-Adressraum (0x0000 bis 0xFFFF) ist in deterministische Zonen unterteilt:

| Adressbereich | Hex-Bereich | Funktion | Beschreibung |
|---|---|---|---|
| `0 .. 959` | `0x0000 - 0x03BF` | Allgemeines RAM & Variablen | Statischer Speicherbereich für Variablen, Compiler-Temporaries und Stack |
| `960 .. 1023` | `0x03C0 - 0x03FF` | LED-Display (VRAM) | 64 Wörter à 16 Bit für die 32×32 Matrix (1 Bit pro Pixel, bitmapped) |
| `1024 .. 65535` | `0x0400 - 0xFFFF` | Programm-ROM / Code | Maschinencode, Sprungtabellen und Compiler-Runtime |

### Display-Mapping (32×32 LED-Matrix)
* Die 32 Zeilen der Matrix werden über 64 Speicherwörter (jeweils 2 Wörter pro Zeile à 16 Bit = 32 Pixel) abgebildet:
  $$\text{Adresse}(Y, X) = 960 + (Y \times 2) + \lfloor X / 16 \rfloor$$
  $$\text{Bit}(X) = 15 - (X \bmod 16)$$

---

## 3. Arithmetik & Software-Runtime

Da die physische Relais-ALU ausschließlich über Hardware-Addition (`ADD`), Bit-Invertierung (`NOT`), logische Gatter (`AND`, `OR`) und 1-Bit-Shifts (`SHL`, `SHR`) verfügt, werden alle komplexeren mathematischen Operationen durch softwareseitige Runtime-Routinen realisiert:

1. **Subtraktion:** Realisiert über Zweierkomplement ($A - B = A + (\sim B + 1)$).
2. **Multiplikation (`__runtime_mul`):** 16-Bit Shift-and-Add-Algorithmus (Dauer ca. 16 Iterationen $\times 20$ T-States $\approx 22.4$ Sekunden bei 14 Hz).
3. **Division (`__runtime_div`):** Restwert-Restoring-Division über Shift-and-Subtract.
4. **Ganzzahlige Quadratwurzel (`__runtime_sqr`):**
   * Verwendet einen hardware-optimierten Bisection- bzw. Bit-Shift-Algorithmus (Digit-by-Digit).
   * Ermöglicht das Berechnen von Vektorlängen und Distanzen für Grafikdemos ohne Gleitkomma-Hardware.
5. **Fixed-Point Mathematik:**
   * Für komplexe Berechnungen wie die Mandelbrot-Menge (`mandelbrot_clack_v3.bas`) nutzt der Compiler ein festes Q8.8- oder Q4.12-Festkommaformat.
   * Multiplikationen im Festkommaformat erfordern einen abschließenden Rechtsshift um 8 bzw. 12 Bits zur Skalierungskorrektur.

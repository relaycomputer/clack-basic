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
   * Für komplexe Berechnungen wie die Mandelbrot-Menge (`mandelbrot_clack_v3.bas`) nutzt der Compiler ein festes Q8.8- oder Q4.12-Festkommaformat (bzw. Q10.5 bei 16-Bit-Überlaufschutz ohne 32-Bit-Zwischenregister).
   * Multiplikationen im Festkommaformat erfordern einen abschließenden Rechtsshift um 8 bzw. 12 Bits zur Skalierungskorrektur.

---

## 4. Architekturvergleich: Zuse-Style Floating-Point (`S1-E7-M8`) vs. `Q8.8` Festkomma (Argon 16-Bit @ 70 ms)

Auf der 16-Bit Relais-Architektur (Ein-Register-Akkumulator `ACCU`, 70 ms T-State-Periode $\approx 14.28\text{ Hz}$, nur 1-Bit-Shifts `SHL`/`SHR` à 4 T-States ohne Barrel-Shifter, dynamisches Operand-Patching via `STAM`) konkurrieren zwei Zahlendarstellungen in einem einzelnen 16-Bit-Registerwort:

### 4.1 Bit-Layout & Eigenschaften im 16-Bit-Register

| Eigenschaft | `Q8.8` Festkomma (2er-Komplement) | Zuse-Style Float `S1-E7-M8` (Halblogarithmisch) |
|---|---|---|
| **Bit-Aufteilung (16 Bit)** | `[Bit 15: Sign] [Bit 14..8: Int (7)] [Bit 7..0: Frac (8)]` | `[Bit 15: S (1)] [Bit 14..8: E (7, Bias 64)] [Bit 7..0: M (8 + Hidden 1)]` |
| **Effektive Mantissen-Präzision** | Skalenabhängig (0 bis 15 Bit; kollabiert bei $|x| \ll 1$) | Konstant **9 Bit** ($1.M_7\dots M_0$, ca. $2.7$ Dezimalstellen) |
| **Dynamikbereich** | $-128.0000$ bis $+127.9961$ ($\Delta = 2^{-8} \approx 0.0039$) | $\pm 2^{-64} \dots \pm 2^{+63}$ ($\approx \pm 5.4 \times 10^{-20} \dots \pm 9.2 \times 10^{18}$) |
| **ALU-Hardware-Synergie** | **Hoch:** Native 16-Bit `ADD`/`NOT`-Befehle arbeiten direkt auf dem Gesamtwort | **Gering:** Erfordert Maskierung (`AND`), 8-fache Einzel-Shifts (`8x SHR` = 32 Takte) zum Entpacken/Packen von `E` |
| **Überlaufrisiko bei `FMUL`** | **Kritisch:** $16\times 16\text{ Bit}$ im 16-Bit-Akku überläuft ab $A \cdot B \ge 1.0$ (daher in Praxis oft Reduktion auf Q10.5 nötig) | **Minimal:** $9\times 9\text{ Bit}$ Mantissen-Multiplikation benötigt max. 18 Bit bzw. 9 Iterationen mit direktem Shift-Right |

### 4.2 Tabellarische Gegenüberstellung der Taktzyklen (`FADD` / `FSUB` / `FMUL` / `FDIV`)

Grundlage sind die realen Argon/Clack-Befehlszyklen (`LDA`/`ADD`/`AND`/`OR`/`NOT`/`SHL`/`SHR`/`BA`/`BZ`/`BN`/`BE`/`CALL` = **4 T-States**, `RET` = **5 T-States**, `LDAM`/`STAM` = **6 T-States**, `1 T-State = 70 ms`):

| Operation | `Q8.8` Festkomma (T-States) | `Q8.8` Reale Laufzeit (@ 70 ms) | Zuse `S1-E7-M8` (T-States) | Zuse `S1-E7-M8` Reale Laufzeit (@ 70 ms) | Technischer Engpass / Dominanter Kostenfaktor |
|---|:---:|:---:|:---:|:---:|---|
| **`FADD`** ($A + B$) | **22** *(Reg-Patch)*<br>**28** *(inkl. `STAM`)* | **1.54 s**<br>*(1.96 s)* | **~458** *(Ø $\Delta E=3$)*<br>*(340 .. 720)* | **~32.06 s**<br>*(23.8 s .. 50.4 s)* | **Q8.8:** Direkter 16-Bit Hardware-`ADD` (`LDAM`+`STAM patch`+`LDAM`+`ADD`).<br>**S1-E7-M8:** Unpack (140 T) + Exponenten-Alignment ($\Delta E$ Shifts) + Norm + Pack (70 T). |
| **`FSUB`** ($A - B$) | **30** *(Reg-Patch)*<br>**36** *(inkl. `STAM`)* | **2.10 s**<br>*(2.52 s)* | **~466** *(Ø $\Delta E=3$)*<br>*(348 .. 728)* | **~32.62 s**<br>*(24.4 s .. 51.0 s)* | **Q8.8:** Zweierkomplement-Negation (`NOT` + `ADD 1` = +8 T-States ggü. `FADD`).<br>**S1-E7-M8:** Vorzeichenbit-Invertierung (`XOR 0x8000`) + `FADD`-Pipeline. |
| **`FMUL`** ($A \times B$) | **~860 – 1.115**<br>*(16-Bit `__mul` + `8x SHR`)* | **60.20 s – 78.05 s** | **~900**<br>*(9-Bit Mul + Exp-Add)* | **~63.00 s** | **S1-E7-M8 schlägt/egalisiert Q8.8:** Nur **9 Shift-and-Add-Iterationen** der Mantisse (~576 T) + einfache Exponenten-Addition ($E_A + E_B - 64$) statt 16-Bit-Schleife! |
| **`FDIV`** ($A / B$) | **~1.280** *(16-Bit Shift-Sub)*<br>**~10.330** *(akt. `__div` bei $Q=256$)* | **89.60 s**<br>*(723.10 s / 12.1 min)* | **~1.000**<br>*(9-Bit Shift-Sub + Exp-Sub)* | **~70.00 s** | **S1-E7-M8 klar überlegen:** Nur **9 Restoring-Division-Schritte** der 9-Bit-Mantisse (~684 T) + Exponenten-Subtraktion ($E_A - E_B + 64$) ohne 16-Bit-Vorabskalierung. |

### 4.3 Fazit für die Argon-Architektur
1. **Additions-/Subtraktions-dominierte Algorithmen (z. B. Lineare Algebra, Bresenham, Mandelbrot-Innenschleife mit Shifts):** `Q8.8` (bzw. `Q10.5`) ist bei `FADD`/`FSUB` um den **Faktor ~16× bis 20× schneller** (1.54 s vs. ~32 s), da kein Exponenten-Entpacken (`8x SHR`) und kein iteratives Mantissen-De-Alignment über den 1-Bit-Shifter nötig sind.
2. **Multiplikations-/Divisions-intensive & hochdynamische Berechnungen:** Das **Zuse-Format `S1-E7-M8`** amortisiert seinen Pack/Unpack-Overhead (~210 T-States) vollständig, da sich die teuren Shift-and-Add- bzw. Shift-and-Subtract-Schleifen von 16 auf **9 Iterationen** verkürzen und kein 16-Bit-Zwischenprodukt-Überlauf auftritt.

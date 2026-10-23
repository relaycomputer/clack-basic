# Konzept & Entscheidungsvorlage: Gleitkomma- & Festkomma-Arithmetik in Clack-BASIC

**Autoren / Entscheidungsträger:** Dipl.-Ing. (FH) Peter Prikasky & Ing. Christian Buchhas  
**Systemarchitektur:** 16-Bit Relaiscomputer (1024 Worte RAM, 19 Opcodes, 14.3 Hz / 70 ms pro Takt)  
**Status:** Konzept & Architektur-Entscheidungsvorlage (Noch keine Compiler-Änderung)

---

## 1. Ausgangslage & Hardware-Rahmenbedingungen

Unser 16-Bit Relaiscomputer besitzt spezifische architektonische Eigenschaften, die für die Wahl eines Fließkomma-Formats entscheidend sind:

1. **16-Bit Wortbreite & 1024 Worte Gesamtspeicher (`0x000`–`0x3FF`)**:
   - Abzüglich Boot-Jump (`0..1`), Hardware-I/O (`2..3`), Variablen und Framebuffer (`768..1023` bei 64×64 bzw. `960..1023` bei 32×32) stehen für Programmcode **und** mathematische Laufzeit-Subroutinen effektiv **ca. 730 bis 900 Worte RAM** zur Verfügung.
   - Jedes eingesparte Wort in den Laufzeit-Routinen (`__fadd`, `__fmul`, `__fdiv`) lässt mehr Platz für das eigentliche BASIC-Programm.
2. **Kein Hardware-Barrel-Shifter**:
   - Die Befehle `SHR` (`0x3580`) und `SHL` (`0x35C0`) schieben den Akkumulator pro Befehl um **exakt 1 Bit**.
   - Ein dynamischer Shift um $N$ Bits (wie er beim Exponenten-Angleich einer Gleitkomma-Addition nötig ist) erfordert eine Schleife im RAM (ca. 6 Instruktionen = 28 Takte *pro geschobenem Bit*).
3. **1-Level Hardware-Return-Register (`SR`)**:
   - Da `CALL` (`0x5000`) die Rücksprungadresse in ein einzelnes Register schreibt, dürfen sich Laufzeit-Routinen nicht gegenseitig per `CALL` verschachteln (oder müssen ihre Rücksprungadresse im RAM sichern).
4. **Reale Relais-Schaltzeit (70 ms / Takt)**:
   - Eine Operation mit 200 Instruktionen (~900 Takten) dauert auf der echten Hardware rund **63 Sekunden**. Effizienz entscheidet also direkt über die Praxistauglichkeit im Live-Betrieb.

---

## 2. Bewertung der Kandidaten-Formate

Auf Basis der übermittelten Voranalyse ergeben sich drei realistische Kandidaten für Clack-BASIC:

### Kandidat A: Festkomma `Q8.8` (Signed Fixed-Point)

```
Bit:  15  14 ............ 8   7 ............. 0
    +---+-------------------+-------------------+
    | S | Vorkomma (7 Bit)  | Nachkomma (8 Bit) |
    +---+-------------------+-------------------+
    Wert = Integerwert / 256
```

- **Wertebereich:** `-128.000` bis `+127.996`
- **Auflösung (Genauigkeit):** $\frac{1}{256} = 0.00390625$ (ca. 2,4 Dezimalstellen)
- **Darstellung im RAM:** Normales Zweierkomplement, skaliert mit Faktor $256$ (`1.0` = `0x0100`, `0.5` = `0x0080`, `-2.25` = `0xFDC0`).

#### Rechenoperationen in `Q8.8`:
- **Addition & Subtraktion (`+`, `-`):**
  - **100% identisch zur normalen 16-Bit Integer-Addition!**
  - Kein Unterprogramm nötig! Direkt inline **2 Instruktionen** (`LDAM`, `ADD`).
  - **Dauer auf Relais-Hardware:** ~0,7 Sekunden (10 Takte).
- **Multiplikation (`*`):**
  - Mathematisch: $\text{Ergebnis} = \frac{A \times B}{256} = (A \times B) \gg 8$.
  - Umsetzung über eine angepasste Shift-Add-Routine (`__fmul_q88`), die Vorzeichen behandelt und das Zwischenergebnis um 8 Bit schiebt.
  - **Umfang:** ca. **45 Worte RAM**, Laufzeit ~80–120 Instruktionen (~30s auf Echt-Hardware).
- **Division (`/`):**
  - Mathematisch: $\text{Ergebnis} = \frac{A \times 256}{B} = (A \ll 8) / B$.
  - Nutzt das Prinzip unserer bestehenden Restoring-Division (`__fdiv_q88`).
  - **Umfang:** ca. **50 Worte RAM**, Laufzeit ~130 Instruktionen (~40s auf Echt-Hardware).

---

### Kandidat B: „Zuse-Style“ Relay-Float16 (`S1-E7-M8` ohne IEEE-Ballast)

Wie bei Konrad Zuses Z1/Z3 wird ein reines, schnörkelloses Binär-Gleitkommaformat ohne `NaN`, ohne `Infinity`, ohne `Denormals` und ohne komplexe Rundungsmodi genutzt.

```
Bit:  15  14 ........... 8   7 .............. 0
    +---+------------------+--------------------+
    | S | Exponent (7 Bit) |  Mantisse (8 Bit)  |
    +---+------------------+--------------------+
    Wert = (-1)^S * (1 + Mantisse / 256) * 2^(Exponent - 64)
    Sonderfall: 0x0000 = 0.0
```

- **Wertebereich:** $\approx \pm 5.4 \times 10^{-20}$ bis $\pm 1.8 \times 10^{+19}$ (enorme wissenschaftliche Dynamik!)
- **Genauigkeit:** 8 Bit gespeicherte Mantisse + 1 implizites Leading-Bit = **9 Bit Präzision** ($\approx 2.7$ signifikante Dezimalstellen).
- **Architektur-Vorteil (Byte-Alignment):**
  - Der Exponent liegt exakt im oberen Byte (`Bits 14..8`), die Mantisse exakt im unteren Byte (`Bits 7..0`, Maske `0x00FF`, implizite Eins bei `0x0100`).

#### Rechenoperationen in „Zuse-Style“ Float16:
1. **Multiplikation (`FMUL`):**
   - Überraschend kompakt!
   - Vorzeichen = `S1 XOR S2`
   - Exponent = `E1 + E2 - 64` (einfache Integer-Addition!)
   - Mantisse = `(M1 * M2) >> 8` (9×9 Bit Multiplikation in ein 16-Bit Wort – **kein 32-Bit Überlauf möglich**, falls man jeweils um 1 Bit vorab skaliert oder 8×8 rechnet!).
   - Falls Bit 9 gesetzt ist: 1× Rechtsshift (`SHR`) und Exponent `+1`.
   - **Umfang:** ca. **65 Worte RAM**, ~100 Instruktionen.
2. **Division (`FDIV`):**
   - Vorzeichen = `S1 XOR S2`
   - Exponent = `E1 - E2 + 64`
   - Mantisse = `(M1 << 7) / M2` (passt direkt in 16-Bit!).
   - **Umfang:** ca. **70 Worte RAM**, ~130 Instruktionen.
3. **Addition & Subtraktion (`FADD`, `FSUB`):**
   - **Die aufwendigste Operation bei echtem Gleitkomma:**
     1. Entpacken von `S1, E1, M1` und `S2, E2, M2` (mit impliziter Eins `0x0100`).
     2. Exponenten vergleichen (`dE = E1 - E2`). Falls `dE < 0`, Operanden tauschen.
     3. Falls `dE > 8`: Der kleinere Operand fällt komplett aus der Genauigkeit -> Ergebnis ist direkt Operand 1!
     4. Mantisse des kleineren Operanden in einer Schleife `dE`-mal mit `SHR` nach rechts schieben.
     5. Je nach Vorzeichen `M1 + M2` oder `M1 - M2` berechnen.
     6. **Normalisieren:**
        - Bei Überlauf (`>= 0x0200`): `SHR` und `E++`.
        - Bei Auslöschung (`< 0x0100`): Schleife mit `SHL` und `E--`, bis Bit 8 (`0x0100`) wieder `1` ist (oder Ergebnis `0` wird).
     7. Wieder zusammenpacken in `S | (E << 8) | (M & 0xFF)`.
   - **Umfang:** ca. **130–150 Worte RAM**, ~180–240 Instruktionen pro Addition.

---

### Kandidat C: IEEE 754 `binary16` (Tiny Half-Precision `S1-E5-M10`)

```
Bit:  15  14 ...... 10   9 .................. 0
    +---+--------------+------------------------+
    | S | Exp (5 Bit)  |   Mantisse (10 Bit)    |
    +---+--------------+------------------------+
    Wert = (-1)^S * (1 + Mantisse / 1024) * 2^(Exponent - 15)
```

- **Wertebereich:** $\approx \pm 6.1 \times 10^{-5}$ bis $\pm 65504$
- **Genauigkeit:** 10 Bit + 1 implizites Bit = **11 Bit Präzision** ($\approx 3.3$ Dezimalstellen).
- **Herausforderung auf unserer 16-Bit-CPU:**
  - Bei der Multiplikation zweier 11-Bit Mantissen (`0x0400`..`0x07FF`) entsteht ein **22-Bit Ergebnis**, das **nicht mehr in ein 16-Bit Register passt**!
  - Man benötigt daher zwingend eine **32-Bit Doppelwort-Arithmetik im RAM** (Low-Word + High-Word mit Carry-Übertrag via `BC`) oder muss vor der Multiplikation 3 Bits Genauigkeit wegwerfen (wodurch der Vorteil gegenüber Kandidat B verloren geht).
  - Dadurch steigt der RAM-Verbrauch für `FMUL` und `FDIV` auf ca. **90–110 Worte** je Routine.

---

## 3. Direkter Vergleich auf unserer Relais-Maschine

| Kriterium | Option A: `Q8.8` Festkomma | Option B: „Zuse-Float16“ (`S1-E7-M8`) | Option C: IEEE 754 `binary16` (`S1-E5-M10`) |
| :--- | :--- | :--- | :--- |
| **Wertebereich** | `-128.0` .. `+127.996` | $\pm 10^{-19}$ .. $\pm 10^{+19}$ | $\pm 6 \cdot 10^{-5}$ .. $\pm 65504$ |
| **Präzision** | 8 Bit Nachkomma (`0.0039`) | 9 Bit signifikant (~2.7 Stellen) | 11 Bit signifikant (~3.3 Stellen) |
| **Passt Mantissen-Mul in 16-Bit?** | ✅ Ja (mit Shift) | ✅ **Ja! (8/9 Bit passt in 16 Bit)** | ❌ Nein (22 Bit -> 32-Bit Mathe nötig) |
| **RAM-Bedarf `FADD`/`FSUB`** | **0 Worte** (2 Befehle inline!) | ~140 Worte Subroutine | ~160 Worte Subroutine |
| **RAM-Bedarf `FMUL`** | ~45 Worte Subroutine | ~65 Worte Subroutine | ~100 Worte Subroutine (32-Bit) |
| **RAM-Bedarf `FDIV`** | ~50 Worte Subroutine | ~70 Worte Subroutine | ~110 Worte Subroutine (32-Bit) |
| **Gesamt-RAM (alle 4 Ops)** | **~95 Worte (9% RAM)** | **~275 Worte (27% RAM)** | **~370 Worte (36% RAM)** |
| **Dauer 1× Addition (14.3 Hz)** | **~0,7 Sek.** (10 Takte) | **~35 Sek.** (~500 Takte) | **~45 Sek.** (~650 Takte) |
| **Dauer 1× Multiplikation** | **~25 Sek.** (~360 Takte) | **~22 Sek.** (~320 Takte) | **~45 Sek.** (~650 Takte) |
| **Historischer Bezug** | Klassische Minicomputer / Demos | **Konrad Zuse Z1 / Z3 Philosophie** | Moderner GPU/CPU-Standard |

> [!IMPORTANT]
> **Wichtiger Hinweis zum Speicherverbrauch (Lazy Emission):**  
> Genau wie heute schon bei `__mul`, `__div` und `__sqr` würde der Compiler die Float-Subroutinen **nur dann** in den Maschinencode einbinden, wenn sie im jeweiligen BASIC-Programm auch tatsächlich aufgerufen werden! Bestehende Programme verbrauchen also **0 Worte zusätzlichen Speicher**.

---

## 4. Vorschlag für die BASIC-Syntax

Wie soll man Kommazahlen in Clack-BASIC schreiben? Es gibt zwei elegante Möglichkeiten:

### Syntax-Variante 1: Explizite Funktionen / Operatoren (Klar getrennt, sehr sicher)
Normale Variablen (`A`, `X`) bleiben 16-Bit-Worte. Kommazahlen-Literale wie `3.14` oder `0.5` werden vom Compiler automatisch ins Binärformat umgerechnet:
```basic
REM Kommazahlen-Literale werden beim Kompilieren direkt ins 16-Bit-Format übersetzt
LET A = 1.5
LET B = 2.25

REM Explizite Float-Operationen:
LET C = FADD(A, B)
LET D = FSUB(A, B)
LET E = FMUL(A, B)
LET F = FDIV(A, B)
```
*Vorteil:* Man sieht im Code sofort, wo teure Float-Subroutinen aufgerufen werden, und kann Integer-Schleifen (`FOR I = 0 TO 31`) problemlos mit Float-Berechnungen mischen.

### Syntax-Variante 2: Typisierte Float-Variablen mit `#`-Suffix (Klassisches BASIC)
Variablen mit `#` am Ende (wie `X#`, `Y#`, `RADIUS#`) sind Kommazahlen. Bei normalen Operatoren (`+`, `-`, `*`, `/`) wählt der Compiler automatisch die Float-Routinen:
```basic
LET X# = 1.5
LET Y# = 0.25
LET Z# = (X# + Y#) * 2.0
LET Q# = X# / Y#
```
*Vorteil:* Liest sich extrem natürlich wie echtes historisches Microsoft/Homecomputer-BASIC!

---

## 5. Unsere Empfehlung & Entscheidungsfragen an Peter & Christian

### Unsere technische Empfehlung:
1. **Format:** **Kandidat B („Zuse-Style“ `S1-E7-M8`)** ODER **Kandidat A (`Q8.8`)** (oder beide umschaltbar per `OPTION FLOAT ZUSE` / `OPTION FLOAT Q8.8`):
   - Warum **Zuse-Float16 (`S1-E7-M8`)** statt IEEE 754 `binary16`?  
     Weil bei `S1-E7-M8` die Mantisse (8 Bit) bei Multiplikation und Division **ohne 32-Bit-Überlauf in unsere 16-Bit-Register passt** und das Format perfekt auf Byte-Grenzen liegt. Das spart über 100 Worte kostbaren RAM und halbiert die Laufzeit im Vergleich zu IEEE 754 – bei echtem Zuse-Flair!
   - Warum zusätzlich **`Q8.8`**?  
     Weil in `Q8.8` eine Addition/Subtraktion nur **2 Maschinenbefehle (0,7 Sekunden)** statt 200 Befehle (35 Sekunden) kostet. Für Grafik, Sinus/Cosinus und Fraktale ist `Q8.8` unschlagbar schnell.

### Die 3 Entscheidungen für euch:
1. **Welches Zahlenformat wollen wir bauen?**
   - [ ] **Option 1:** Echtes Gleitkomma im **Zuse-Stil (`S1-E7-M8`)** (optimal für 16-Bit-Register, riesiger Wertebereich)
   - [ ] **Option 2:** Echtes **IEEE 754 `binary16` (`S1-E5-M10`)** (Standard, braucht aber 32-Bit-Hilfsarithmetik im RAM)
   - [ ] **Option 3:** Schnelles **Festkomma `Q8.8`** (`-128.0` bis `+127.996`, Addition in 1 Takt)
   - [ ] **Option 4:** **Kombi-Lösung**: Standardmäßig Zuse-Float16 (oder Q8.8), umschaltbar im Kopf des Programms via `OPTION FLOAT Q8.8` / `OPTION FLOAT ZUSE16`!
2. **Welche Syntax gefällt euch im BASIC besser?**
   - [ ] **Syntax A:** Natürliche Schreibweise mit `#`-Suffix (`LET Z# = X# + 1.5`, `LET P# = X# * Y#`)
   - [ ] **Syntax B:** Explizite Funktionsaufrufe (`FADD(A, B)`, `FMUL(A, B)`, `FDIV(A, B)`)
   - [ ] **Syntax C:** Beides unterstützen (`#`-Variablen für `+ - * /` und zusätzlich `FADD/FMUL/...`)
3. **Ausgabe / Anzeige:**
   - Sollen Kommazahlen im Simulator-Speicherinspektor / Watch-Fenster automatisch auch als Dezimalwert (z. B. `0x4040 (3.0)`) angezeigt werden?

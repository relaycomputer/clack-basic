# ⚡ Clack-BASIC Sprach- & Grafik-Referenz
**16-Bit Relais-Computer: Grafikbefehle, Pixel-Animation & Ablaufsteuerung**

Dieses Dokument beschreibt den erweiterten Sprachumfang von **Clack-BASIC** für den 16-Bit Relaisrechner, insbesondere für **Grafik-Manipulation (LED-Matrix 64×64 / 32×32)**, **Punkt-Animationen** und **strukturierte Unterprogramme (`GOSUB` / `RETURN`)**.

---

## 🎨 1. Grafikbefehle für die LED-Matrix

Der Framebuffer der LED-Matrix liegt direkt im Relais-RAM:
- **32×32 Matrix (4x Hardware-Module):** RAM-Adressen `0x03C0` (960) bis `0x03FF` (1023) — 64 Worte.
- **64×64 Matrix (Full Panel):** RAM-Adressen `0x0300` (768) bis `0x03FF` (1023) — 256 Worte.

### `CLS` — Matrix löschen
Löscht das gesamte Display oder einen Zeilenbereich in $<1\,\text{ms}$.
```basic
CLS                 ' Löscht das gesamte Display
CLS 0, 15           ' Löscht nur die oberen 16 Zeilen
```

---

### `PLOT X, Y [, Color]` & `PSET X, Y, Color` — Pixel setzen / löschen
Setzt oder löscht einen einzelnen Bildschirmpunkt.
- **`X`**: Horizontale Koordinate (0..63 bzw. 0..31).
- **`Y`**: Vertikale Koordinate (0..63 bzw. 0..31).
- **`Color`** *(optional bei `PLOT`)*:
  - `1` (oder weggelassen) = Pixel einschalten (aktuelle Palettenfarbe).
  - `0` = Pixel ausschalten (Schwarz).
  - `2..15` = Farbindex bei 4-Bit RGB-Modus (z. B. Rot, Grün, Cyan, Gold).

```basic
' Einfacher Plot (Default: Farbe 1 / An)
PLOT 16, 16

' Plot mit Farbangabe (0 = Löschen, 1 = Setzen)
PLOT X, Y, 1
PLOT X, Y, 0

' PSET Syntax
PSET X, Y, 0xFFFF
```

---

### `PIXELMOVE` — Intelligente Punkt-Verschiebung (Animation)
`PIXELMOVE` ist ein spezialisierter Befehl für Animationen (z. B. Bouncing Ball, Cursor, Partikel). Er führt das **Löschen des alten Punktes** und das **Setzen des neuen Punktes atomar in einem Schritt** aus, wodurch Bildschirmflackern verhindert wird!

#### Variante A: Absolute Verschiebung (`TO`)
```basic
' Verschiebt Punkt von (X1, Y1) nach (X2, Y2)
PIXELMOVE X_ALT, Y_ALT TO X_NEU, Y_NEU [, Color]
```

#### Variante B: Relative Verschiebung (`BY` / Delta)
```basic
' Verschiebt Punkt um DX nach rechts/links und DY nach oben/unten
PIXELMOVE X, Y BY DX, DY [, Color]
```

**Wie `PIXELMOVE` unter der Haube funktioniert:**
1. Prüft, ob sich die Koordinaten überhaupt geändert haben.
2. Wenn ja: Ruft `__pset(X_alt, Y_alt, 0)` auf (alter Punkt wird gelöscht).
3. Ruft `__pset(X_neu, Y_neu, Color)` auf (neuer Punkt wird gezeichnet).
4. Aktualisiert die Variablen `X = X_neu` und `Y = Y_neu` automatisch!

---

## 🔁 2. Unterprogramme: `GOSUB` und `RETURN`

**Ja, `GOSUB` und `RETURN` werden zu 100 % in Hardware unterstützt!**

Der Relaisrechner verfügt über ein dediziertes **Hardware-Stack-Register**:
- **`GOSUB <Target>`**: 
  - Übersetzt in den Maschinencode-Befehl **`CALL Target`** (`0x5000`).
  - Speichert die Rücksprungadresse ($PC + 2$) automatisch im Stack-Register und springt zur Subroutine.
- **`RETURN`**:
  - Übersetzt in den Maschinencode-Befehl **`RET`** (`0x6000`).
  - Lädt die gesicherte Adresse aus dem Stack-Register zurück in den $PC$.

### Beispiel: Unterprogramm-Aufruf
```basic
' ============================================================
' Hauptprogramm
' ============================================================
CLS

FOR I = 1 TO 5
    LET X = I * 4
    LET Y = 10
    GOSUB ZEICHNE_KREUZ
NEXT I

END

' ============================================================
' Unterprogramm: Zeichnet ein kleines 3x3 Kreuz an (X, Y)
' ============================================================
ZEICHNE_KREUZ:
    PLOT X, Y, 1
    PLOT X - 1, Y, 1
    PLOT X + 1, Y, 1
    PLOT X, Y - 1, 1
    PLOT X, Y + 1, 1
    RETURN
```

---

## 🎮 3. Vollständiges Demo-Programm: Bouncing Ball

Dieses Programm demonstriert die Kombination aus `CLS`, `PIXELMOVE`, Schleifen und `GOSUB`:

```basic
' ============================================================
' 🏀 16-Bit Relais Bouncing Ball Demo
' ============================================================
CLS

' Startposition & Geschwindigkeit
LET X = 5
LET Y = 5
LET DX = 1
LET DY = 1

' Hauptschleife
MAIN_LOOP:
    ' 1. Physik berechnen (Kollision mit Display-Raendern 0..31)
    GOSUB UPDATE_PHYSICS

    ' 2. Punkt verschieben (alter Punkt loeschen, neuer setzen)
    PIXELMOVE X - DX, Y - DY TO X, Y, 1

    ' 3. Endlosschleife fortsetzen
    GOTO MAIN_LOOP

' ============================================================
' Unterprogramm: Kollisionsabfrage
' ============================================================
UPDATE_PHYSICS:
    LET X = X + DX
    LET Y = Y + DY

    ' X-Rand pruefen (0..31)
    IF X >= 31 THEN LET DX = -1
    IF X <= 0 THEN LET DX = 1

    ' Y-Rand pruefen (0..31)
    IF Y >= 31 THEN LET DY = -1
    IF Y <= 0 THEN LET DY = 1

    RETURN
```

---

## 📋 Befehls-Übersicht (Quick-Cheat-Sheet)

| Befehl | Syntax | Hardware-Opcode | Beschreibung |
| :--- | :--- | :---: | :--- |
| **`PLOT`** | `PLOT X, Y [, C]` | `CALL __pset` | Punkt auf Matrix setzen / löschen ($C=1$ an, $C=0$ aus) |
| **`PSET`** | `PSET X, Y, C` | `CALL __pset` | Alias zu PLOT mit expliziter Farbangabe |
| **`PIXELMOVE`** | `PIXELMOVE X1, Y1 TO X2, Y2` | $2	imes$ `CALL __pset` | Punkt von $(X_1, Y_1)$ nach $(X_2, Y_2)$ flimmerfrei bewegen |
| **`CLS`** | `CLS [StartRow, EndRow]` | `CALL __cls` | Löscht den Framebuffer im RAM |
| **`GOSUB`** | `GOSUB Label` / `GOSUB 500` | `CALL target` (`0x5000`) | Unterprogrammaufruf (Stack Push) |
| **`RETURN`** | `RETURN` | `RET` (`0x6000`) | Rücksprung aus Unterprogramm (Stack Pop) |
| **`FOR..NEXT`**| `FOR I = 1 TO 10 [STEP 2]` | Loop-Code | Zählschleife mit automatischer Register-Verwaltung |
| **`IF..THEN`** | `IF A > B THEN ... [ELSE]` | `BN / BZ` | Verzweigung anhand der ALU-Flags ($Z, N, C, E$) |
| **`POKE`** | `POKE Addr, Val` | `STAM` | Direktes Schreiben in 16-Bit RAM-Zelle |
| **`PEEK`** | `PEEK(Addr)` | `LDAM` | Direktes Lesen aus 16-Bit RAM-Zelle |

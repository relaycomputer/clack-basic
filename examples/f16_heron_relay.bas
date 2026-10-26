REM ============================================================
REM ⚡ Clack-BASIC Aufgabe 1: Heron-Wurzel √2 in CLACK-F16
REM Echtes 16-Bit Gleitkomma (S1-E7-M8) für reale Relais-Hardware
REM Laufzeit auf echtem Relaissystem (14.3 Hz): ca. 3.5 Minuten
REM ============================================================
REM Berechnet √2 ≈ 1.4140625 (CLACK-F16: 0x3F6A) in 2 Heron-Schritten:
REM   x(n+1) = 0.5 * (x(n) + 2.0 / x(n))
REM   Startwert: x0 = 1.5     (0x3F80 -> Mantisse 128)
REM   Schritt 1: x1 = 1.41797 (0x3F6B -> Mantisse 107)
REM   Schritt 2: x2 = 1.41406 (0x3F6A -> Mantisse 106, Fixpunkt!)
REM
REM Live-Anzeige:
REM   - Hardware OUT-Port (Adresse 2): Low-Byte der F16-Mantisse
REM   - 32x32 LED-Matrix: Konvergenzkurve der Näherungswerte
REM ============================================================

DIM S#, X#, H# AS FLOAT16

LET S# = 2.0
LET X# = 1.5
LET H# = 0.5

REM Startwert (1.5 -> Mantisse 0x80 = 128) ausgeben und plotten
POKE 2, X# AND 255
PLOT 8, 8, 1

FOR I = 1 TO 2
    REM Heron-Formel in echtem CLACK-F16 Gleitkomma:
    LET X# = H# * (X# + S# / X#)

    REM Mantissen-Byte (Bits 7..0) extrahieren und auf Port 2 ausgeben
    LET M = X# AND 255
    POKE 2, M

    REM Konvergenzpunkt auf der 32x32 Matrix plotten
    REM Y-Achse skaliert aus der Mantisse (128 -> Y=8, 107 -> Y=29, 106 -> Y=30)
    PLOT 8 + (I SHL 3), 136 - M, 1
NEXT I

HALT

REM ============================================================
REM 🌌 Clack-BASIC Aufgabe 2: Q8.8 gedämpfter Lissajous-Harmonograph
REM Komplexe 2D-Festkomma-Differentialgleichung & Bresenham-Vektoren
REM ============================================================
REM Simuliert ein gekoppeltes mechanisches Doppelpendel (Harmonograph)
REM mit echter Q8.8 Festkomma-Arithmetik (Auflösung 1/256 = 0.0039):
REM
REM   X-Oszillator (ω_x ≈ 0.35): VX = VX - X * 0.125 - VX * 0.0078125
REM                              X  = X + VX
REM   Y-Oszillator (ω_y = 0.25): VY = VY - Y * 0.0625 - VY * 0.0078125
REM                              Y  = Y + VY
REM
REM Beide Amplituden bleiben innerhalb von ±13.0 Pixeln um das Zentrum
REM (16, 16) und zeichnen eine kontinuierliche Lissajous-Spirale per
REM Bresenham-Vektorlinien (LINE) auf die 32x32 LED-Matrix.
REM ============================================================

DIM X, Y, VX, VY AS FIXED8

REM Startauslenkung (X=13, Y=0) und Startgeschwindigkeit (VX=0, VY=3.25)
REM Bei KY = 0.0625 (ω_y = 0.25) ergibt VY = 3.25 exakt Amplitude A_y = 13.0!
LET X = 13.0
LET Y = 0.0
LET VX = 0.0
LET VY = 3.25

REM Startpunkt am rechten Rand (16 + 13 = 29, 16)
LET PX = 29
LET PY = 16

FOR I = 1 TO 90
    REM Symplektischer Euler-Integrationsschritt in Q8.8 Festkomma
    LET VX = VX - (X * 0.125) - (VX * 0.0078125)
    LET X = X + VX

    LET VY = VY - (Y * 0.0625) - (VY * 0.0078125)
    LET Y = Y + VY

    REM Umrechnung auf 32x32 Display-Koordinaten (Zentrum 16, 16)
    LET NX = 16 + FTOI(X)
    LET NY = 16 + FTOI(Y)

    REM Verbinde alten und neuen Phasenraumpunkt per Bresenham-Linie
    LINE PX, PY TO NX, NY, 1

    LET PX = NX
    LET PY = NY

    REM Live-Telemetrie auf Hardware Port 2 (aktuelle X-Koordinate)
    POKE 2, NX
NEXT I

HALT

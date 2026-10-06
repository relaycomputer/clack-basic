REM ============================================================
REM ⚡ Clack-BASIC: LED Matrix Grafik-Demo (32x32 / 64x64)
REM Demonstrating CLS, LINE (Bresenham), PLOT, and PIXELMOVE
REM ============================================================

REM 1. Display leeren
CLS

REM 2. Rahmen zeichnen mit LINE
LINE 0, 0 TO 31, 0, 1
LINE 31, 0 TO 31, 31, 1
LINE 31, 31 TO 0, 31, 1
LINE 0, 31 TO 0, 0, 1

REM 3. Diagonales Kreuz zeichnen
LINE 0, 0 TO 31, 31, 1
LINE 0, 31 TO 31, 0, 1

REM 4. Einzelne Punkte setzen mit PLOT
PLOT 16, 8, 1
PLOT 16, 24, 1

REM 5. Punkt verschieben mit PIXELMOVE
PIXELMOVE 16, 8 TO 16, 9, 1

END

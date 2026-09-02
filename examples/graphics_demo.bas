REM ============================================================
REM ⚡ Clack-BASIC: 4-Display LED Matrix Grafik-Demo (32x32)
REM ============================================================

REM 1. Display leeren (Worte 960..1023)
CLS

REM 2. Diagonale Linie zeichnen
FOR I = 0 TO 15 STEP 1
    PSET I, I, 0xFFFF
NEXT I

REM 3. Horizontale Linie bei Y = 8
FOR X = 0 TO 31 STEP 1
    PSET X, 8, 0xAAAA
NEXT X

HALT

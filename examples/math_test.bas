REM ============================================================
REM ⚡ Clack-BASIC: Mathematik- & Logiktest
REM ============================================================

LET A = 25
LET B = 14

REM 1. Addition & Subtraktion
LET C = A + B
LET D = A - B

REM 2. Multiplikation (16-Bit Shift & Add Subroutine)
LET P = 12 * 7

REM 3. Division & Modulo (16-Bit Restoring Division)
LET Q = 100 / 8
LET R = 100 MOD 8

REM 4. Verzweigung
IF P > 80 THEN
    LET STATUS = 1
ELSE
    LET STATUS = 0
END IF

HALT

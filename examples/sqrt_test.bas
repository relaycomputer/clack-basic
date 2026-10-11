REM ============================================================
REM ⚡ Clack-BASIC: Hardwarenahe Wurzelfunktion (SQR / SQRT)
REM ============================================================
REM 1. Ganzzahlige Wurzeln (8-Schritt Bit-by-Bit ohne Division)
LET R1 = SQR(64)       REM Ergibt 8
LET R2 = SQR(100)      REM Ergibt 10
LET R3 = SQRT(25)      REM Ergibt 5 (SQRT als Alias)

REM 2. Festkomma-Wurzeln mit Dezimalstellen (Ziffernextraktion)
LET W2_1 = SQR(2, 1)   REM Ergibt 14  (entspricht 1.4)
LET W2_2 = SQR(2, 2)   REM Ergibt 141 (entspricht 1.41)
LET W3_2 = SQR(3, 2)   REM Ergibt 173 (entspricht 1.73)
LET W10_2 = SQR(10, 2) REM Ergibt 316 (entspricht 3.16)

HALT

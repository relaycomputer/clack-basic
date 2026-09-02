REM ============================================================
REM 🏓 Clack-BASIC: Autonomes Ping-Pong (Self-Playing AI Pong)
REM Laeuft in Echtzeit auf der 32x32 LED-Matrix (Worte 960..1023)
REM ============================================================

REM 1. Initialisierung
CLS

LET BX = 16
LET BY = 16
LET VX = 1
LET VY = 1

LET LY = 14
LET RY = 14

GAME_LOOP:
    REM --- Display leeren fuer neuen Frame ---
    CLS
    
    REM --- 1. Linkes Paddle zeichnen (X=1, Hoehe 4) ---
    PSET 1, LY, 0xFFFF
    PSET 1, LY + 1, 0xFFFF
    PSET 1, LY + 2, 0xFFFF
    PSET 1, LY + 3, 0xFFFF
    
    REM --- 2. Rechtes Paddle zeichnen (X=30, Hoehe 4) ---
    PSET 30, RY, 0xFFFF
    PSET 30, RY + 1, 0xFFFF
    PSET 30, RY + 2, 0xFFFF
    PSET 30, RY + 3, 0xFFFF
    
    REM --- 3. Ball zeichnen (BX, BY) ---
    PSET BX, BY, 0xFFFF
    
    REM --- 4. Ball bewegen ---
    LET BX = BX + VX
    LET BY = BY + VY
    
    REM --- 5. Bandenkollision oben / unten ---
    IF BY <= 1 THEN
        LET VY = 1
    END IF
    IF BY >= 30 THEN
        LET VY = -1
    END IF
    
    REM --- 6. Paddle-Kollision Links (X=2) ---
    IF BX <= 2 THEN
        LET VX = 1
    END IF
    
    REM --- 7. Paddle-Kollision Rechts (X=29) ---
    IF BX >= 29 THEN
        LET VX = -1
    END IF
    
    REM --- 8. Autonome KI: Linkes Paddle folgt Ball ---
    IF BY > LY + 1 THEN
        IF LY < 27 THEN
            LET LY = LY + 1
        END IF
    END IF
    IF BY < LY + 1 THEN
        IF LY > 1 THEN
            LET LY = LY - 1
        END IF
    END IF
    
    REM --- 9. Autonome KI: Rechtes Paddle folgt Ball ---
    IF BY > RY + 1 THEN
        IF RY < 27 THEN
            LET RY = RY + 1
        END IF
    END IF
    IF BY < RY + 1 THEN
        IF RY > 1 THEN
            LET RY = RY - 1
        END IF
    END IF
    
    GOTO GAME_LOOP

REM ============================================================
REM 🏓 Clack-BASIC: Ultra-Fast Autonomes Ping-Pong
REM ============================================================

REM 1. Initialisierung: Einmalig Bildschirm leeren
CLS

REM Ball-Koordinaten (Startmitte)
LET BX = 16
LET BY = 16
LET OLD_BX = 16
LET OLD_BY = 16
LET VX = 1
LET VY = 1

REM Paddle-Positionen (Hoehe 4 Pixel)
LET LY = 14
LET RY = 14
LET OLD_LY = 14
LET OLD_RY = 14

GAME_LOOP:
    REM --- Alte Positionen gezielt löschen (Zero Flicker & Turbo-Speed!) ---
    PSET 1, OLD_LY, 0
    PSET 1, OLD_LY + 1, 0
    PSET 1, OLD_LY + 2, 0
    PSET 1, OLD_LY + 3, 0
    
    PSET 30, OLD_RY, 0
    PSET 30, OLD_RY + 1, 0
    PSET 30, OLD_RY + 2, 0
    PSET 30, OLD_RY + 3, 0
    
    PSET OLD_BX, OLD_BY, 0
    
    REM --- Neue Positionen zeichnen ---
    PSET 1, LY, 1
    PSET 1, LY + 1, 1
    PSET 1, LY + 2, 1
    PSET 1, LY + 3, 1
    
    PSET 30, RY, 1
    PSET 30, RY + 1, 1
    PSET 30, RY + 2, 1
    PSET 30, RY + 3, 1
    
    PSET BX, BY, 1
    
    REM --- Alte Positionen merken ---
    LET OLD_LY = LY
    LET OLD_RY = RY
    LET OLD_BX = BX
    LET OLD_BY = BY
    
    REM --- Ball bewegen ---
    LET BX = BX + VX
    LET BY = BY + VY
    
    REM --- Bandenkollision oben / unten ---
    IF BY <= 1 THEN
        LET VY = 1
    END IF
    IF BY >= 30 THEN
        LET VY = -1
    END IF
    
    REM --- Paddle-Kollision Links (X=2) ---
    IF BX <= 2 THEN
        LET VX = 1
    END IF
    
    REM --- Paddle-Kollision Rechts (X=29) ---
    IF BX >= 29 THEN
        LET VX = -1
    END IF
    
    REM --- Autonome KI: Linkes Paddle folgt Ball ---
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
    
    REM --- Autonome KI: Rechtes Paddle folgt Ball ---
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

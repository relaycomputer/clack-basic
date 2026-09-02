# 🏓 Clack-BASIC Code-Beispiele

Hier findest du vollständige, getestete Beispielprogramme für den 16-Bit Relaisrechner.

---

## 1. 🏓 Autonomes Ping-Pong (`ping_pong.bas`)

Ein vollständiges, selbstspielendes Pong-Spiel mit zwei computergesteuerten Paddles und Ball-Physik auf der 32×32 LED-Matrix:

```basic
REM ============================================================
REM 🏓 Clack-BASIC: Autonomes Ping-Pong (Self-Playing AI Pong)
REM Laeuft in Echtzeit auf der 32x32 LED-Matrix (Worte 960..1023)
REM ============================================================

CLS

LET BX = 16
LET BY = 16
LET VX = 1
LET VY = 1

LET LY = 14
LET RY = 14

GAME_LOOP:
    REM --- Frame leeren ---
    CLS
    
    REM --- 1. Linkes Paddle (X=1, Hoehe 4) ---
    PSET 1, LY, 0xFFFF
    PSET 1, LY + 1, 0xFFFF
    PSET 1, LY + 2, 0xFFFF
    PSET 1, LY + 3, 0xFFFF
    
    REM --- 2. Rechtes Paddle (X=30, Hoehe 4) ---
    PSET 30, RY, 0xFFFF
    PSET 30, RY + 1, 0xFFFF
    PSET 30, RY + 2, 0xFFFF
    PSET 30, RY + 3, 0xFFFF
    
    REM --- 3. Ball zeichnen ---
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
    
    REM --- 6. Paddle-Kollision Links / Rechts ---
    IF BX <= 2 THEN
        LET VX = 1
    END IF
    IF BX >= 29 THEN
        LET VX = -1
    END IF
    
    REM --- 7. Autonome KI: Paddles folgen Ballhoehe ---
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
```

---

## 2. 🔢 Mathematik & Logiktest (`math_test.bas`)

```basic
LET A = 25
LET B = 14

LET C = A + B
LET D = A - B
LET P = 12 * 7
LET Q = 100 / 8
LET R = 100 MOD 8

IF P > 80 THEN
    LET STATUS = 1
ELSE
    LET STATUS = 0
END IF

HALT
```

---

## 3. 📋 DATA / READ / RESTORE (`data_test.bas`)

```basic
DATA 100, 200, 300, 400

READ A, B
READ C, D

RESTORE
READ FIRST_AGAIN

HALT
```

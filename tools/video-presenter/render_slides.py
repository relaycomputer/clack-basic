#!/usr/bin/env python3
"""
Renders 6 high-definition 1920x1080 16:9 presentation slides for the Clack-BASIC compiler video.
Fixed typography: All text wraps strictly within card bounding boxes with generous padding.
Dark retro-computing aesthetic with deep obsidian/navy (#0b0f19), cyan (#38bdf8), amber (#f59e0b), emerald (#10b981).
Muppet Presenter Safe Zone: Bottom-right circular badge at x: 1615..1895, y: 775..1055.
All slides strictly leave this area 100% free of text, graphs, or UI elements.
"""

import os
from PIL import Image, ImageDraw, ImageFont

SLIDES_DIR = "/home/veitscherbub/work/ai/cinematic-video-pipeline/projects/clack_compiler_video/slides"
os.makedirs(SLIDES_DIR, exist_ok=True)

WIDTH, HEIGHT = 1920, 1080

# Colors
BG_COLOR = (12, 16, 26)         # Deep slate/obsidian (#0c101a)
CARD_BG = (22, 29, 46)          # Card container background
CARD_BORDER = (45, 60, 90)      # Card border outline
TEXT_WHITE = (248, 250, 252)    # Heading white
TEXT_MUTED = (148, 163, 184)    # Description gray
ACCENT_CYAN = (56, 189, 248)    # Highlight cyan (#38bdf8)
ACCENT_AMBER = (245, 158, 11)   # Accent amber (#f59e0b)
ACCENT_GREEN = (16, 185, 129)   # Accent emerald (#10b981)
ACCENT_PURPLE = (168, 85, 247)  # Accent purple
CODE_BG = (15, 23, 42)          # Code snippet background

# Fonts
FONT_TITLE = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 44)
FONT_SUBTITLE = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 22)
FONT_CARD_TITLE = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 22)
FONT_BODY = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 17)
FONT_BODY_BOLD = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 17)
FONT_CODE = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf", 16)
FONT_CODE_BOLD = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSansMono-Bold.ttf", 16)
FONT_BADGE = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 14)

def draw_header(draw, title, subtitle, tag="CLACK COMPILER ARCHITECTURE"):
    draw.rectangle([0, 0, WIDTH, 6], fill=ACCENT_CYAN)
    
    # Tag badge
    draw.rounded_rectangle([80, 40, 80 + 340, 40 + 30], radius=6, fill=(30, 41, 59), outline=ACCENT_CYAN, width=1)
    draw.text((95, 46), tag, font=FONT_BADGE, fill=ACCENT_CYAN)
    
    # Title & Subtitle
    draw.text((80, 85), title, font=FONT_TITLE, fill=TEXT_WHITE)
    draw.text((80, 145), subtitle, font=FONT_SUBTITLE, fill=ACCENT_AMBER)
    
    # Horizontal rule
    draw.line([(80, 190), (WIDTH - 80, 190)], fill=(40, 50, 75), width=2)

def draw_card(draw, x, y, w, h, title, title_color=ACCENT_CYAN):
    draw.rounded_rectangle([x, y, x + w, y + h], radius=12, fill=CARD_BG, outline=CARD_BORDER, width=2)
    # Header strip inside card
    draw.rounded_rectangle([x, y, x + w, y + 46], radius=12, fill=(30, 41, 65))
    draw.rectangle([x, y + 36, x + w, y + 46], fill=(30, 41, 65))
    draw.text((x + 18, y + 12), title, font=FONT_CARD_TITLE, fill=title_color)

def draw_wrapped_text(draw, x, y, max_w, text, font, fill, line_height=24):
    words = text.split()
    lines = []
    cur_line = []
    
    for word in words:
        test_line = " ".join(cur_line + [word])
        bbox = font.getbbox(test_line)
        w = bbox[2] - bbox[0]
        if w <= max_w:
            cur_line.append(word)
        else:
            if cur_line:
                lines.append(" ".join(cur_line))
                cur_line = [word]
            else:
                lines.append(word)
                cur_line = []
    if cur_line:
        lines.append(" ".join(cur_line))
        
    for i, line in enumerate(lines):
        draw.text((x, y + i * line_height), line, font=font, fill=fill)
    return len(lines) * line_height

# ------------------------------------------------------------------------------
# Slide 1: Introduction & Relay Constraints
# ------------------------------------------------------------------------------
def render_slide_01():
    img = Image.new("RGB", (WIDTH, HEIGHT), BG_COLOR)
    draw = ImageDraw.Draw(img)
    draw_header(draw, "Building a BASIC Compiler for a 16-Bit Relay CPU",
                "Engineering an Optimizing Compiler Under Extreme Physical Constraints",
                "01 / 06 : ARCHITECTURAL DILEMMA")
    
    # 3 Cards (Width: 460 each, spacing: 35)
    # Card 1: Physical Hardware Constraints
    draw_card(draw, 80, 220, 460, 520, "Physical Relay Constraints", ACCENT_AMBER)
    items_c1 = [
        ("16-Bit Register Datapath", "Direct ALU bus with 8 general-purpose registers (R0-R7)."),
        ("No RAM Stack Pointer", "Hardware lacks SP register and push/pop memory operations."),
        ("1-Level Return Register", "Hardware saves subroutine return address in a single RET register."),
        ("No Nested CALLs", "A second CALL overwrites the return address without warning!"),
        ("Relay Wear Minimization", "Optimize instructions to avoid unnecessary physical relay clicks.")
    ]
    cur_y = 285
    for header, desc in items_c1:
        draw.text((100, cur_y), f"• {header}", font=FONT_BODY_BOLD, fill=TEXT_WHITE)
        h = draw_wrapped_text(draw, 118, cur_y + 24, 400, desc, FONT_BODY, TEXT_MUTED, line_height=22)
        cur_y += 30 + h + 14

    # Card 2: Compiler Solutions
    draw_card(draw, 575, 220, 460, 520, "Compiler Strategies", ACCENT_CYAN)
    items_c2 = [
        ("Static Activation Records", "All local variables and return slots are statically placed in memory."),
        ("Zero-Overhead Leaf Inlining", "Complex mathematical routines designed exclusively as leaf functions."),
        ("Register Allocation Pass", "Live range analysis maximizes register re-use across loops."),
        ("Peephole Optimization", "Eliminates redundant LOAD and STORE instructions."),
        ("Deterministic Cycle Timing", "Exact relay cycle budget calculated before execution.")
    ]
    cur_y = 285
    for header, desc in items_c2:
        draw.text((595, cur_y), f"• {header}", font=FONT_BODY_BOLD, fill=TEXT_WHITE)
        h = draw_wrapped_text(draw, 613, cur_y + 24, 400, desc, FONT_BODY, TEXT_MUTED, line_height=22)
        cur_y += 30 + h + 14

    # Card 3: The Pipeline
    draw_card(draw, 1070, 220, 460, 520, "Compilation Pipeline", ACCENT_GREEN)
    steps = [
        ("1. Source Code", "Clack-BASIC (.bas) with line numbers"),
        ("2. Pratt Lexer / Parser", "Generates clean, typed AST with line tracking"),
        ("3. Semantic Analysis", "Type resolution, scope validation, symbol table"),
        ("4. Relay Code Generator", "Emits 16-bit register-mapped Clack Assembly"),
        ("5. Cycle-Accurate Sim", "Verified on web simulator before physical ROM")
    ]
    cur_y = 285
    for header, desc in steps:
        draw.rounded_rectangle([1095, cur_y, 1095 + 410, cur_y + 68], radius=8, fill=CODE_BG, outline=CARD_BORDER, width=1)
        draw.text((1110, cur_y + 10), header, font=FONT_BODY_BOLD, fill=ACCENT_GREEN)
        draw_wrapped_text(draw, 1110, cur_y + 34, 380, desc, FONT_BODY, TEXT_MUTED, line_height=20)
        cur_y += 82

    # Bottom notice (stays left of Muppet safe zone: x < 1550)
    draw.rounded_rectangle([80, 770, 1450, 840], radius=10, fill=(20, 30, 50), outline=ACCENT_CYAN, width=1)
    draw.text((105, 792), "KEY INSIGHT: The compiler turns physical relay constraints into deterministic execution.", font=FONT_BODY_BOLD, fill=ACCENT_CYAN)

    img.save(os.path.join(SLIDES_DIR, "slide_01.png"))
    print("Slide 01 generated.")

# ------------------------------------------------------------------------------
# Slide 2: Pratt Parser & Grammar
# ------------------------------------------------------------------------------
def render_slide_02():
    img = Image.new("RGB", (WIDTH, HEIGHT), BG_COLOR)
    draw = ImageDraw.Draw(img)
    draw_header(draw, "Lexer, Pratt Parser & Abstract Syntax Tree",
                "High-Precedence Expression Parsing Without Recursive Descent Bloat",
                "02 / 06 : FRONTEND & PARSER")
    
    # Left Card: Why Pratt Parsing?
    draw_card(draw, 80, 220, 680, 520, "Why We Built a Pratt Parser", ACCENT_CYAN)
    draw.text((105, 285), "Recursive Descent vs. Pratt Expression Parser:", font=FONT_BODY_BOLD, fill=TEXT_WHITE)
    
    reasons = [
        ("Single-Loop Precedence", "Pratt parsing resolves infix, prefix, and postfix operators with binding power in a single loop."),
        ("Zero Stack Overhead", "Avoids creating dozens of nested AST call frames during compilation."),
        ("Pinpoint Error Diagnostics", "Every token preserves exact line and column numbers for precise compiler warnings."),
        ("Optimized for WASM", "Fast, allocation-free parsing enables instant compilation in browser environments.")
    ]
    ry = 320
    for title, desc in reasons:
        draw.text((105, ry), f"• {title}:", font=FONT_BODY_BOLD, fill=ACCENT_AMBER)
        h = draw_wrapped_text(draw, 125, ry + 22, 610, desc, FONT_BODY, TEXT_MUTED, line_height=22)
        ry += 28 + h + 8

    # Code snippet box
    draw.rounded_rectangle([105, 550, 735, 715], radius=8, fill=CODE_BG, outline=CARD_BORDER, width=1)
    draw.text((125, 565), "; Clack-BASIC Expression Test:", font=FONT_CODE_BOLD, fill=ACCENT_AMBER)
    code_lines = [
        "10 LET X = 16",
        "20 IF (A + 4) * 2 >= SQR(X) THEN GOTO 100",
        "30 PRINT \"ROOT RESULT:\"; SQR(X)"
    ]
    cy = 600
    for line in code_lines:
        draw.text((125, cy), line, font=FONT_CODE, fill=TEXT_WHITE)
        cy += 32

    # Right Card: Operator Precedence Table (Width: 680, x: 800)
    draw_card(draw, 800, 220, 680, 520, "Operator Precedence Hierarchy", ACCENT_AMBER)
    table_data = [
        ("Level 1 (Lowest)", "OR, XOR, AND", "Logical & bitwise operations"),
        ("Level 2", "=, <>, <, >, <=, >=", "Relational comparisons"),
        ("Level 3", "+, -", "Additive expressions"),
        ("Level 4", "*, /, %", "Multiplication & restoring division"),
        ("Level 5 (Highest)", "Unary -, NOT, SQR", "Prefix & intrinsic functions")
    ]
    ty = 285
    for lvl, ops, desc in table_data:
        draw.rounded_rectangle([825, ty, 825 + 630, ty + 70], radius=8, fill=CODE_BG, outline=CARD_BORDER, width=1)
        draw.text((845, ty + 12), lvl, font=FONT_BODY_BOLD, fill=ACCENT_AMBER)
        draw.text((1050, ty + 12), ops, font=FONT_CODE_BOLD, fill=ACCENT_CYAN)
        draw_wrapped_text(draw, 845, ty + 38, 590, desc, FONT_BODY, TEXT_MUTED, line_height=20)
        ty += 84

    # Bottom notice
    draw.rounded_rectangle([80, 770, 1450, 840], radius=10, fill=(20, 30, 50), outline=ACCENT_CYAN, width=1)
    draw.text((105, 792), "ERROR TELEMETRY: Syntax errors highlight exact line and column directly in web terminal.", font=FONT_BODY_BOLD, fill=ACCENT_CYAN)

    img.save(os.path.join(SLIDES_DIR, "slide_02.png"))
    print("Slide 02 generated.")

# ------------------------------------------------------------------------------
# Slide 3: Fixed-Point Arithmetic (Q8.8)
# ------------------------------------------------------------------------------
def render_slide_03():
    img = Image.new("RGB", (WIDTH, HEIGHT), BG_COLOR)
    draw = ImageDraw.Draw(img)
    draw_header(draw, "High-Speed Fixed-Point Arithmetic (Q8.8)",
                "8-Bit Integer & 8-Bit Fractional Resolution for Real-Time Algorithms",
                "03 / 06 : NUMERICAL ENGINE")
    
    # Card 1: Bit Layout & Specification
    draw_card(draw, 80, 220, 680, 520, "16-Bit Q8.8 Format Specification", ACCENT_CYAN)
    draw.text((105, 285), "Two's Complement Binary Bit Layout:", font=FONT_BODY_BOLD, fill=TEXT_WHITE)
    
    bx = 105
    by = 320
    # Sign bit
    draw.rectangle([bx, by, bx + 45, by + 45], fill=(220, 38, 38), outline=CARD_BORDER, width=2)
    draw.text((bx + 14, by + 12), "S", font=FONT_BODY_BOLD, fill=TEXT_WHITE)
    # Integer bits (7)
    draw.rectangle([bx + 45, by, bx + 270, by + 45], fill=(30, 64, 175), outline=CARD_BORDER, width=2)
    draw.text((bx + 85, by + 12), "INTEGER (7 Bits)", font=FONT_BODY_BOLD, fill=TEXT_WHITE)
    # Fractional bits (8)
    draw.rectangle([bx + 270, by, bx + 580, by + 45], fill=(16, 185, 129), outline=CARD_BORDER, width=2)
    draw.text((bx + 335, by + 12), "FRACTION (8 Bits)", font=FONT_BODY_BOLD, fill=TEXT_WHITE)
    
    draw.text((bx + 8, by + 56), "Bit 15", font=FONT_CODE, fill=TEXT_MUTED)
    draw.text((bx + 110, by + 56), "Bits 14 .. 8", font=FONT_CODE, fill=TEXT_MUTED)
    draw.text((bx + 380, by + 56), "Bits 7 .. 0", font=FONT_CODE, fill=TEXT_MUTED)

    specs = [
        ("Signed Range:", "-128.000 to +127.996"),
        ("Resolution:", "1 / 256 = 0.00390625"),
        ("BASIC Syntax:", "DIM X AS FIXED8  (or suffix X#)")
    ]
    sy = 425
    for label, val in specs:
        draw.text((105, sy), f"• {label}", font=FONT_BODY_BOLD, fill=ACCENT_AMBER)
        draw.text((290, sy), val, font=FONT_CODE_BOLD, fill=TEXT_WHITE)
        sy += 36

    # Division box
    draw.rounded_rectangle([105, 545, 735, 715], radius=8, fill=CODE_BG, outline=CARD_BORDER, width=1)
    draw.text((125, 560), "Restoring Division Routine (__q88_div):", font=FONT_BODY_BOLD, fill=ACCENT_GREEN)
    draw_wrapped_text(draw, 125, 590, 580,
                      "Executes a 16-step non-restoring shift-subtract loop. The entire division finishes in fewer than 90 relay cycles, ideal for real-time graphics.",
                      FONT_BODY, TEXT_MUTED, line_height=22)

    # Card 2: Benchmarks
    draw_card(draw, 800, 220, 680, 520, "Performance & Algorithm Benchmarks", ACCENT_GREEN)
    bench_data = [
        ("Integer Arithmetic", "Too coarse for smooth fractals and orbits", "1 Cycle / Op"),
        ("Q8.8 Fixed-Point", "Optimal speed and precision for Mandelbrot", "88 Cycles / Div"),
        ("Software Float16", "Highest dynamic range for scientific physics", "~450 Cycles / Div")
    ]
    by = 285
    for name, desc, speed in bench_data:
        draw.rounded_rectangle([825, by, 825 + 630, by + 85], radius=8, fill=CODE_BG, outline=CARD_BORDER, width=1)
        draw.text((845, by + 12), name, font=FONT_BODY_BOLD, fill=ACCENT_CYAN)
        draw.text((1240, by + 12), speed, font=FONT_CODE_BOLD, fill=ACCENT_AMBER)
        draw_wrapped_text(draw, 845, by + 42, 590, desc, FONT_BODY, TEXT_MUTED, line_height=20)
        by += 102

    draw.text((825, 620), "RESULT: Smooth graphics on physical relays", font=FONT_BODY_BOLD, fill=TEXT_WHITE)
    draw_wrapped_text(draw, 825, 650, 610,
                      "Delivers 5.1x faster execution than software floating-point while preserving crisp fractional detail.",
                      FONT_BODY, TEXT_MUTED, line_height=22)

    # Bottom notice
    draw.rounded_rectangle([80, 770, 1450, 840], radius=10, fill=(20, 30, 50), outline=ACCENT_CYAN, width=1)
    draw.text((105, 792), "SATURATION CLAMPING: Multiplications clamp automatically against signed integer overflow.", font=FONT_BODY_BOLD, fill=ACCENT_CYAN)

    img.save(os.path.join(SLIDES_DIR, "slide_03.png"))
    print("Slide 03 generated.")

# ------------------------------------------------------------------------------
# Slide 4: CLACK-F16 Floating-Point (Zuse Style)
# ------------------------------------------------------------------------------
def render_slide_04():
    img = Image.new("RGB", (WIDTH, HEIGHT), BG_COLOR)
    draw = ImageDraw.Draw(img)
    draw_header(draw, "CLACK-F16: Zuse-Style 16-Bit Floating-Point",
                "Extreme Dynamic Range (10^±19) Inspired by Konrad Zuse's Z3 Relay Architecture",
                "04 / 06 : SCIENTIFIC FLOATING-POINT")
    
    # Card 1: Format Spec
    draw_card(draw, 80, 220, 680, 520, "CLACK-F16 Architecture (S1-E7-M8)", ACCENT_CYAN)
    draw.text((105, 285), "Zuse Float16 Bit Representation:", font=FONT_BODY_BOLD, fill=TEXT_WHITE)
    
    bx = 105
    by = 320
    # Sign bit
    draw.rectangle([bx, by, bx + 45, by + 45], fill=(220, 38, 38), outline=CARD_BORDER, width=2)
    draw.text((bx + 14, by + 12), "S", font=FONT_BODY_BOLD, fill=TEXT_WHITE)
    # Exponent bits (7)
    draw.rectangle([bx + 45, by, bx + 285, by + 45], fill=(147, 51, 234), outline=CARD_BORDER, width=2)
    draw.text((bx + 75, by + 12), "EXPONENT (7 Bits, Bias 63)", font=FONT_BODY_BOLD, fill=TEXT_WHITE)
    # Mantissa bits (8)
    draw.rectangle([bx + 285, by, bx + 580, by + 45], fill=(14, 165, 233), outline=CARD_BORDER, width=2)
    draw.text((bx + 330, by + 12), "MANTISSA (8 Bits)", font=FONT_BODY_BOLD, fill=TEXT_WHITE)

    draw.text((bx + 8, by + 56), "Bit 15", font=FONT_CODE, fill=TEXT_MUTED)
    draw.text((bx + 110, by + 56), "Bits 14 .. 8", font=FONT_CODE, fill=TEXT_MUTED)
    draw.text((bx + 370, by + 56), "Bits 7 .. 0 (Implicit 1.M)", font=FONT_CODE, fill=TEXT_MUTED)

    f_specs = [
        ("Dynamic Range:", "2^-62 ≈ 2.16 x 10^-19 to 2^64 ≈ 1.84 x 10^19"),
        ("Implicit Leading Bit:", "1.M (Yields 9 bits effective precision)"),
        ("BASIC Syntax:", "DIM F AS FLOAT16")
    ]
    sy = 425
    for label, val in f_specs:
        draw.text((105, sy), f"• {label}", font=FONT_BODY_BOLD, fill=ACCENT_AMBER)
        draw.text((320, sy), val, font=FONT_CODE_BOLD, fill=TEXT_WHITE)
        sy += 36

    draw.rounded_rectangle([105, 550, 735, 715], radius=8, fill=CODE_BG, outline=CARD_BORDER, width=1)
    draw.text((125, 565), "Konrad Zuse Tribute (1941 Z3 Design):", font=FONT_BODY_BOLD, fill=ACCENT_GREEN)
    draw_wrapped_text(draw, 125, 595, 580,
                      "Zuse used floating-point relays because scientific calculations quickly exceed fixed-point limits. CLACK-F16 modernizes this heritage with full normalization.",
                      FONT_BODY, TEXT_MUTED, line_height=22)

    # Card 2: Zero-Call Leaf Subroutines
    draw_card(draw, 800, 220, 680, 520, "Zero-Call Leaf Subroutine Engine", ACCENT_AMBER)
    draw.text((825, 285), "Overcoming the Single-Level Return Stack:", font=FONT_BODY_BOLD, fill=TEXT_WHITE)
    
    routines = [
        ("__f16_add / __f16_sub", "Denormal alignment, 16-bit addition, and re-normalization in leaf."),
        ("__f16_mul", "Exponent add with bias compensation, 8x8 mantissa multiply in leaf."),
        ("__f16_div", "Exponent subtract, 16-step restoring mantissa divide."),
        ("__f16_itof / __f16_ftoi", "Fast integer conversion without recursive call nesting.")
    ]
    ry = 325
    for name, desc in routines:
        draw.rounded_rectangle([825, ry, 825 + 630, ry + 68], radius=8, fill=CODE_BG, outline=CARD_BORDER, width=1)
        draw.text((845, ry + 10), name, font=FONT_CODE_BOLD, fill=ACCENT_CYAN)
        draw_wrapped_text(draw, 845, ry + 34, 590, desc, FONT_BODY, TEXT_MUTED, line_height=20)
        ry += 78

    draw.text((825, 665), "NO STACK CORRUPTION: User programs can safely CALL __f16_* functions.", font=FONT_BODY_BOLD, fill=ACCENT_GREEN)

    # Bottom notice
    draw.rounded_rectangle([80, 770, 1450, 840], radius=10, fill=(20, 30, 50), outline=ACCENT_CYAN, width=1)
    draw.text((105, 792), "FULL NORMALIZATION: Features denormal detection, underflow zero-flush, and overflow protection.", font=FONT_BODY_BOLD, fill=ACCENT_CYAN)

    img.save(os.path.join(SLIDES_DIR, "slide_04.png"))
    print("Slide 04 generated.")

# ------------------------------------------------------------------------------
# Slide 5: Code Generation & Simulator
# ------------------------------------------------------------------------------
def render_slide_05():
    img = Image.new("RGB", (WIDTH, HEIGHT), BG_COLOR)
    draw = ImageDraw.Draw(img)
    draw_header(draw, "Code Generation & Cycle-Accurate Simulator",
                "Translating High-Level BASIC Down to Physical Relay Bus Telemetry",
                "05 / 06 : CODEGEN & TELEMETRY")
    
    # Left Card: Source to Assembly
    draw_card(draw, 80, 220, 680, 520, "Input BASIC -> Emitted Assembly", ACCENT_CYAN)
    
    draw.rounded_rectangle([105, 280, 735, 450], radius=8, fill=CODE_BG, outline=CARD_BORDER, width=1)
    draw.text((120, 295), "; BASIC Source (squareroot.bas):", font=FONT_CODE_BOLD, fill=ACCENT_AMBER)
    draw.text((120, 330), "10 DIM X, G AS FIXED8", font=FONT_CODE, fill=TEXT_WHITE)
    draw.text((120, 365), "20 X = 2.0# : G = 1.0#", font=FONT_CODE, fill=TEXT_WHITE)
    draw.text((120, 400), "30 G = (G + X / G) / 2.0#", font=FONT_CODE, fill=ACCENT_CYAN)

    draw.rounded_rectangle([105, 470, 735, 715], radius=8, fill=CODE_BG, outline=CARD_BORDER, width=1)
    draw.text((120, 485), "; Generated Clack Relay Assembly:", font=FONT_CODE_BOLD, fill=ACCENT_GREEN)
    asm_lines = [
        "LOAD R0, [VAR_X]    ; Load fixed-point X",
        "LOAD R1, [VAR_G]    ; Load guess G",
        "MOV  R2, R0",
        "CALL __q88_div      ; R2 = X / G",
        "ADD  R2, R1         ; R2 = G + X/G",
        "SHR  R2, 1          ; Divide by 2"
    ]
    ay = 520
    for al in asm_lines:
        draw.text((120, ay), al, font=FONT_CODE, fill=TEXT_WHITE)
        ay += 28

    # Right Card: Simulator Telemetry
    draw_card(draw, 800, 220, 680, 520, "Cycle-Accurate Web Simulator", ACCENT_AMBER)
    sim_points = [
        ("Single-Step Execution", "Trace every instruction and inspect all 8 general-purpose registers."),
        ("Relay Bus Telemetry", "Visualize active control lines, ALU opcodes, and data bus states."),
        ("Hardware ALU Flags", "Full emulation of Zero (Z), Carry (C), Overflow (V), and Negative (N)."),
        ("Acoustic Audio Synthesis", "Authentic audio synthesis of physical relay clicks per cycle."),
        ("EPROM Binary Export", "One-click generation of raw binary images for EEPROM burning.")
    ]
    sy = 280
    for title, desc in sim_points:
        draw.rounded_rectangle([825, sy, 825 + 630, sy + 70], radius=8, fill=CODE_BG, outline=CARD_BORDER, width=1)
        draw.text((845, sy + 10), title, font=FONT_BODY_BOLD, fill=ACCENT_CYAN)
        draw_wrapped_text(draw, 845, sy + 36, 590, desc, FONT_BODY, TEXT_MUTED, line_height=20)
        sy += 82

    # Bottom notice
    draw.rounded_rectangle([80, 770, 1450, 840], radius=10, fill=(20, 30, 50), outline=ACCENT_CYAN, width=1)
    draw.text((105, 792), "SAFE VERIFICATION: Catch logic bugs and stack overflows before touching physical relay coils.", font=FONT_BODY_BOLD, fill=ACCENT_CYAN)

    img.save(os.path.join(SLIDES_DIR, "slide_05.png"))
    print("Slide 05 generated.")

# ------------------------------------------------------------------------------
# Slide 6: Outro & Ecosystem
# ------------------------------------------------------------------------------
def render_slide_06():
    img = Image.new("RGB", (WIDTH, HEIGHT), BG_COLOR)
    draw = ImageDraw.Draw(img)
    draw_header(draw, "Open Source Toolchain & What's Next",
                "Try Clack-BASIC in Your Browser Today at relay-computer.org",
                "06 / 06 : COMMUNITY & OUTRO")
    
    # 3 Cards
    # Card 1: Try it in Browser
    draw_card(draw, 80, 220, 460, 520, "WebAssembly Playground", ACCENT_CYAN)
    draw.text((105, 280), "Full Compiler in Browser:", font=FONT_BODY_BOLD, fill=TEXT_WHITE)
    points_c1 = [
        "Go compiler compiled directly to WASM.",
        "Instant compilation in under 10ms.",
        "Built-in examples: Babylonian SQR, prime sieve, Mandelbrot, and floating-point."
    ]
    py = 315
    for p in points_c1:
        h = draw_wrapped_text(draw, 105, py, 410, f"• {p}", FONT_BODY, TEXT_MUTED, line_height=22)
        py += h + 10

    draw.rounded_rectangle([105, 520, 515, 680], radius=8, fill=CODE_BG, outline=CARD_BORDER, width=1)
    draw.text((125, 540), "PLAYGROUND URL:", font=FONT_BADGE, fill=ACCENT_AMBER)
    draw.text((125, 575), "relay-computer.org", font=FONT_BODY_BOLD, fill=ACCENT_CYAN)
    draw.text((125, 615), "No installation required!", font=FONT_BODY, fill=TEXT_WHITE)

    # Card 2: Coming Next
    draw_card(draw, 575, 220, 460, 520, "Coming in Video 2", ACCENT_AMBER)
    draw.text((600, 280), "Clack Studio IDE:", font=FONT_BODY_BOLD, fill=TEXT_WHITE)
    points_c2 = [
        "Syntax-highlighted code editor.",
        "Real-time relay state animations.",
        "Memory map and register inspector.",
        "Step-by-step disassembly view.",
        "Acoustic relay click synthesizer."
    ]
    py = 315
    for p in points_c2:
        draw.text((600, py), f"• {p}", font=FONT_BODY, fill=TEXT_MUTED)
        py += 32

    draw.rounded_rectangle([600, 520, 1010, 680], radius=8, fill=CODE_BG, outline=CARD_BORDER, width=1)
    draw.text((620, 540), "UP NEXT:", font=FONT_BADGE, fill=ACCENT_GREEN)
    draw.text((620, 575), "Clack Studio Walkthrough", font=FONT_BODY_BOLD, fill=TEXT_WHITE)
    draw.text((620, 615), "Interactive relay debugging", font=FONT_BODY, fill=TEXT_MUTED)

    # Card 3: Peter's Hardware Channel
    draw_card(draw, 1070, 220, 460, 520, "Peter's Hardware Channel", ACCENT_GREEN)
    draw.text((1095, 280), "Physical Relay Machine:", font=FONT_BODY_BOLD, fill=TEXT_WHITE)
    points_c3 = [
        "Watch Peter build the physical racks.",
        "Relays, diodes, LEDs, and bus wiring.",
        "Power supply and acoustic engineering."
    ]
    py = 315
    for p in points_c3:
        h = draw_wrapped_text(draw, 1095, py, 410, f"• {p}", FONT_BODY, TEXT_MUTED, line_height=22)
        py += h + 10

    draw.rounded_rectangle([1095, 500, 1505, 680], radius=8, fill=CODE_BG, outline=CARD_BORDER, width=1)
    draw.text((1115, 520), "LINKS IN DESCRIPTION:", font=FONT_BADGE, fill=ACCENT_CYAN)
    draw.text((1115, 555), "• Peter's Hardware Channel", font=FONT_BODY, fill=TEXT_WHITE)
    draw.text((1115, 590), "• GitHub Repositories", font=FONT_BODY, fill=TEXT_WHITE)
    draw.text((1115, 625), "• Wiki Documentation", font=FONT_BODY, fill=TEXT_WHITE)

    # Bottom notice
    draw.rounded_rectangle([80, 770, 1450, 840], radius=10, fill=(20, 30, 50), outline=ACCENT_CYAN, width=1)
    draw.text((105, 792), "SUBSCRIBE & CLICK THE BELL: Don't miss Video 2 on Clack Studio & live relay debugging!", font=FONT_BODY_BOLD, fill=ACCENT_CYAN)

    img.save(os.path.join(SLIDES_DIR, "slide_06.png"))
    print("Slide 06 generated.")

def main():
    print("Generating all 6 presentation slides with strict text wrapping...")
    render_slide_01()
    render_slide_02()
    render_slide_03()
    render_slide_04()
    render_slide_05()
    render_slide_06()
    print("All 6 slides successfully regenerated in 1920x1080!")

if __name__ == "__main__":
    main()

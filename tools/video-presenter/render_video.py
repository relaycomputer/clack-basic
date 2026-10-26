#!/usr/bin/env python3
"""
Full Presentation Video Production Engine for Clack-BASIC Compiler Series (v3).
Upgrades & Features:
1. Voice: en-GB-RyanNeural (British vintage computer scientist narrator).
2. Active Visual Motion (Zero Static Frames!):
   - Synchronized Laser Pointer & Active Card Highlighting: Smoothly moves between
     the active cards/sections being discussed, drawing a glowing highlight frame,
     active progress bar, and animated laser pointer dot with pulsing aura.
   - Live 16-Bit Relay Telemetry Bar: Animated top-right hardware status bar showing
     live Program Counter (PC), Instruction Register (IR), ALU Flags (Z/C/V/N),
     and toggling 16-bit relay LEDs synced to execution cycles.
   - Subtle Camera Pan/Zoom (Ken Burns): Gentle, smooth focus shifts toward the
     currently discussed region while keeping the presenter PiP rock-solid.
3. Enhanced 2.5D Muppet Presenter Kinematics:
   - Glitch-free isolated mouth cavity morphing (closed, medium, open).
   - Dynamic multi-phase pointing gestures toward the active slide card.
   - Expressive head-turning, speech-cadence nodding, and natural felt eyelid blinking.
"""

import math
import os
import subprocess
import sys
import time
from typing import Dict, List, Tuple

import cv2
import numpy as np
from scipy.ndimage import gaussian_filter1d
import soundfile as sf

# Project directories
PROJECT_DIR = "/home/veitscherbub/work/ai/cinematic-video-pipeline/projects/clack_compiler_video"
ASSETS_AVATAR_DIR = "/home/veitscherbub/work/ai/cinematic-video-pipeline/assets/avatar"
SLIDES_DIR = os.path.join(PROJECT_DIR, "slides")
AUDIO_DIR = os.path.join(PROJECT_DIR, "audio")
SEGMENTS_DIR = os.path.join(PROJECT_DIR, "segments")
OUTPUT_DIR = "/home/veitscherbub/work/ai/cinematic-video-pipeline/output"
FINAL_MASTER_MP4 = os.path.join(OUTPUT_DIR, "Clack_BASIC_Compiler_Ryan_Dynamic_1080p.mp4")

os.makedirs(SEGMENTS_DIR, exist_ok=True)
os.makedirs(OUTPUT_DIR, exist_ok=True)

# Video standards
CANVAS_WIDTH = 1920
CANVAS_HEIGHT = 1080
FRAME_RATE = 30
AUDIO_SAMPLE_RATE = 48000

# Presenter PiP Circular Badge Geometry (Bottom-Right Clear Zone)
PIP_SIZE = 280
PIP_MARGIN = 25
PIP_X = CANVAS_WIDTH - PIP_SIZE - PIP_MARGIN   # 1615 px
PIP_Y = CANVAS_HEIGHT - PIP_SIZE - PIP_MARGIN  # 775 px
PIP_BORDER_WIDTH = 5

POST_SPEECH_PAUSE_SEC = 1.5

# Per-slide synchronization waypoints:
# Each waypoint defines (start_frac, end_frac, card_rect (x, y, w, h), pointer_target (px, py), label)
SLIDE_WAYPOINTS: Dict[int, List[Tuple[float, float, Tuple[int, int, int, int], Tuple[int, int], str]]] = {
    1: [
        (0.00, 0.25, (80, 220, 460, 520), (115, 300), "16-BIT RELAY DATAPATH"),
        (0.25, 0.52, (80, 220, 460, 520), (115, 475), "NO RAM STACK / 1-LEVEL RET"),
        (0.52, 0.78, (575, 220, 460, 520), (610, 385), "STATIC ACTIVATION & LEAF INLINING"),
        (0.78, 1.00, (1070, 220, 460, 520), (1110, 465), "5-STAGE COMPILER PIPELINE"),
    ],
    2: [
        (0.00, 0.30, (80, 220, 680, 520), (120, 340), "PRATT PARSER FRONTEND"),
        (0.30, 0.58, (800, 220, 680, 520), (840, 405), "OPERATOR PRECEDENCE LEVELS 1..5"),
        (0.58, 0.82, (80, 220, 680, 520), (120, 460), "AST SOURCE LINE TRACKING"),
        (0.82, 1.00, (105, 550, 630, 165), (140, 630), "SINGLE-PASS BASIC COMPILATION"),
    ],
    3: [
        (0.00, 0.28, (800, 220, 680, 520), (840, 325), "MANDELBROT FRACTAL PRECISION"),
        (0.28, 0.60, (80, 220, 680, 520), (340, 342), "Q8.8 BIT LAYOUT (S1-I7-F8)"),
        (0.60, 0.82, (80, 220, 680, 520), (280, 460), "SIGNED RANGE -128 TO +127.996"),
        (0.82, 1.00, (105, 545, 630, 170), (140, 605), "88-CYCLE NON-RESTORING DIVISION"),
    ],
    4: [
        (0.00, 0.30, (80, 220, 680, 520), (320, 425), "DYNAMIC RANGE 10^-19 TO 10^+19"),
        (0.30, 0.58, (80, 220, 680, 520), (350, 342), "ZUSE Z3 S1-E7-M8 FORMAT"),
        (0.58, 0.85, (800, 220, 680, 520), (845, 420), "ZERO-CALL LEAF FPU ROUTINES"),
        (0.85, 1.00, (80, 770, 1370, 70), (140, 805), "IEEE-STYLE NORMALIZATION"),
    ],
    5: [
        (0.00, 0.32, (105, 280, 630, 170), (135, 395), "HERON / BABYLONIAN SQR DEMO"),
        (0.32, 0.62, (105, 470, 630, 245), (135, 595), "GENERATED CLACK RELAY ASSEMBLY"),
        (0.62, 0.85, (800, 220, 680, 520), (845, 395), "CYCLE-ACCURATE WEB SIMULATOR"),
        (0.85, 1.00, (800, 220, 680, 520), (845, 560), "ALU FLAGS & RELAY TELEMETRY"),
    ],
    6: [
        (0.00, 0.35, (80, 220, 460, 520), (135, 575), "WASM PLAYGROUND IN BROWSER"),
        (0.35, 0.68, (575, 220, 460, 520), (610, 410), "COMING NEXT: CLACK STUDIO IDE"),
        (0.68, 1.00, (1070, 220, 460, 520), (1110, 410), "PETER'S HARDWARE CHANNEL"),
    ],
}

RELAY_OPCODES = [
    ("LOAD R0", 0x1042),
    ("ADD  R1", 0x2408),
    ("SHL  R2", 0x4810),
    ("CALL F16", 0x9C00),
    ("SUB  R3", 0x3104),
    ("AND  R0", 0x50FF),
    ("BNE  LP", 0x7E12),
    ("STOR R2", 0x8240),
]


def smoothstep(edge0: float, edge1: float, x: float) -> float:
    t = max(0.0, min(1.0, (x - edge0) / (edge1 - edge0 + 1e-9)))
    return t * t * (3.0 - 2.0 * t)


def get_interpolated_motion(slide_index: int, progress: float):
    wps = SLIDE_WAYPOINTS.get(slide_index, SLIDE_WAYPOINTS[1])
    n = len(wps)
    active_idx = 0
    for i, (s, e, _, _, _) in enumerate(wps):
        if s <= progress <= e:
            active_idx = i
            break
    else:
        if progress > wps[-1][1]:
            active_idx = n - 1

    s, e, rect_cur, ptr_cur, label_cur = wps[active_idx]
    local_t = (progress - s) / max(1e-6, (e - s))

    # Smooth transition to next waypoint during the last 18% of current waypoint
    if active_idx < n - 1 and local_t > 0.82:
        blend = smoothstep(0.82, 1.0, local_t)
        _, _, rect_next, ptr_next, _ = wps[active_idx + 1]
        rx = rect_cur[0] + (rect_next[0] - rect_cur[0]) * blend
        ry = rect_cur[1] + (rect_next[1] - rect_cur[1]) * blend
        rw = rect_cur[2] + (rect_next[2] - rect_cur[2]) * blend
        rh = rect_cur[3] + (rect_next[3] - rect_cur[3]) * blend
        px = ptr_cur[0] + (ptr_next[0] - ptr_cur[0]) * blend
        py = ptr_cur[1] + (ptr_next[1] - ptr_cur[1]) * blend
    else:
        rx, ry, rw, rh = rect_cur
        # Add subtle organic scanning motion to pointer within the section
        scan_x = math.sin(local_t * math.pi * 2.0) * 35.0 + local_t * 65.0
        scan_y = math.cos(local_t * math.pi * 4.0) * 8.0
        px = ptr_cur[0] + scan_x
        py = ptr_cur[1] + scan_y

    return (int(rx), int(ry), int(rw), int(rh)), (int(px), int(py)), label_cur, active_idx, local_t


def draw_dynamic_overlays(
    frame_img: np.ndarray,
    slide_index: int,
    f: int,
    total_frames: int,
    speech_frames: int,
    amp: float,
) -> np.ndarray:
    out = frame_img.copy()
    progress = min(1.0, f / float(max(1, speech_frames)))
    rect, ptr, label, wp_idx, local_t = get_interpolated_motion(slide_index, progress)
    rx, ry, rw, rh = rect
    px, py = ptr

    # 1. Active Card Glowing Highlight Border & Corner Brackets
    overlay = out.copy()
    pulse = 0.5 + 0.5 * math.sin(f * 0.22)
    # Amber/Cyan active glow border
    glow_color = (18, 175, 250)  # BGR bright amber-gold (#faaf12)
    cyan_color = (248, 189, 56)  # BGR cyan (#38bdf8)

    # Subtle bright tint inside the active card
    cv2.rectangle(overlay, (rx, ry), (rx + rw, ry + rh), (40, 58, 88), 3)
    out = cv2.addWeighted(overlay, 0.55, out, 0.45, 0)

    # Corner target brackets on the active card
    b_len = 26
    b_thick = 3
    # Top-left
    cv2.line(out, (rx - 2, ry - 2), (rx + b_len, ry - 2), glow_color, b_thick, cv2.LINE_AA)
    cv2.line(out, (rx - 2, ry - 2), (rx - 2, ry + b_len), glow_color, b_thick, cv2.LINE_AA)
    # Top-right
    cv2.line(out, (rx + rw + 2, ry - 2), (rx + rw - b_len, ry - 2), glow_color, b_thick, cv2.LINE_AA)
    cv2.line(out, (rx + rw + 2, ry - 2), (rx + rw + 2, ry + b_len), glow_color, b_thick, cv2.LINE_AA)
    # Bottom-left
    cv2.line(out, (rx - 2, ry + rh + 2), (rx + b_len, ry + rh + 2), glow_color, b_thick, cv2.LINE_AA)
    cv2.line(out, (rx - 2, ry + rh + 2), (rx - 2, ry + rh - b_len), glow_color, b_thick, cv2.LINE_AA)
    # Bottom-right
    cv2.line(out, (rx + rw + 2, ry + rh + 2), (rx + rw - b_len, ry + rh + 2), glow_color, b_thick, cv2.LINE_AA)
    cv2.line(out, (rx + rw + 2, ry + rh + 2), (rx + rw + 2, ry + rh - b_len), glow_color, b_thick, cv2.LINE_AA)

    # 2. Animated Laser Pointer / Focus Reticle with Trailing Beam from Presenter PiP
    # Subtle laser line from Muppet presenter hand toward the active pointer when pointing
    pip_hand_origin = (PIP_X + 35, PIP_Y + 135)
    beam_overlay = out.copy()
    cv2.line(beam_overlay, pip_hand_origin, (px, py), (40, 90, 255), 2, cv2.LINE_AA)
    out = cv2.addWeighted(beam_overlay, 0.22, out, 0.78, 0)

    # Laser pointer outer pulsing halo
    halo_r = int(18 + 6 * pulse + 8 * amp)
    halo_overlay = out.copy()
    cv2.circle(halo_overlay, (px, py), halo_r, (30, 80, 255), -1, cv2.LINE_AA)
    cv2.circle(halo_overlay, (px, py), int(halo_r * 0.6), (20, 180, 255), -1, cv2.LINE_AA)
    out = cv2.addWeighted(halo_overlay, 0.38, out, 0.62, 0)

    # Crisp laser core & outer ring
    cv2.circle(out, (px, py), halo_r, (30, 140, 255), 2, cv2.LINE_AA)
    cv2.circle(out, (px, py), 7, (60, 210, 255), -1, cv2.LINE_AA)
    cv2.circle(out, (px, py), 3, (255, 255, 255), -1, cv2.LINE_AA)

    # Floating pointer callout tag next to laser dot
    tag_w = len(label) * 9 + 24
    tag_x = min(px + 20, 1480 - tag_w)
    tag_y = max(210, py - 28)
    tag_overlay = out.copy()
    cv2.rectangle(tag_overlay, (tag_x, tag_y), (tag_x + tag_w, tag_y + 26), (16, 24, 42), -1)
    cv2.rectangle(tag_overlay, (tag_x, tag_y), (tag_x + tag_w, tag_y + 26), glow_color, 1, cv2.LINE_AA)
    out = cv2.addWeighted(tag_overlay, 0.85, out, 0.15, 0)
    cv2.putText(
        out, label, (tag_x + 10, tag_y + 18),
        cv2.FONT_HERSHEY_SIMPLEX, 0.45, (240, 250, 255), 1, cv2.LINE_AA
    )

    # 3. Live 16-Bit Relay Telemetry HUD in Top-Right Header (x: 1180..1840, y: 28..172)
    hud_x, hud_y, hud_w, hud_h = 1220, 32, 620, 138
    hud_overlay = out.copy()
    cv2.rectangle(hud_overlay, (hud_x, hud_y), (hud_x + hud_w, hud_y + hud_h), (18, 25, 42), -1)
    cv2.rectangle(hud_overlay, (hud_x, hud_y), (hud_x + hud_w, hud_y + hud_h), (65, 90, 130), 1)
    out = cv2.addWeighted(hud_overlay, 0.88, out, 0.12, 0)

    cycle_num = (slide_index - 1) * 240 + (f // 3)
    op_name, op_hex = RELAY_OPCODES[cycle_num % len(RELAY_OPCODES)]
    pc_val = 0x0200 + ((cycle_num * 2) & 0x01FF)
    reg_val = ((cycle_num * 0x3B59 + slide_index * 0x1103) ^ (int(amp * 255))) & 0xFFFF

    cv2.putText(
        out,
        f"LIVE RELAY CPU TELEMETRY  |  CLK: 12 Hz  |  CYCLE #{cycle_num:04d}",
        (hud_x + 16, hud_y + 24),
        cv2.FONT_HERSHEY_SIMPLEX, 0.46, cyan_color, 1, cv2.LINE_AA
    )
    cv2.putText(
        out,
        f"PC: 0x{pc_val:04X}   IR: {op_name} (0x{op_hex:04X})   ACC: 0x{reg_val:04X}",
        (hud_x + 16, hud_y + 52),
        cv2.FONT_HERSHEY_SIMPLEX, 0.48, (240, 248, 255), 1, cv2.LINE_AA
    )

    # 16 Animated Relay Bus LEDs
    cv2.putText(out, "BUS [15:0]:", (hud_x + 16, hud_y + 88), cv2.FONT_HERSHEY_SIMPLEX, 0.44, (160, 175, 195), 1, cv2.LINE_AA)
    led_start_x = hud_x + 115
    led_y = hud_y + 82
    for bit in range(16):
        bit_on = ((reg_val >> (15 - bit)) & 1) == 1
        lx = led_start_x + bit * 22
        if bit_on:
            cv2.circle(out, (lx, led_y), 7, (80, 230, 40), -1, cv2.LINE_AA)
            cv2.circle(out, (lx, led_y), 3, (200, 255, 180), -1, cv2.LINE_AA)
        else:
            cv2.circle(out, (lx, led_y), 6, (35, 50, 40), -1, cv2.LINE_AA)
        cv2.circle(out, (lx, led_y), 7, (70, 95, 80), 1, cv2.LINE_AA)

    # ALU Flags (Z, C, V, N) + Audio Waveform Meter
    z_flag = (reg_val == 0)
    c_flag = ((cycle_num % 3) == 0)
    v_flag = False
    n_flag = ((reg_val & 0x8000) != 0)
    flags = [("Z", z_flag), ("C", c_flag), ("V", v_flag), ("N", n_flag)]
    fx = hud_x + 16
    fy = hud_y + 120
    cv2.putText(out, "ALU FLAGS:", (fx, fy), cv2.FONT_HERSHEY_SIMPLEX, 0.44, (160, 175, 195), 1, cv2.LINE_AA)
    for idx_f, (fn, f_on) in enumerate(flags):
        bx = fx + 95 + idx_f * 52
        col = (20, 180, 255) if f_on else (50, 60, 80)
        cv2.rectangle(out, (bx, fy - 15), (bx + 40, fy + 4), col, -1 if f_on else 1)
        t_col = (10, 15, 25) if f_on else (130, 145, 165)
        cv2.putText(out, f"{fn}:{int(f_on)}", (bx + 6, fy - 1), cv2.FONT_HERSHEY_SIMPLEX, 0.40, t_col, 1, cv2.LINE_AA)

    # Live Voice Oscilloscope Mini-Bars in HUD
    vx_start = hud_x + 335
    cv2.putText(out, "VOICE:", (vx_start, fy), cv2.FONT_HERSHEY_SIMPLEX, 0.42, (160, 175, 195), 1, cv2.LINE_AA)
    for b_i in range(18):
        wave_h = int(3 + 16 * amp * (0.4 + 0.6 * abs(math.sin(f * 0.45 + b_i * 0.55))))
        bx = vx_start + 58 + b_i * 11
        cv2.rectangle(out, (bx, fy + 2 - wave_h), (bx + 7, fy + 2), cyan_color, -1)

    # 4. Slide Timeline Progress Bar at the very bottom (x: 80..1550, y: 1045)
    bar_x1, bar_x2, bar_y = 80, 1550, 1048
    cv2.rectangle(out, (bar_x1, bar_y), (bar_x2, bar_y + 6), (30, 42, 65), -1)
    fill_x = int(bar_x1 + (bar_x2 - bar_x1) * min(1.0, f / float(max(1, total_frames))))
    cv2.rectangle(out, (bar_x1, bar_y), (fill_x, bar_y + 6), cyan_color, -1)

    return out


def create_circular_pip_masks(size: int, border_width: int, border_bgr: Tuple[int, int, int]) -> Tuple[np.ndarray, np.ndarray]:
    center = (size // 2, size // 2)
    radius = (size // 2) - border_width
    mask = np.zeros((size, size), dtype=np.float32)
    cv2.circle(mask, center, radius, 1.0, -1)
    mask_blurred = cv2.GaussianBlur(mask, (5, 5), 0)
    mask_3d = np.repeat(mask_blurred[:, :, np.newaxis], 3, axis=2)

    border_overlay = np.zeros((size, size, 3), dtype=np.uint8)
    cv2.circle(border_overlay, center, (size // 2) - (border_width // 2) - 1, border_bgr, border_width, cv2.LINE_AA)
    return mask_3d, border_overlay


def build_mouth_mask(h: int = 1024, w: int = 1024) -> np.ndarray:
    """Builds a feathered mask strictly around the mouth cavity (never touching the chin)."""
    mouth_mask = np.zeros((h, w), dtype=np.float32)
    mouth_poly = np.array([
        [320, 545],
        [512, 535],
        [704, 545],
        [735, 680],
        [675, 780],
        [512, 798],
        [350, 780],
        [290, 680],
    ], dtype=np.int32)
    cv2.fillPoly(mouth_mask, [mouth_poly], 1.0)
    mouth_mask = cv2.GaussianBlur(mouth_mask, (31, 31), 0)
    return np.repeat(mouth_mask[:, :, np.newaxis], 3, axis=2)


def render_slide_segment(
    slide_index: int,
    slide_png_path: str,
    audio_wav_path: str,
    img_closed: np.ndarray,
    img_mid: np.ndarray,
    img_open: np.ndarray,
    img_point: np.ndarray,
    mouth_mask_3d: np.ndarray,
    output_segment_mp4: str,
    fps: int = FRAME_RATE,
) -> str:
    print(f"\n🎬 Rendering Slide {slide_index:02d}: {os.path.basename(slide_png_path)}")

    base_slide = cv2.imread(slide_png_path)
    if base_slide is None:
        raise FileNotFoundError(f"Cannot load slide image: {slide_png_path}")
    if (base_slide.shape[1], base_slide.shape[0]) != (CANVAS_WIDTH, CANVAS_HEIGHT):
        base_slide = cv2.resize(base_slide, (CANVAS_WIDTH, CANVAS_HEIGHT), interpolation=cv2.INTER_LANCZOS4)

    raw_audio, sr = sf.read(audio_wav_path)
    if raw_audio.ndim > 1:
        raw_audio = np.mean(raw_audio, axis=1)

    speech_duration = len(raw_audio) / float(sr)
    speech_frames = int(speech_duration * fps)

    # Post-speech pause buffer
    silence_samples = int(POST_SPEECH_PAUSE_SEC * sr)
    silence_buf = np.zeros(silence_samples, dtype=raw_audio.dtype)
    padded_audio = np.concatenate([raw_audio, silence_buf])
    total_duration = len(padded_audio) / float(sr)
    total_frames = int(math.ceil(total_duration * fps))

    temp_padded_wav = f"/tmp/padded_slide_{slide_index}_{int(time.time()*1000)}.wav"
    sf.write(temp_padded_wav, padded_audio, sr)

    # Compute frame-level RMS energy envelope
    samples_per_frame = sr / float(fps)
    envelope = np.zeros(total_frames, dtype=np.float32)
    for f in range(speech_frames):
        s_idx = int(f * samples_per_frame)
        e_idx = min(len(raw_audio), int((f + 1) * samples_per_frame))
        if s_idx < len(raw_audio):
            chunk = raw_audio[s_idx:e_idx]
            rms = np.sqrt(np.mean(chunk**2)) if len(chunk) > 0 else 0.0
            envelope[f] = rms

    p95 = np.percentile(envelope[:speech_frames], 95) if speech_frames > 0 else 1.0
    norm_env = np.clip(envelope / (p95 * 0.85 + 1e-6), 0.0, 1.0)
    smooth_env = gaussian_filter1d(norm_env, sigma=1.0)

    # PiP border and masks
    border_bgr = (248, 189, 56)  # Cyan #38bdf8 in BGR
    pip_mask_3d, pip_border = create_circular_pip_masks(PIP_SIZE, PIP_BORDER_WIDTH, border_bgr)
    border_mask = (pip_border.sum(axis=2) > 0)[:, :, np.newaxis]

    # Facial coordinates
    eye_l_center = (430, 415)
    eye_r_center = (595, 415)
    eye_radius = 50
    felt_eyelid_color = (125, 165, 205)
    head_center = (512, 450)

    next_blink = int(fps * 1.8)

    # Foreground arm mask for pointing pose
    diff = cv2.absdiff(img_point, img_closed)
    gray = cv2.cvtColor(diff, cv2.COLOR_BGR2GRAY)
    h, w, _ = img_closed.shape
    raw_arm = np.zeros((h, w), dtype=np.float32)
    raw_arm[(gray > 18) & (np.arange(h)[:, None] >= 710)] = 1.0
    raw_arm[(gray > 18) & (np.arange(h)[:, None] < 710) & (np.arange(w)[None, :] < 360)] = 1.0
    kernel = cv2.getStructuringElement(cv2.MORPH_ELLIPSE, (7, 7))
    raw_arm = cv2.morphologyEx(raw_arm, cv2.MORPH_CLOSE, kernel)
    arm_mask = cv2.GaussianBlur(raw_arm, (15, 15), 0)[:, :, np.newaxis]

    temp_raw_video = f"/tmp/slide_raw_{slide_index}_{int(time.time()*1000)}.mp4"
    ffmpeg_pipe_cmd = [
        "ffmpeg", "-y",
        "-f", "rawvideo",
        "-vcodec", "rawvideo",
        "-s", f"{CANVAS_WIDTH}x{CANVAS_HEIGHT}",
        "-pix_fmt", "bgr24",
        "-r", str(fps),
        "-i", "-",
        "-c:v", "libx264",
        "-pix_fmt", "yuv420p",
        "-preset", "fast",
        "-crf", "18",
        temp_raw_video,
    ]
    proc = subprocess.Popen(ffmpeg_pipe_cmd, stdin=subprocess.PIPE)

    for f in range(total_frames):
        amp = float(smooth_env[f])

        # Draw dynamic slide overlays (laser pointer, active card brackets, live 16-bit relay telemetry HUD)
        comp_slide = draw_dynamic_overlays(
            frame_img=base_slide,
            slide_index=slide_index,
            f=f,
            total_frames=total_frames,
            speech_frames=speech_frames,
            amp=amp,
        )

        # 1. Glitch-Free Mouth Morphing (Piecewise linear interpolation within mouth cavity)
        if amp < 0.02:
            mouth_frame = img_closed
        elif amp < 0.40:
            t = amp / 0.40
            mouth_frame = cv2.addWeighted(img_closed, 1.0 - t, img_mid, t, 0)
        else:
            t = (amp - 0.40) / 0.60
            mouth_frame = cv2.addWeighted(img_mid, 1.0 - t, img_open, t, 0)

        head_composite = (img_closed * (1.0 - mouth_mask_3d) + mouth_frame * mouth_mask_3d).astype(np.uint8)

        # 2. Dynamic multi-phase pointing gesture synced to speech & waypoint transitions
        if f < speech_frames:
            # Point actively during speech with brief natural hand relaxations between phrases
            wave_pose = 0.65 + 0.35 * math.sin(f * 0.08)
            pose_weight = min(1.0, max(0.25, wave_pose))
        else:
            fade = max(0.0, 1.0 - (f - speech_frames) / 12.0)
            pose_weight = fade * 0.5

        if pose_weight > 0.001:
            # Add subtle arm wave/emphasis when speaking loudly
            arm_Composite = img_point
            head_composite = (head_composite * (1.0 - arm_mask * pose_weight) + arm_Composite * (arm_mask * pose_weight)).astype(np.uint8)

        # 3. 4-frame felt eye-blinking
        if next_blink <= f < next_blink + 4:
            pct = [0.5, 1.0, 0.6, 0.2][f - next_blink]
            for c_eye in [eye_l_center, eye_r_center]:
                lid_h = int(eye_radius * 2 * pct)
                y_top = c_eye[1] - eye_radius
                cv2.ellipse(
                    head_composite,
                    (c_eye[0], y_top + lid_h // 2),
                    (eye_radius, max(1, lid_h // 2)),
                    0, 0, 360,
                    felt_eyelid_color,
                    -1,
                )
        elif f >= next_blink + 4:
            next_blink = f + int(fps * (2.4 + (f % 4) * 0.35))

        # 4. Expressive Head Movement, Nodding, and Dynamic Zoom/Sway
        nod_y = math.sin(f * 0.45) * (4.5 * min(1.0, amp * 1.8)) + math.cos(f * 0.15) * 2.0
        base_angle = -3.8 * pose_weight
        cadence_angle = math.sin(f * 0.22) * (2.5 * amp) + 1.2 * math.sin(f * 0.10)
        total_angle = base_angle + cadence_angle
        shift_x = -8.0 * pose_weight + math.sin(f * 0.14) * 3.0
        scale = 1.0 + 0.025 * amp + 0.01 * math.sin(f * 0.18)

        rot_mat = cv2.getRotationMatrix2D(head_center, total_angle, scale)
        rot_mat[0, 2] += shift_x
        rot_mat[1, 2] += nod_y
        animated_avatar = cv2.warpAffine(head_composite, rot_mat, (w, h), borderMode=cv2.BORDER_REFLECT)

        crop_size = min(animated_avatar.shape[0], animated_avatar.shape[1])
        c_avatar = animated_avatar[0:crop_size, 0:crop_size]
        pip_resized = cv2.resize(c_avatar, (PIP_SIZE, PIP_SIZE), interpolation=cv2.INTER_AREA)

        # Composite PiP into bottom-right clear zone
        bg_roi = comp_slide[PIP_Y:PIP_Y + PIP_SIZE, PIP_X:PIP_X + PIP_SIZE].astype(np.float32)
        blended_pip = (pip_resized.astype(np.float32) * pip_mask_3d + bg_roi * (1.0 - pip_mask_3d)).astype(np.uint8)
        final_pip = np.where(border_mask, pip_border, blended_pip)
        comp_slide[PIP_Y:PIP_Y + PIP_SIZE, PIP_X:PIP_X + PIP_SIZE] = final_pip

        proc.stdin.write(comp_slide.tobytes())

    proc.stdin.close()
    proc.wait()

    # Mux video with audio
    mux_cmd = [
        "ffmpeg", "-y",
        "-i", temp_raw_video,
        "-i", temp_padded_wav,
        "-c:v", "copy",
        "-c:a", "aac",
        "-b:a", "192k",
        "-shortest",
        output_segment_mp4,
    ]
    subprocess.run(mux_cmd, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

    for p in [temp_raw_video, temp_padded_wav]:
        if os.path.exists(p):
            os.remove(p)

    print(f"✅ Slide {slide_index:02d} rendered: {output_segment_mp4} ({total_duration:.2f}s)")
    return output_segment_mp4


def concatenate_segments(segment_paths: List[str], output_final_mp4: str) -> str:
    print(f"\n📦 Stitching {len(segment_paths)} segments into master presentation...")
    concat_list_path = "/tmp/clack_concat_list.txt"
    with open(concat_list_path, "w") as f:
        for p in segment_paths:
            f.write(f"file '{os.path.abspath(p)}'\n")

    cmd = [
        "ffmpeg", "-y",
        "-f", "concat",
        "-safe", "0",
        "-i", concat_list_path,
        "-c", "copy",
        output_final_mp4,
    ]
    subprocess.run(cmd, check=True)
    if os.path.exists(concat_list_path):
        os.remove(concat_list_path)
    print(f"🎉 Final presentation master ready: {output_final_mp4}")
    return output_final_mp4


def main():
    print("=== Clack-BASIC Presentation Video Production Engine v3 (Ryan + Dynamic Pointer & Relay HUD) ===")
    
    img_closed = cv2.imread(os.path.join(ASSETS_AVATAR_DIR, "user_muppet_closed.jpg"))
    img_mid = cv2.imread(os.path.join(ASSETS_AVATAR_DIR, "user_muppet_medium.jpg"))
    img_open = cv2.imread(os.path.join(ASSETS_AVATAR_DIR, "user_muppet_open.jpg"))
    img_point = cv2.imread(os.path.join(ASSETS_AVATAR_DIR, "user_muppet_point.jpg"))

    for name, img in [("closed", img_closed), ("mid", img_mid), ("open", img_open), ("point", img_point)]:
        if img is None:
            raise FileNotFoundError(f"Failed to load avatar asset: {name}")

    mouth_mask_3d = build_mouth_mask(img_closed.shape[0], img_closed.shape[1])

    segment_paths = []
    for idx in range(1, 7):
        slide_png = os.path.join(SLIDES_DIR, f"slide_{idx:02d}.png")
        audio_wav = os.path.join(AUDIO_DIR, f"slide_{idx:02d}.wav")
        segment_mp4 = os.path.join(SEGMENTS_DIR, f"segment_{idx:02d}.mp4")

        seg = render_slide_segment(
            slide_index=idx,
            slide_png_path=slide_png,
            audio_wav_path=audio_wav,
            img_closed=img_closed,
            img_mid=img_mid,
            img_open=img_open,
            img_point=img_point,
            mouth_mask_3d=mouth_mask_3d,
            output_segment_mp4=segment_mp4,
        )
        segment_paths.append(seg)

    concatenate_segments(segment_paths, FINAL_MASTER_MP4)


if __name__ == "__main__":
    main()

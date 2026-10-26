#!/usr/bin/env python3
"""
Generates narration audio for the 6 scenes of the Clack-BASIC Compiler presentation video.
Uses edge-tts with the selected voice: en-GB-RyanNeural.
"""

import os
import subprocess
import soundfile as sf
import numpy as np

SCENES = [
    {
        "index": 1,
        "name": "01_intro_architecture",
        "title": "How We Built a BASIC Compiler for a 16-Bit Relay Computer",
        "text": (
            "Welcome back! In our relay computing series, Peter is covering all the intricate relay hardware and wiring. "
            "But a 16-bit relay CPU is only as capable as the software you feed into it. "
            "Today, we're diving into the software side: how we engineered an optimizing BASIC compiler and toolchain "
            "specifically tailored to the unique architectural constraints of a physical relay computer. "
            "Unlike modern microprocessors, our machine has no hardware RAM stack and only a single-level subroutine return register. "
            "Let's look at how we conquered these limitations."
        )
    },
    {
        "index": 2,
        "name": "02_lexer_pratt_parser",
        "title": "Lexer, Pratt Parser & Abstract Syntax Tree",
        "text": (
            "At the front end of our compiler sits a handcrafted Pratt parser. "
            "Traditional recursive descent parsers often produce bloated abstract syntax trees. "
            "With our Pratt expression parser, operator precedence—from bitwise logic and arithmetic up to comparison operators—is parsed with optimal efficiency. "
            "The AST preserves full source line tracking, enabling pinpoint error messages directly in the terminal or browser. "
            "Once the AST is validated, our semantic analyzer resolves variable types and scopes before emitting single-pass assembly."
        )
    },
    {
        "index": 3,
        "name": "03_fixed_point_q88",
        "title": "High-Speed Fixed-Point Arithmetic (Q8.8)",
        "text": (
            "For real-time graphics and iterative algorithms like the Mandelbrot fractal, integers are too coarse, but floating-point can be expensive. "
            "That is why we implemented native Q8.8 fixed-point arithmetic. "
            "Using eight bits for the integer part and eight bits for the fractional part, we get a signed range of minus 128 to plus 127 point 996. "
            "Our multiplication routine handles 16-bit sign-extension with saturation, "
            "while division uses a 16-step non-restoring algorithm that executes in fewer than 90 relay cycles."
        )
    },
    {
        "index": 4,
        "name": "04_float16_zuse",
        "title": "CLACK-F16: Zuse-Style 16-Bit Floating-Point",
        "text": (
            "When you need a massive dynamic range—spanning from 10 to the minus 19 up to 10 to the plus 19—Q8.8 isn't enough. "
            "For scientific calculations, we engineered CLACK-F16, inspired directly by Konrad Zuse's Z3 computer. "
            "It uses a 1-bit sign, 7-bit exponent with bias 63, and an 8-bit mantissa with an implicit leading one. "
            "Because our hardware return stack is only one level deep, all FPU routines are written as leaf functions with zero nested calls, "
            "avoiding memory overhead while providing full IEEE-style normalization."
        )
    },
    {
        "index": 5,
        "name": "05_codegen_and_sim",
        "title": "Code Generation & Cycle-Accurate Simulator",
        "text": (
            "Here you see the compiler in action on a real demo: computing integer and fractional square roots using the Babylonian method. "
            "The compiler generates clean, human-readable Clack Assembly, complete with register allocation and symbolic jump labels. "
            "Because running tests on physical relays takes time and generates acoustic clicks, we couple the compiler directly to our cycle-accurate web simulator. "
            "You can single-step instructions, inspect relay bus lines, and verify every ALU flag before sending code to the physical hardware."
        )
    },
    {
        "index": 6,
        "name": "06_wasm_and_outro",
        "title": "WebAssembly Playground & Community Outro",
        "text": (
            "Everything we built is completely open-source. "
            "You can try the Clack-BASIC compiler right in your web browser via our WebAssembly build at relay-computer dot org. "
            "In the next video, we will explore Clack Studio, our dedicated IDE with integrated syntax highlighting and live relay telemetry. "
            "Be sure to check out Peter's hardware videos linked below to see the physical machine clicking in real life. "
            "Thanks for watching, and see you next time!"
        )
    },
]

AUDIO_DIR = "/home/veitscherbub/work/ai/cinematic-video-pipeline/projects/clack_compiler_video/audio"
VOICE = "en-GB-RyanNeural"

def main():
    os.makedirs(AUDIO_DIR, exist_ok=True)
    print(f"Generating {len(SCENES)} narration audio tracks using {VOICE}...")
    
    for scene in SCENES:
        idx = scene["index"]
        mp3_out = os.path.join(AUDIO_DIR, f"slide_{idx:02d}.mp3")
        wav_out = os.path.join(AUDIO_DIR, f"slide_{idx:02d}.wav")
        
        print(f"\n[Scene {idx}] {scene['title']}")
        # Call edge-tts
        cmd = [
            "uvx", "edge-tts",
            "--voice", VOICE,
            "--text", scene["text"],
            "--write-media", mp3_out
        ]
        res = subprocess.run(cmd, capture_output=True, text=True)
        if res.returncode != 0:
            print(f"Error generating audio for scene {idx}: {res.stderr}")
            continue
            
        # Convert MP3 to 48kHz 16-bit WAV
        ffmpeg_cmd = [
            "ffmpeg", "-y",
            "-i", mp3_out,
            "-ar", "48000",
            "-ac", "1",
            wav_out
        ]
        subprocess.run(ffmpeg_cmd, capture_output=True, check=True)
        
        # Check duration
        data, sr = sf.read(wav_out)
        duration = len(data) / float(sr)
        print(f"-> Generated {wav_out}: {duration:.2f} seconds ({len(data)} samples)")

if __name__ == "__main__":
    main()

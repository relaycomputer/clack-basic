# 🌐 Das 16-Bit Relais-Computer Ökosystem

Das Relais-Computing-Projekt ist in **3 eigenständige, perfekt aufeinander abgestimmte Repositories** unterteilt:

```mermaid
flowchart TD
    subgraph Repo1["1. memory-subsystem (Hardware & Firmware)"]
        M1["Arduino Mega 2560 C++ Firmware (SRAM & MMIO)"]
        M2["Hardware-Schaltpläne & Pinbelegung"]
        M3["Low-Level Upload-Werkzeuge (Go & Python)"]
    end

    subgraph Repo2["2. clack-basic (Compiler-Suite)"]
        B1["Cross-Compiler Core (Lexer, Parser, Codegen, Emitter)"]
        B2["bin/clack (Kommandozeilen-Werkzeug)"]
        B3["Beispielprogramme (ping_pong.bas, etc.)"]
    end

    subgraph Repo3["3. clack-studio (Visual Lab & IDE)"]
        S1["Svelte 5 + Vite Web-Frontend (Zero-DOM-Overhead)"]
        S2["64x64 / 32x32 LED Dot-Matrix Display"]
        S3["Hardware-Frontpanel & Relais-Sound"]
        S4["Integrierter Arduino Mega Web-Flasher"]
    end

    Repo1 <-->|USB 115200 Baud / Hardware-Bus| Repo3
    Repo2 -->|Kompiliert Maschinencode| Repo1
    Repo2 -->|Compiler-Bibliothek| Repo3
```

---

## 🏛️ Rollenverteilung

1. **`memory-subsystem` (Hardware & Firmware):**
   * Stellt den 1024-Wort RAM für den physischen Relaisrechner über 41 GPIO-Leitungen bereit.
   * Empfängt Maschinencode über USB mit 115200 Baud.
   * Überträgt Framebuffer-Änderungen an das 4-Display LED-Matrix-Subsystem.

2. **`clack-basic` (Compiler-Suite):**
   * Übersetzt menschenlesbaren BASIC-Code in optimierten 16-Bit Relais-Maschinencode.
   * Bietet ein mächtiges Standalone-CLI (`clack compile`, `clack flash`, `clack run`).

3. **`clack-studio` (Visual Hardware Lab & IDE):**
   * Die All-in-One Entwicklungs- und Simulationsumgebung im Browser.
   * Visualisiert alle Register, Busse und Displays live und flasht auf Knopfdruck auf die echte Hardware.

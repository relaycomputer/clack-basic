package main

/*
 ==============================================================================
 ⚡ CLACK-BASIC: 16-BIT RELAISRECHNER CROSS-COMPILER
 ==============================================================================
 Übersetzt höhere BASIC-Programme (*.bas) in Maschinencode für den 16-Bit
 Relaisrechner und unterstützt den direkten Upload auf den Arduino Mega.
 ==============================================================================
*/

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"clack-basic/pkg/codegen"
	"clack-basic/pkg/emitter"
	"clack-basic/pkg/lexer"
	"clack-basic/pkg/parser"

	"go.bug.st/serial"
)

const (
	Version = "1.0.0"
)

func main() {
	if len(os.Args) < 2 {
		printBanner()
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]

	switch cmd {
	case "compile", "build", "c":
		handleCompile(os.Args[2:])
	case "flash", "upload", "run":
		handleFlash(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("⚡ Clack-BASIC Cross-Compiler Version %s\n", Version)
	case "help", "-h", "--help":
		printBanner()
		printUsage()
	default:
		// Wenn kein Unterbefehl angegeben wurde, sondern direkt eine Datei:
		if strings.HasSuffix(cmd, ".bas") || strings.HasSuffix(cmd, ".txt") {
			handleCompile(os.Args[1:])
		} else {
			fmt.Printf("Unbekannter Befehl: '%s'\n\n", cmd)
			printUsage()
			os.Exit(1)
		}
	}
}

func handleCompile(args []string) {
	fs := flag.NewFlagSet("compile", flag.ExitOnError)
	outFile := fs.String("o", "", "Ausgabedatei (Standard: <eingabe>.txt)")
	formatStr := fs.String("format", "arduino", "Ausgabeformat: 'arduino', 'c-array' oder 'listing'")
	fs.Parse(args)

	files := fs.Args()
	if len(files) < 1 {
		fmt.Println("[!] FEHLER: Keine BASIC-Quelldatei (.bas) angegeben.")
		fmt.Println("    Verwendung: clack compile <datei.bas> [-o output.txt] [--format=arduino|c-array|listing]")
		os.Exit(1)
	}
	inputFile := files[0]

	// 1. Quellcode einlesen
	content, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Printf("[!] FEHLER beim Lesen von '%s': %v\n", inputFile, err)
		os.Exit(1)
	}

	fmt.Printf("[*] Kompiliere '%s' mit Clack-BASIC...\n", inputFile)
	startTime := time.Now()

	// 2. Lexer & Tokenizer
	l := lexer.NewLexer(string(content))
	tokens := l.Tokenize()

	// 3. Parser & AST
	p := parser.NewParser(tokens)
	prog, err := p.ParseProgram()
	if err != nil {
		fmt.Printf("[!] KOMPILIERFEHLER:\n%v\n", err)
		os.Exit(1)
	}

	// 4. Code-Generator
	cg := codegen.NewCodeGenerator()
	memory, comments, err := cg.Generate(prog)
	if err != nil {
		fmt.Printf("[!] CODE-GENERIERUNGSFEHLER:\n%v\n", err)
		os.Exit(1)
	}

	duration := time.Since(startTime)

	// 5. Emitter
	format := emitter.OutputFormat(*formatStr)
	outText := emitter.Emit(memory, comments, format)

	// Ausgabepfad ermitteln
	targetFile := *outFile
	if targetFile == "" {
		ext := filepath.Ext(inputFile)
		base := strings.TrimSuffix(inputFile, ext)
		targetFile = base + ".txt"
	}

	if err := os.WriteFile(targetFile, []byte(outText), 0644); err != nil {
		fmt.Printf("[!] FEHLER beim Schreiben von '%s': %v\n", targetFile, err)
		os.Exit(1)
	}

	// Statistik berechnen
	usedWords := 0
	for _, w := range memory {
		if w != 0 {
			usedWords++
		}
	}

	fmt.Printf("[+] ERFOLG: In %.2f ms kompiliert!\n", float64(duration.Microseconds())/1000.0)
	fmt.Printf("[+] Speicherbelegung: %d / %d Worte (%.1f%%)\n", usedWords, codegen.MaxMemorySize, float64(usedWords)/float64(codegen.MaxMemorySize)*100.0)
	fmt.Printf("[+] Maschinencode geschrieben nach: %s (Format: %s)\n", targetFile, format)
}

func handleFlash(args []string) {
	fs := flag.NewFlagSet("flash", flag.ExitOnError)
	portFlag := fs.String("p", "", "Serieller Port (z.B. /dev/ttyACM0 oder COM3). Standard: Auto-Erkennung")
	baudFlag := fs.Int("b", 115200, "Baudrate (Standard: 115200)")
	monitorFlag := fs.Bool("m", false, "Nach dem Upload im interaktiven Monitor bleiben")
	fs.Parse(args)

	files := fs.Args()
	if len(files) < 1 {
		fmt.Println("[!] FEHLER: Keine BASIC-Quelldatei (.bas) zum Flashen angegeben.")
		os.Exit(1)
	}
	inputFile := files[0]

	// 1. Kompilieren in Arduino-Format
	content, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Printf("[!] FEHLER: %v\n", err)
		os.Exit(1)
	}

	l := lexer.NewLexer(string(content))
	tokens := l.Tokenize()
	p := parser.NewParser(tokens)
	prog, err := p.ParseProgram()
	if err != nil {
		fmt.Printf("[!] KOMPILIERFEHLER: %v\n", err)
		os.Exit(1)
	}

	cg := codegen.NewCodeGenerator()
	memory, comments, err := cg.Generate(prog)
	if err != nil {
		fmt.Printf("[!] CODE-GENERIERUNGSFEHLER: %v\n", err)
		os.Exit(1)
	}

	arduinoCode := emitter.Emit(memory, comments, emitter.FormatArduino)

	// 2. Port ermitteln
	selectedPort := *portFlag
	if selectedPort == "" {
		detected, err := autoDetectPort()
		if err != nil {
			fmt.Printf("[!] Port-Erkennung: %v\n", err)
			os.Exit(1)
		}
		selectedPort = detected
	}

	fmt.Printf("[*] Verbinde mit Arduino auf %s (%d Baud)...\n", selectedPort, *baudFlag)
	mode := &serial.Mode{
		BaudRate: *baudFlag,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(selectedPort, mode)
	if err != nil {
		fmt.Printf("[!] FEHLER beim Oeffnen von %s: %v\n", selectedPort, err)
		os.Exit(1)
	}
	defer port.Close()

	// Reset via DTR
	_ = port.SetDTR(false)
	time.Sleep(100 * time.Millisecond)
	_ = port.SetDTR(true)
	time.Sleep(2 * time.Second)

	fmt.Println("[*] Uebertrage Maschinencode an Relaisrechner-Speichersubsystem...")
	lines := strings.Split(arduinoCode, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		_, _ = port.Write([]byte(trimmed + "\n"))
		time.Sleep(5 * time.Millisecond)
		fmt.Printf("\r[*] Upload: %3d / %3d Zeilen...", i+1, len(lines))
	}

	fmt.Printf("\n[+] ERFOLG: Programm erfolgreich auf Relaisrechner geflasht!\n")

	if *monitorFlag {
		fmt.Println("[*] Starte interaktiven Monitor (Beenden mit Strg+C)...")
		buf := make([]byte, 256)
		for {
			_ = port.SetReadTimeout(200 * time.Millisecond)
			n, err := port.Read(buf)
			if err != nil {
				break
			}
			if n > 0 {
				os.Stdout.Write(buf[:n])
			}
		}
	}
}

func autoDetectPort() (string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return "", err
	}
	for _, name := range ports {
		desc := strings.ToLower(name)
		if strings.Contains(desc, "arduino") || strings.Contains(desc, "ch340") || strings.Contains(desc, "cp210") || strings.Contains(desc, "ftdi") || strings.Contains(desc, "ttyacm") || strings.Contains(desc, "ttyusb") || strings.Contains(desc, "usbmodem") || strings.Contains(desc, "usbserial") || strings.Contains(desc, "com") {
			return name, nil
		}
	}
	if len(ports) > 0 {
		return ports[0], nil
	}
	return "", fmt.Errorf("kein serieller Port gefunden")
}

func printBanner() {
	fmt.Println(`
  ██████╗██╗      █████╗  ██████╗██╗  ██╗   ██████╗  █████╗ ███████╗██╗ ██████╗
 ██╔════╝██║     ██╔══██╗██╔════╝██║ ██╔╝   ██╔══██╗██╔══██╗██╔════╝██║██╔════╝
 ██║     ██║     ███████║██║     █████═╝    ██████╔╝███████║███████╗██║██║     
 ██║     ██║     ██╔══██║██║     ██╔═██╗    ██╔══██╗██╔══██║╚════██║██║██║     
 ╚██████╗███████╗██║  ██║╚██████╗██║ ╚██╗   ██████╔╝██║  ██║███████║██║╚██████╗
  ╚═════╝╚══════╝╚═╝  ╚═╝ ╚═════╝╚═╝  ╚═╝   ╚═════╝ ╚═╝  ╚═╝╚══════╝╚═╝ ╚═════╝`)
	fmt.Printf("   16-Bit Relay Computer Cross-Compiler • Version %s\n", Version)
	fmt.Println("================================================================================")
}

func printUsage() {
	fmt.Println("Verwendung:")
	fmt.Println("  clack compile <datei.bas> [Optionen]   Kompiliert BASIC-Code in Maschinencode")
	fmt.Println("  clack flash <datei.bas> [Optionen]     Kompiliert und flasht direkt auf den Arduino")
	fmt.Println()
	fmt.Println("Kompilier-Optionen:")
	fmt.Println("  -o <Ausgabedatei>      Pfad zur Ausgabedatei (Standard: <eingabe>.txt)")
	fmt.Println("  --format=<Format>      Ausgabeformat:")
	fmt.Println("                           'arduino'  (Standard: fuer upload.py / relay-upload)")
	fmt.Println("                           'c-array'  (daten[0] = ...; fuer C/C++ Code)")
	fmt.Println("                           'listing'  (Assembler-Listing mit Disassembly)")
	fmt.Println()
	fmt.Println("Flash-Optionen:")
	fmt.Println("  -p <Port>              Serieller Port (Standard: Auto-Erkennung)")
	fmt.Println("  -b <Baud>              Baudrate (Standard: 115200)")
	fmt.Println("  -m                     Nach dem Flashen im interaktiven Monitor bleiben")
	fmt.Println()
	fmt.Println("Beispiele:")
	fmt.Println("  clack compile examples/math_test.bas")
	fmt.Println("  clack compile examples/graphics_demo.bas --format=c-array")
	fmt.Println("  clack flash examples/dijkstra.bas -m")
}

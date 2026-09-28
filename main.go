package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"clack-basic/pkg/assembler"
	"clack-basic/pkg/codegen"
	"clack-basic/pkg/decompiler"
	"clack-basic/pkg/emitter"
	"clack-basic/pkg/lexer"
	"clack-basic/pkg/parser"

	"go.bug.st/serial"
)

const (
	Version = "1.4.0"
)

type CmdOptions struct {
	InputFile   string
	OutputFile  string
	Format      string
	Port        string
	Baud        int
	StayMonitor bool
}

func parseOptions(args []string) CmdOptions {
	opts := CmdOptions{
		Format: "arduino",
		Baud:   115200,
	}
	var positional []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-o" && i+1 < len(args) {
			opts.OutputFile = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "-o=") {
			opts.OutputFile = strings.TrimPrefix(arg, "-o=")
		} else if (arg == "-format" || arg == "--format") && i+1 < len(args) {
			opts.Format = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "--format=") {
			opts.Format = strings.TrimPrefix(arg, "--format=")
		} else if strings.HasPrefix(arg, "-format=") {
			opts.Format = strings.TrimPrefix(arg, "-format=")
		} else if arg == "-p" && i+1 < len(args) {
			opts.Port = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "-p=") {
			opts.Port = strings.TrimPrefix(arg, "-p=")
		} else if arg == "-b" && i+1 < len(args) {
			if b, err := strconv.Atoi(args[i+1]); err == nil && b > 0 {
				opts.Baud = b
			}
			i++
		} else if strings.HasPrefix(arg, "-b=") {
			if b, err := strconv.Atoi(strings.TrimPrefix(arg, "-b=")); err == nil && b > 0 {
				opts.Baud = b
			}
		} else if arg == "-m" || arg == "--monitor" {
			opts.StayMonitor = true
		} else if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
		}
	}

	if len(positional) > 0 {
		opts.InputFile = positional[0]
	}
	return opts
}

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
	case "asm", "assemble", "a":
		handleAssemble(os.Args[2:])
	case "decompile", "decomp", "d":
		handleDecompile(os.Args[2:])
	case "flash", "upload", "run":
		handleFlash(os.Args[2:])
	case "version", "-v", "--version":
		fmt.Printf("⚡ Clack-BASIC Cross-Compiler & Assembler Toolkit Version %s\n", Version)
	case "help", "-h", "--help":
		printBanner()
		printUsage()
	default:
		if strings.HasSuffix(cmd, ".bas") || strings.HasSuffix(cmd, ".txt") {
			handleCompile(os.Args[1:])
		} else if strings.HasSuffix(cmd, ".asm") {
			handleAssemble(os.Args[1:])
		} else {
			fmt.Printf("Unbekannter Befehl: '%s'\n\n", cmd)
			printUsage()
			os.Exit(1)
		}
	}
}

func handleCompile(args []string) {
	opts := parseOptions(args)
	if opts.InputFile == "" {
		fmt.Println("[!] FEHLER: Keine BASIC-Quelldatei (.bas) angegeben.")
		fmt.Println("    Verwendung: clack compile <datei.bas> [-o output.txt] [--format=arduino|c-array|listing]")
		os.Exit(1)
	}

	content, err := os.ReadFile(opts.InputFile)
	if err != nil {
		fmt.Printf("[!] FEHLER beim Lesen von '%s': %v\n", opts.InputFile, err)
		os.Exit(1)
	}

	fmt.Printf("[*] Kompiliere '%s' mit Clack-BASIC...\n", opts.InputFile)
	startTime := time.Now()

	l := lexer.NewLexer(string(content))
	tokens := l.Tokenize()

	p := parser.NewParser(tokens)
	prog, err := p.ParseProgram()
	if err != nil {
		fmt.Printf("[!] KOMPILIERFEHLER:\n%v\n", err)
		os.Exit(1)
	}

	cg := codegen.NewCodeGenerator()
	mem, comments, err := cg.Generate(prog)
	if err != nil {
		fmt.Printf("[!] CODE-GENERIERUNGSFEHLER:\n%v\n", err)
		os.Exit(1)
	}

	var outputFormat emitter.OutputFormat
	var ext string
	switch strings.ToLower(opts.Format) {
	case "c-array", "c", "cpp":
		outputFormat = emitter.FormatCArray
		ext = ".txt"
	case "listing", "lst", "disasm":
		outputFormat = emitter.FormatListing
		ext = ".lst"
	case "arduino", "hex", "txt":
		fallthrough
	default:
		outputFormat = emitter.FormatArduino
		ext = ".txt"
	}

	outputContent := emitter.Emit(mem, comments, outputFormat)

	usedCount := 0
	for _, val := range mem {
		if val != 0 {
			usedCount++
		}
	}

	outFile := opts.OutputFile
	if outFile == "" {
		base := strings.TrimSuffix(opts.InputFile, filepath.Ext(opts.InputFile))
		outFile = base + ext
	}

	if err := os.WriteFile(outFile, []byte(outputContent), 0644); err != nil {
		fmt.Printf("[!] FEHLER beim Schreiben nach '%s': %v\n", outFile, err)
		os.Exit(1)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("[+] ERFOLG: In %.2f ms kompiliert!\n", float64(elapsed.Microseconds())/1000.0)
	fmt.Printf("[+] Speicherbelegung: %d / 1024 Worte (%.1f%%)\n", usedCount, float64(usedCount)/1024.0*100.0)
	fmt.Printf("[+] Maschinencode geschrieben nach: %s (Format: %s)\n", outFile, opts.Format)
}

func handleAssemble(args []string) {
	opts := parseOptions(args)
	if opts.InputFile == "" {
		fmt.Println("[!] FEHLER: Keine Assembler-Datei (.asm) angegeben.")
		fmt.Println("    Verwendung: clack asm <datei.asm> [-o output.txt] [--format=arduino|c-array]")
		os.Exit(1)
	}

	content, err := os.ReadFile(opts.InputFile)
	if err != nil {
		fmt.Printf("[!] FEHLER beim Lesen von '%s': %v\n", opts.InputFile, err)
		os.Exit(1)
	}

	fmt.Printf("[*] Assembliere '%s' mit Clack-Assembler...\n", opts.InputFile)
	startTime := time.Now()

	asm := assembler.NewAssembler()
	mem, used, err := asm.Assemble(string(content))
	if err != nil {
		fmt.Printf("[!] ASSEMBLER-FEHLER:\n%v\n", err)
		os.Exit(1)
	}

	usedCount := 0
	for _, u := range used {
		if u {
			usedCount++
		}
	}

	var outputContent string
	if strings.ToLower(opts.Format) == "c-array" {
		outputContent = asm.FormatCArrayOutput()
	} else {
		outputContent = asm.FormatArduinoOutput()
	}

	outFile := opts.OutputFile
	if outFile == "" {
		base := strings.TrimSuffix(opts.InputFile, filepath.Ext(opts.InputFile))
		outFile = base + ".txt"
	}

	if err := os.WriteFile(outFile, []byte(outputContent), 0644); err != nil {
		fmt.Printf("[!] FEHLER beim Schreiben nach '%s': %v\n", outFile, err)
		os.Exit(1)
	}

	elapsed := time.Since(startTime)
	_ = mem
	fmt.Printf("[+] ERFOLG: In %.2f ms assembliert!\n", float64(elapsed.Microseconds())/1000.0)
	fmt.Printf("[+] Speicherbelegung: %d Worte\n", usedCount)
	fmt.Printf("[+] Maschinencode geschrieben nach: %s\n", outFile)
}

func handleDecompile(args []string) {
	opts := parseOptions(args)
	if opts.InputFile == "" {
		fmt.Println("[!] FEHLER: Keine Maschinencode- oder Assembler-Datei angegeben.")
		fmt.Println("    Verwendung: clack decompile <datei.txt|datei.asm> [-o output.bas]")
		os.Exit(1)
	}

	content, err := os.ReadFile(opts.InputFile)
	if err != nil {
		fmt.Printf("[!] FEHLER beim Lesen von '%s': %v\n", opts.InputFile, err)
		os.Exit(1)
	}

	fmt.Printf("[*] Dekompiliere '%s' nach Clack-BASIC...\n", opts.InputFile)
	startTime := time.Now()

	decomp := decompiler.NewDecompiler()
	var basicCode string

	if strings.HasSuffix(strings.ToLower(opts.InputFile), ".asm") {
		asm := assembler.NewAssembler()
		mem, used, err := asm.Assemble(string(content))
		if err != nil {
			fmt.Printf("[!] FEHLER beim Assemblieren: %v\n", err)
			os.Exit(1)
		}
		decomp.SetMemory(mem, used[:])
		basicCode, err = decomp.Decompile()
		if err != nil {
			fmt.Printf("[!] DEKOMPILIER-FEHLER: %v\n", err)
			os.Exit(1)
		}
	} else {
		err := decomp.LoadSource(string(content))
		if err != nil {
			asm := assembler.NewAssembler()
			mem, used, asmErr := asm.Assemble(string(content))
			if asmErr == nil {
				decomp.SetMemory(mem, used[:])
			} else {
				fmt.Printf("[!] FEHLER beim Parsen der Eingabedatei:\n%v\n", err)
				os.Exit(1)
			}
		}
		basicCode, err = decomp.Decompile()
		if err != nil {
			fmt.Printf("[!] DEKOMPILIER-FEHLER: %v\n", err)
			os.Exit(1)
		}
	}

	outFile := opts.OutputFile
	if outFile == "" {
		base := strings.TrimSuffix(opts.InputFile, filepath.Ext(opts.InputFile))
		outFile = base + ".bas"
	}

	if err := os.WriteFile(outFile, []byte(basicCode), 0644); err != nil {
		fmt.Printf("[!] FEHLER beim Schreiben nach '%s': %v\n", outFile, err)
		os.Exit(1)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("[+] ERFOLG: In %.2f ms nach Clack-BASIC dekompiliert!\n", float64(elapsed.Microseconds())/1000.0)
	fmt.Printf("[+] BASIC-Quellcode geschrieben nach: %s\n", outFile)
}

func handleFlash(args []string) {
	opts := parseOptions(args)
	if opts.InputFile == "" {
		fmt.Println("[!] FEHLER: Keine Datei angegeben.")
		fmt.Println("    Verwendung: clack flash <datei.bas|datei.asm|datei.txt> [-p /dev/ttyACM0] [-b 115200] [-m]")
		os.Exit(1)
	}

	content, err := os.ReadFile(opts.InputFile)
	if err != nil {
		fmt.Printf("[!] FEHLER beim Lesen von '%s': %v\n", opts.InputFile, err)
		os.Exit(1)
	}

	var arduinoLines string
	if strings.HasSuffix(opts.InputFile, ".bas") {
		fmt.Printf("[*] Kompiliere '%s' fuer Arduino...\n", opts.InputFile)
		l := lexer.NewLexer(string(content))
		tokens := l.Tokenize()
		p := parser.NewParser(tokens)
		prog, err := p.ParseProgram()
		if err != nil {
			fmt.Printf("[!] KOMPILIERFEHLER:\n%v\n", err)
			os.Exit(1)
		}
		cg := codegen.NewCodeGenerator()
		mem, comments, err := cg.Generate(prog)
		if err != nil {
			fmt.Printf("[!] CODEGEN-FEHLER:\n%v\n", err)
			os.Exit(1)
		}
		arduinoLines = emitter.Emit(mem, comments, emitter.FormatArduino)
	} else if strings.HasSuffix(opts.InputFile, ".asm") {
		fmt.Printf("[*] Assembliere '%s' fuer Arduino...\n", opts.InputFile)
		asm := assembler.NewAssembler()
		_, _, err := asm.Assemble(string(content))
		if err != nil {
			fmt.Printf("[!] ASSEMBLER-FEHLER:\n%v\n", err)
			os.Exit(1)
		}
		arduinoLines = asm.FormatArduinoOutput()
	} else {
		arduinoLines = string(content)
	}

	targetPort := opts.Port
	if targetPort == "" {
		fmt.Println("[*] Suche automatisch nach angeschlossenem Arduino...")
		autoPort, err := autoDetectPort()
		if err != nil {
			fmt.Printf("[!] Port-Erkennung fehlgeschlagen: %v\n", err)
			fmt.Println("    Bitte Port manuell angeben: clack flash <datei> -p <Port>")
			os.Exit(1)
		}
		targetPort = autoPort
		fmt.Printf("[+] Arduino gefunden an: %s\n", targetPort)
	}

	fmt.Printf("[*] Oeffne seriellen Port '%s' mit %d Baud...\n", targetPort, opts.Baud)
	mode := &serial.Mode{
		BaudRate: opts.Baud,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	portConn, err := serial.Open(targetPort, mode)
	if err != nil {
		fmt.Printf("[!] FEHLER beim Oeffnen des Ports: %v\n", err)
		os.Exit(1)
	}
	defer portConn.Close()

	time.Sleep(1500 * time.Millisecond)

	scanner := bufio.NewScanner(strings.NewReader(arduinoLines))
	lineCount := 0
	fmt.Println("[*] Sende Maschinencode an Relaiscomputer-Speicher...")
	startTime := time.Now()

	portConn.Write([]byte("CLR\r\n"))
	time.Sleep(50 * time.Millisecond)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, ";") {
			continue
		}
		_, err := portConn.Write([]byte(line + "\r\n"))
		if err != nil {
			fmt.Printf("[!] Fehler beim Senden: %v\n", err)
			os.Exit(1)
		}
		lineCount++
		time.Sleep(3 * time.Millisecond)
	}

	portConn.Write([]byte("RUN\r\n"))
	elapsed := time.Since(startTime)

	fmt.Printf("[+] ERFOLG: %d Befehlsworte in %.2f s uebertragen!\n", lineCount, elapsed.Seconds())
	fmt.Println("[+] Relaisrechner laeuft!")

	if opts.StayMonitor {
		fmt.Println("\n[*] Serieller Monitor aktiv (Beenden mit Strg+C)...")
		buf := make([]byte, 128)
		for {
			n, err := portConn.Read(buf)
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
	fmt.Printf("   16-Bit Relay Computer Cross-Compiler & Toolkit • Version %s\n", Version)
	fmt.Println("================================================================================")
}

func printUsage() {
	fmt.Println("Verwendung:")
	fmt.Println("  clack compile <datei.bas> [Optionen]    Kompiliert BASIC-Code in Maschinencode")
	fmt.Println("  clack asm <datei.asm> [Optionen]        Assembliert Assembler-Code in Maschinencode")
	fmt.Println("  clack decompile <datei.txt|asm> [-o ...]  Dekompiliert Maschinencode/ASM nach BASIC")
	fmt.Println("  clack flash <datei> [Optionen]          Flasht BASIC, ASM oder Maschinencode auf Arduino")
	fmt.Println()
	fmt.Println("Optionen:")
	fmt.Println("  -o <Ausgabedatei>      Pfad zur Ausgabedatei")
	fmt.Println("  --format=<Format>      Ausgabeformat: 'arduino', 'c-array', 'listing'")
	fmt.Println("  -p <Port>              Serieller Port (z.B. /dev/ttyACM0 oder COM3)")
	fmt.Println("  -m                     Im seriellen Monitor bleiben")
	fmt.Println()
	fmt.Println("Beispiele:")
	fmt.Println("  clack compile examples/ping_pong.bas -o ping_pong.txt")
	fmt.Println("  clack asm examples/collatz.asm -o collatz.txt")
	fmt.Println("  clack decompile dijkstra.txt -o dijkstra.bas")
	fmt.Println("  clack flash examples/ping_pong.bas -m")
}

// See https://aka.ms/new-console-template for more information
using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Linq.Expressions;
using System.Text.RegularExpressions;

namespace DataParser
{
    class Program
    {
        const int buffersize = 10000;

        static ushort[] historyPC = new ushort[buffersize];
        static ushort[] historyAccu = new ushort[buffersize];
        static ushort[] historyAdressbus = new ushort[buffersize];
        static ushort[] historyDatabus = new ushort[buffersize];
        static ushort[] historyReads = new ushort[buffersize];
        static ushort[] historyWrites = new ushort[buffersize];
        static ushort[] historyFlags = new ushort[buffersize];
        static ushort[] historyStack = new ushort[buffersize];
        static ushort[] historyCmd = new ushort[buffersize];
        static ushort[] historyCycles = new ushort[buffersize];


        static int runIndex = 1;

        static int cycletime = 70;

        static void runProgram(int[] prog)
        {
            // Initialisierung aller Register und Flags
            ushort pc = 0,
                   accu = 0,
                   stack = 0,
                   address = 0,
                   databus = 0,
                   carryF = 0,
                   addr = 0;

            bool runActive = true;
            historyCycles[0] = 0;

            try
            {
                while (runActive && pc < prog.Length)
                {
                    ushort cmd = (ushort)prog[pc];
                    switch (cmd)
                    {
                        case 0xF000:        // NOP
                            historyCmd[runIndex] = cmd;
                            historyPC[runIndex] = pc;
                            historyAccu[runIndex] = accu;
                            historyAdressbus[runIndex] = 0;
                            historyDatabus[runIndex] = 0;
                            historyReads[runIndex] = 0;
                            historyWrites[runIndex] = 0;
                            historyFlags[runIndex] = carryF;
                            historyStack[runIndex] = stack;
                            pc++;
                            historyCycles[runIndex] = (ushort) (historyCycles[runIndex-1] + 2);
                            break;

                        case 0x1400:        // LDA
                            accu = (ushort)prog[pc + 1];
                            historyCmd[runIndex] = cmd;
                            historyPC[runIndex] = pc;
                            historyAccu[runIndex] = accu;
                            historyAdressbus[runIndex] = (ushort)(pc + 1);
                            historyDatabus[runIndex] = accu;
                            historyReads[runIndex] = accu;
                            historyWrites[runIndex] = 0;
                            historyFlags[runIndex] = carryF;
                            historyStack[runIndex] = stack;
                            pc += 2;
                            historyCycles[runIndex] = (ushort)(historyCycles[runIndex - 1] + 4);
                            break;

                        case 0x1800:        // LDAM
                            addr = (ushort)prog[pc + 1];
                            accu = (ushort)prog[addr];
                            historyCmd[runIndex] = cmd;
                            historyPC[runIndex] = pc;
                            historyAccu[runIndex] = accu;
                            historyAdressbus[runIndex] = addr;
                            historyDatabus[runIndex] = accu;
                            historyReads[runIndex] = accu;
                            historyWrites[runIndex] = 0;
                            historyFlags[runIndex] = carryF;
                            historyStack[runIndex] = stack;
                            pc += 2;
                            historyCycles[runIndex] = (ushort)(historyCycles[runIndex - 1] + 6);
                            break;

                        case 0x1C00:        // LDAMM
                            addr = (ushort)prog[prog[pc + 1]];
                            accu = (ushort)prog[addr];
                            historyCmd[runIndex] = cmd;
                            historyPC[runIndex] = pc;
                            historyAccu[runIndex] = accu;
                            historyAdressbus[runIndex] = addr;
                            historyDatabus[runIndex] = accu;
                            historyReads[runIndex] = accu;
                            historyWrites[runIndex] = 0;
                            historyFlags[runIndex] = carryF;
                            historyStack[runIndex] = stack;
                            pc += 2;
                            historyCycles[runIndex] = (ushort)(historyCycles[runIndex - 1] + 8);
                            break;

                        case 0x2800:        // STAM
                            prog[prog[pc + 1]] = accu;
                            historyCmd[runIndex] = cmd;
                            historyPC[runIndex] = pc;
                            historyAccu[runIndex] = accu;
                            historyAdressbus[runIndex] = (ushort)prog[prog[pc + 1]];
                            historyDatabus[runIndex] = accu;
                            historyReads[runIndex] = 0;
                            historyWrites[runIndex] = accu;
                            historyFlags[runIndex] = carryF;
                            historyStack[runIndex] = stack;
                            pc += 2;
                            historyCycles[runIndex] = (ushort)(historyCycles[runIndex - 1] + 6);
                            break;

                        case 0x2C00:        // STAMM
                            prog[prog[prog[pc + 1]]] = accu;
                            historyCmd[runIndex] = cmd;
                            historyPC[runIndex] = pc;
                            historyAccu[runIndex] = accu;
                            historyAdressbus[runIndex] = (ushort)prog[prog[prog[pc + 1]]];
                            historyDatabus[runIndex] = accu;
                            historyReads[runIndex] = 0;
                            historyWrites[runIndex] = accu;
                            historyFlags[runIndex] = carryF;
                            historyStack[runIndex] = stack;
                            pc += 2;
                            historyCycles[runIndex] = (ushort)(historyCycles[runIndex - 1] + 8);
                            break;

                        case 0x3440:        // ADD
                            if ((int)(accu + prog[pc + 1]) > 0xFFFF)
                                carryF = 1;
                            else
                                carryF = 0;

                            accu = (ushort)(accu + prog[pc + 1]);
                            historyCmd[runIndex] = cmd;
                            historyPC[runIndex] = pc;
                            historyAccu[runIndex] = accu;
                            historyAdressbus[runIndex] = (ushort)(pc + 1);
                            historyDatabus[runIndex] = (ushort)(prog[pc + 1]);
                            historyReads[runIndex] = (ushort)(prog[pc + 1]);
                            historyWrites[runIndex] = 0;
                            historyFlags[runIndex] = carryF;
                            historyStack[runIndex] = stack;
                            pc += 2;
                            historyCycles[runIndex] = (ushort)(historyCycles[runIndex - 1] + 4);
                            break;

                        case 0x3480:        // AND
                            carryF = 0;
                            accu = (ushort)(accu & prog[pc + 1]);
                            historyCmd[runIndex] = cmd;
                            historyPC[runIndex] = pc;
                            historyAccu[runIndex] = accu;
                            historyAdressbus[runIndex] = (ushort)(pc + 1);
                            historyDatabus[runIndex] = (ushort)(prog[pc + 1]);
                            historyReads[runIndex] = (ushort)(prog[pc + 1]);
                            historyWrites[runIndex] = 0;
                            historyFlags[runIndex] = carryF;
                            historyStack[runIndex] = stack;
                            pc += 2;
                            historyCycles[runIndex] = (ushort)(historyCycles[runIndex - 1] + 4);
                            break;

                        case 0x34C0:        // OR
                            carryF = 0;
                            accu = (ushort)(accu | prog[pc + 1]);
                            historyCmd[runIndex] = cmd;
                            historyPC[runIndex] = pc;
                            historyAccu[runIndex] = accu;
                            historyAdressbus[runIndex] = (ushort)(pc + 1);
                            historyDatabus[runIndex] = (ushort)(prog[pc + 1]);
                            historyReads[runIndex] = (ushort)(prog[pc + 1]);
                            historyWrites[runIndex] = 0;
                            historyFlags[runIndex] = carryF;
                            historyStack[runIndex] = stack;
                            pc += 2;
                            historyCycles[runIndex] = (ushort)(historyCycles[runIndex - 1] + 4);
                            break;

                        case 0x3540:        // NOT
                            carryF = 0;
                            accu = (ushort)(~accu);
                            historyCmd[runIndex] = cmd;
                            historyPC[runIndex] = pc;
                            historyAccu[runIndex] = accu;
                            historyAdressbus[runIndex] = 0;
                            historyDatabus[runIndex] = 0;
                            historyReads[runIndex] = 0;
                            historyWrites[runIndex] = 0;
                            historyFlags[runIndex] = carryF;
                            historyStack[runIndex] = stack;
                            pc += 2;
                            historyCycles[runIndex] = (ushort)(historyCycles[runIndex - 1] + 4);
                            break;

                        case 0x35C0:        // SHL
                            carryF = 0;
                            accu = (ushort)(accu << 1);
                            historyCmd[runIndex] = cmd;
                            historyPC[runIndex] = pc;
                            historyAccu[runIndex] = accu;
                            historyAdressbus[runIndex] = 0;
                            historyDatabus[runIndex] = 0;
                            historyReads[runIndex] = 0;
                            historyWrites[runIndex] = 0;
                            historyFlags[runIndex] = carryF;
                            historyStack[runIndex] = stack;
                            pc += 2;
                            historyCycles[runIndex] = (ushort)(historyCycles[runIndex - 1] + 4);
                            break;

                        case 0x3580:        // SHR
                            carryF = 0;
                            accu = (ushort)(accu >> 1);
                            historyCmd[runIndex] = cmd;
                            historyPC[runIndex] = pc;
                            historyAccu[runIndex] = accu;
                            historyAdressbus[runIndex] = 0;
                            historyDatabus[runIndex] = 0;
                            historyReads[runIndex] = 0;
                            historyWrites[runIndex] = 0;
                            historyFlags[runIndex] = carryF;
                            historyStack[runIndex] = stack;
                            pc += 2;
                            historyCycles[runIndex] = (ushort)(historyCycles[runIndex - 1] + 4);
                            break;

                        case 0x4000:        // BA
                            historyPC[runIndex] = pc;
                            pc = (ushort)prog[pc + 1];
                            historyCmd[runIndex] = cmd;
                            historyAccu[runIndex] = accu;
                            historyAdressbus[runIndex] = 0;
                            historyDatabus[runIndex] = 0;
                            historyReads[runIndex] = 0;
                            historyWrites[runIndex] = 0;
                            historyFlags[runIndex] = carryF;
                            historyStack[runIndex] = stack;
                            historyCycles[runIndex] = (ushort)(historyCycles[runIndex - 1] + 4);
                            break;

                        case 0x4008:        // BC
                            historyPC[runIndex] = pc;
                            if (carryF == 1)
                                pc = (ushort)prog[pc + 1];
                            else
                                pc += 2;

                            historyCmd[runIndex] = cmd;
                            historyAccu[runIndex] = accu;
                            historyAdressbus[runIndex] = 0;
                            historyDatabus[runIndex] = 0;
                            historyReads[runIndex] = 0;
                            historyWrites[runIndex] = 0;
                            historyFlags[runIndex] = carryF;
                            historyStack[runIndex] = stack;
                            historyCycles[runIndex] = (ushort)(historyCycles[runIndex - 1] + 4);
                            break;

                        case 0x4010:        // BN
                            historyPC[runIndex] = pc;
                            if ((accu & 0x8000) != 0)
                                pc = (ushort)prog[pc + 1];
                            else
                                pc += 2;

                            historyCmd[runIndex] = cmd;
                            historyAccu[runIndex] = accu;
                            historyAdressbus[runIndex] = 0;
                            historyDatabus[runIndex] = 0;
                            historyReads[runIndex] = 0;
                            historyWrites[runIndex] = 0;
                            historyFlags[runIndex] = carryF;
                            historyStack[runIndex] = stack;
                            historyCycles[runIndex] = (ushort)(historyCycles[runIndex - 1] + 4);
                            break;

                        case 0x4020:        // BE
                            historyPC[runIndex] = pc;
                            if ((accu & 0x0001) == 0)
                                pc = (ushort)prog[pc + 1];
                            else
                                pc += 2;

                            historyCmd[runIndex] = cmd;
                            historyAccu[runIndex] = accu;
                            historyAdressbus[runIndex] = 0;
                            historyDatabus[runIndex] = 0;
                            historyReads[runIndex] = 0;
                            historyWrites[runIndex] = 0;
                            historyFlags[runIndex] = carryF;
                            historyStack[runIndex] = stack;
                            historyCycles[runIndex] = (ushort)(historyCycles[runIndex - 1] + 4);
                            break;

                        case 0x4018:        // BZ
                            historyPC[runIndex] = pc;
                            if (accu == 0)
                                pc = (ushort)prog[pc + 1];
                            else
                                pc += 2;

                            historyCmd[runIndex] = cmd;
                            historyAccu[runIndex] = accu;
                            historyAdressbus[runIndex] = 0;
                            historyDatabus[runIndex] = 0;
                            historyReads[runIndex] = 0;
                            historyWrites[runIndex] = 0;
                            historyFlags[runIndex] = carryF;
                            historyStack[runIndex] = stack;
                            historyCycles[runIndex] = (ushort)(historyCycles[runIndex - 1] + 4);
                            break;

                        case 0x5000:        // CALL
                            stack = (ushort)(pc+2);
                            pc = (ushort)prog[pc + 1];

                            historyCmd[runIndex] = cmd;
                            historyPC[runIndex] = pc;
                            historyAccu[runIndex] = accu;
                            historyAdressbus[runIndex] = 0;
                            historyDatabus[runIndex] = 0;
                            historyReads[runIndex] = 0;
                            historyWrites[runIndex] = 0;
                            historyFlags[runIndex] = carryF;
                            historyStack[runIndex] = stack;
                            historyCycles[runIndex] = (ushort)(historyCycles[runIndex - 1] + 4);
                            break;

                        case 0x6000:        // RET
                            historyPC[runIndex] = pc;
                            pc = stack;

                            historyCmd[runIndex] = cmd;
                            historyAccu[runIndex] = accu;
                            historyAdressbus[runIndex] = 0;
                            historyDatabus[runIndex] = 0;
                            historyReads[runIndex] = 0;
                            historyWrites[runIndex] = 0;
                            historyFlags[runIndex] = carryF;
                            historyStack[runIndex] = stack;
                            historyCycles[runIndex] = (ushort)(historyCycles[runIndex - 1] + 5);
                            break;

                        default:            // Unbekannter Befehl -> Abbruch
                            runActive = false;
                            break;
                    }
                    runIndex++;
                }
            }
            catch (Exception e)                    
            {
                Console.WriteLine("Error, no Halt");
                Console.WriteLine(e.ToString());
                Console.WriteLine(pc);
            }
        }

        static void Main(string[] args)
        {
            string inputFile = args.Length > 0 ? args[0] : "test.txt";
            string outputFile = "save.txt";

            if (!File.Exists(inputFile))
            {
                Console.WriteLine($"Input file '{inputFile}' not found.");
                return;
            }

            // Datei einlesen und Kommentare entfernen
            string content = File.ReadAllText(inputFile);
            content = Regex.Replace(content, @"/\*.*?\*/", string.Empty, RegexOptions.Singleline);

            // Zuweisungen parsen
            var dict = new Dictionary<int, int>();
            var linePattern = new Regex(@"daten\[(\d+)\]\s*=\s*(0x[0-9A-Fa-f]+|\d+);", RegexOptions.Compiled);
            foreach (var rawLine in content.Split(new[] { '\r', '\n' }, StringSplitOptions.RemoveEmptyEntries))
            {
                string line = Regex.Replace(rawLine, @"//.*", string.Empty).Trim();
                if (string.IsNullOrWhiteSpace(line)) continue;
                var match = linePattern.Match(line);
                if (match.Success)
                {
                    int index = int.Parse(match.Groups[1].Value);
                    string token = match.Groups[2].Value;
                    int value = token.StartsWith("0x", StringComparison.OrdinalIgnoreCase)
                        ? Convert.ToInt32(token.Substring(2), 16)
                        : int.Parse(token);
                    dict[index] = value;
                }
            }

            if (dict.Count == 0)
            {
                Console.WriteLine("Keine Daten gefunden.");
                return;
            }

            // Programm-Array mit fester Länge 1024 – alle Elemente default auf 0
            const int programSize = 1024;
            int[] prog = new int[programSize];

            foreach (var kv in dict)
            {
                if (kv.Key >= 0 && kv.Key < programSize)
                    prog[kv.Key] = kv.Value;
                else
                    Console.WriteLine($"Warnung: Index {kv.Key} liegt außerhalb des Programmbereichs.");
            }

            // Simulation starten und History sammeln
            runProgram(prog);

            // Ausgabe: History-Tabelle in Hex
            using (var writer = new StreamWriter(outputFile))
            {
                writer.WriteLine("Idx\tCmd \tPC  \tAccu\tAddr\tData\tRead\tWrite\tFlag\tStack\tCycles");
                int end = runIndex - 1;
                for (int i = 1; i <= end; i++)
                {
                    writer.WriteLine(
                        i.ToString("X4") + '\t' +
                        historyCmd[i].ToString("X4") + '\t' +
                        historyPC[i].ToString("X4") + '\t' +
                        historyAccu[i].ToString("X4") + '\t' +
                        historyAdressbus[i].ToString("X4") + '\t' +
                        historyDatabus[i].ToString("X4") + '\t' +
                        historyReads[i].ToString("X4") + '\t' +
                        historyWrites[i].ToString("X4") + '\t' +
                        historyFlags[i].ToString("X4") + '\t' +
                        historyStack[i].ToString("X4") + '\t' +
                        historyCycles[i].ToString("X4")
                    );
                }
                writer.WriteLine("Time in sec: " + (historyCycles[runIndex - 2] * cycletime) / 1000);
            }

            Console.WriteLine($"History-Tabelle ({runIndex - 1} Einträge) in '{outputFile}' geschrieben.");
        }
    }
}

# ⚙️ Clack-BASIC: Runtime-Routinen & Grafik-Architektur

Da der 16-Bit Relais-Computer eine minimalistische Ein-Register-Hardware (Accu) ist, bindet der Compiler automatisch optimierte Unterprogramme ein, sobald sie im Quellcode benötigt werden:

---

## 1. `__runtime_cls` (Display Löschen)
* **Syntax:** `CLS` oder `CLS StartZeile, EndZeile`
* **Arbeitsweise:**
  * Bei `CLS`: Löscht alle 64 Worte des 32×32 Framebuffers (`960..1023`).
  * Bei `CLS Start, End`: Berechnet `StartAddr = 960 + Start*2` und `EndAddr = 960 + End*2 + 2`. Löscht nur die gewünschten Zeilen.
* **Vorteil:** Erlaubt partielles Löschen in Millisekunden statt immer den gesamten Bildschirm zu bereinigen.

---

## 2. `__runtime_pset` (Single-Pixel Bitmasking)
* **Syntax:** `PSET X, Y, Color` (wobei `Color = 1` für An, `Color = 0` für Aus)
* **Arbeitsweise (Read-Modify-Write):**
  1. **Wort-Adresse berechnen:** `wordAddr = 960 + (Y * 2) + (X / 16)`
  2. **Bitmaske berechnen:** Startet bei `0x8000` (Bit 15) und shiftet um `X & 15` nach rechts (`0x8000 >> (X & 15)`).
  3. **Aktuellen Framebuffer-Wert laden:** `currWord = memory[wordAddr]`.
  4. **Bit-Operation:**
     * Wenn `Color != 0`: `currWord = currWord | mask` (Pixel setzen)
     * Wenn `Color == 0`: `currWord = currWord & ~mask` (Pixel rückstandslos löschen)
  5. **Zurückschreiben:** `memory[wordAddr] = currWord`.
* **Ergebnis:** Ermöglicht echte Single-Pixel-Grafiken und flackerfreies Delta-Rendering für Spiele.

---

## 3. `__runtime_mul` & `__runtime_div`
* **Multiplikation (`*`):** 16-Bit Shift-and-Add Algorithmus über Register `__r0` und `__r1`.
* **Division (`/` und `MOD`):** Wiederholte 2er-Komplement Subtraktion. Der Rest wird in `__rem` abgelegt.

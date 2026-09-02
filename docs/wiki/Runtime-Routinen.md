# 🧮 Runtime-Routinen

Da die Relais-Hardware nur elementare Operationen unterstützt, bindet der Compiler automatisch optimierte Unterprogramme ein:

---

## 1. 16-Bit Multiplikation (Shift-and-Add)

Wird als Subroutine `__mul` eingebunden:

```text
// Subroutine: factor1 * factor2 -> prod
// Nutzt BE (Branch if Even) für LSB-Prüfung und SHL/SHR
```

## 2. 16-Bit Division & Modulo (Restoring Division)

Wird als Subroutine `__div` eingebunden:
* Gibt den Quotienten im Akkumulator zurück.
* Schreibt den Divisionsrest in die interne Zelle `__rem`.

## 3. DATA / READ / RESTORE

* **DATA:** Tabelle am Ende des Codes im oberen Speicherbereich.
* **READ:** Verwendet internen Zeiger `__data_ptr` und indirekten Load `LDAMM`.
* **RESTORE:** Setzt `__data_ptr` wieder auf den Tabellenanfang zurück.

package lexer

import "fmt"

// TokenType repräsentiert die Art eines Tokens im BASIC-Quellcode.
type TokenType int

const (
	// Spezielle Token
	TOKEN_EOF TokenType = iota
	TOKEN_ILLEGAL
	TOKEN_NEWLINE
	TOKEN_COMMENT

	// Literale & Bezeichner
	TOKEN_IDENT   // Variablenname (z.B. A, X, MEINE_VAR)
	TOKEN_NUMBER  // Ganzzahl (z.B. 42, 0x1400, 0b1010)
	TOKEN_STRING  // Zeichenkette (z.B. "HALLO")

	// BASIC-Schlüsselwörter (Keywords)
	TOKEN_REM     // REM Kommentar
	TOKEN_LET     // LET X = 10 (optional)
	TOKEN_GOTO    // GOTO 100 oder GOTO MEIN_LABEL
	TOKEN_GOSUB   // GOSUB 500 (Unterprogrammaufruf)
	TOKEN_RETURN  // RETURN (Rücksprung aus Unterprogramm)
	TOKEN_IF      // IF Bedingung THEN ...
	TOKEN_THEN    // THEN
	TOKEN_ELSE    // ELSE
	TOKEN_FOR     // FOR I = 1 TO 10 STEP 2
	TOKEN_TO      // TO
	TOKEN_STEP    // STEP
	TOKEN_NEXT    // NEXT I
	TOKEN_DATA    // DATA 10, 20, 30
	TOKEN_READ    // READ X, Y
	TOKEN_RESTORE // RESTORE (setzt DATA-Zeiger zurück)
	TOKEN_PEEK    // PEEK(Adresse)
	TOKEN_POKE    // POKE Adresse, Wert
	TOKEN_IN      // IN(Port)
	TOKEN_OUT     // OUT Port, Wert
	TOKEN_CLS     // CLS (Display / Matrix löschen)
	TOKEN_PSET    // PSET X, Y, Color (Pixel auf Matrix setzen)
	TOKEN_PRINT   // PRINT "Wert=", X
	TOKEN_END     // END / STOP
	TOKEN_HALT    // HALT

	// Arithmetische Operatoren
	TOKEN_PLUS    // +
	TOKEN_MINUS   // -
	TOKEN_STAR    // * (Multiplikation via Subroutine)
	TOKEN_SLASH   // / (Division via Subroutine)
	TOKEN_MOD     // MOD (Modulo / Divisionsrest)

	// Bitweise & Logische Operatoren
	TOKEN_AND     // AND
	TOKEN_OR      // OR
	TOKEN_XOR     // XOR
	TOKEN_NOT     // NOT
	TOKEN_SHL     // SHL (Shift Left)
	TOKEN_SHR     // SHR (Shift Right)

	// Vergleichsoperatoren
	TOKEN_EQ      // = oder ==
	TOKEN_NEQ     // <> oder !=
	TOKEN_LT      // <
	TOKEN_LTE     // <=
	TOKEN_GT      // >
	TOKEN_GTE     // >=

	// Trennzeichen & Satzzeichen
	TOKEN_COMMA     // ,
	TOKEN_COLON     // : (Befehlstrenner in einer Zeile)
	TOKEN_LPAREN    // (
	TOKEN_RPAREN    // )
	TOKEN_LBRACKET  // [
	TOKEN_RBRACKET  // ]
)

// Token speichert den Typ, Textwert und die Zeilennummer im Quellcode.
type Token struct {
	Type   TokenType
	Lexeme string
	Line   int
	Column int
	Value  int // Für Zahlen
}

func (t Token) String() string {
	return fmt.Sprintf("Token(%d, '%s', Zeile:%d)", t.Type, t.Lexeme, t.Line)
}

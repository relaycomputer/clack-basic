package lexer

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Lexer zerlegt BASIC-Quelltext in eine Folge von Tokens.
type Lexer struct {
	source []rune
	pos    int
	readPos int
	ch     rune
	line   int
	col    int
}

// NewLexer initialisiert einen neuen Lexer mit dem übergebenen Quellcode.
func NewLexer(input string) *Lexer {
	l := &Lexer{
		source: []rune(input),
		line:   1,
		col:    0,
	}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPos >= len(l.source) {
		l.ch = 0 // EOF
	} else {
		l.ch = l.source[l.readPos]
	}
	l.pos = l.readPos
	l.readPos++
	l.col++
}

func (l *Lexer) peekChar() rune {
	if l.readPos >= len(l.source) {
		return 0
	}
	return l.source[l.readPos]
}

// NextToken ermittelt das nächste Token im Quelltext.
func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	if l.ch == 0 {
		return Token{Type: TOKEN_EOF, Lexeme: "EOF", Line: l.line, Column: l.col}
	}

	tokLine := l.line
	tokCol := l.col

	// 1. Zeilenumbrüche
	if l.ch == '\n' || l.ch == '\r' {
		if l.ch == '\r' && l.peekChar() == '\n' {
			l.readChar()
		}
		l.line++
		l.col = 0
		l.readChar()
		return Token{Type: TOKEN_NEWLINE, Lexeme: "\\n", Line: tokLine, Column: tokCol}
	}

	// 2. Kommentare (// oder ')
	if l.ch == '\'' || (l.ch == '/' && l.peekChar() == '/') {
		l.skipLineComment()
		return l.NextToken()
	}

	// 3. String-Literale ("...")
	if l.ch == '"' {
		str := l.readString()
		return Token{Type: TOKEN_STRING, Lexeme: str, Line: tokLine, Column: tokCol}
	}

	// 4. Zahlen (Dezimal, Hexadezimal 0x... / &H..., Binär 0b...)
	if unicode.IsDigit(l.ch) || (l.ch == '&' && (l.peekChar() == 'H' || l.peekChar() == 'h')) {
		return l.readNumber()
	}

	// 5. Bezeichner und Schlüsselwörter (Buchstaben oder Unterstriche)
	if isLetter(l.ch) {
		ident := l.readIdentifier()
		upper := strings.ToUpper(ident)

		// Spezialfall: REM Kommentarzeile
		if upper == "REM" {
			l.skipLineComment()
			return l.NextToken()
		}

		// Schlüsselwort prüfen
		if kwType, ok := keywords[upper]; ok {
			return Token{Type: kwType, Lexeme: upper, Line: tokLine, Column: tokCol}
		}

		return Token{Type: TOKEN_IDENT, Lexeme: upper, Line: tokLine, Column: tokCol}
	}

	// 6. Operatoren & Satzzeichen
	ch := l.ch
	l.readChar()

	switch ch {
	case '+':
		return Token{Type: TOKEN_PLUS, Lexeme: "+", Line: tokLine, Column: tokCol}
	case '-':
		return Token{Type: TOKEN_MINUS, Lexeme: "-", Line: tokLine, Column: tokCol}
	case '*':
		return Token{Type: TOKEN_STAR, Lexeme: "*", Line: tokLine, Column: tokCol}
	case '/':
		return Token{Type: TOKEN_SLASH, Lexeme: "/", Line: tokLine, Column: tokCol}
	case ',':
		return Token{Type: TOKEN_COMMA, Lexeme: ",", Line: tokLine, Column: tokCol}
	case ':':
		return Token{Type: TOKEN_COLON, Lexeme: ":", Line: tokLine, Column: tokCol}
	case '(':
		return Token{Type: TOKEN_LPAREN, Lexeme: "(", Line: tokLine, Column: tokCol}
	case ')':
		return Token{Type: TOKEN_RPAREN, Lexeme: ")", Line: tokLine, Column: tokCol}
	case '[':
		return Token{Type: TOKEN_LBRACKET, Lexeme: "[", Line: tokLine, Column: tokCol}
	case ']':
		return Token{Type: TOKEN_RBRACKET, Lexeme: "]", Line: tokLine, Column: tokCol}

	case '=':
		if l.ch == '=' {
			l.readChar()
			return Token{Type: TOKEN_EQ, Lexeme: "==", Line: tokLine, Column: tokCol}
		}
		return Token{Type: TOKEN_EQ, Lexeme: "=", Line: tokLine, Column: tokCol}

	case '<':
		if l.ch == '>' {
			l.readChar()
			return Token{Type: TOKEN_NEQ, Lexeme: "<>", Line: tokLine, Column: tokCol}
		} else if l.ch == '=' {
			l.readChar()
			return Token{Type: TOKEN_LTE, Lexeme: "<=", Line: tokLine, Column: tokCol}
		}
		return Token{Type: TOKEN_LT, Lexeme: "<", Line: tokLine, Column: tokCol}

	case '>':
		if l.ch == '=' {
			l.readChar()
			return Token{Type: TOKEN_GTE, Lexeme: ">=", Line: tokLine, Column: tokCol}
		}
		return Token{Type: TOKEN_GT, Lexeme: ">", Line: tokLine, Column: tokCol}

	case '!':
		if l.ch == '=' {
			l.readChar()
			return Token{Type: TOKEN_NEQ, Lexeme: "!=", Line: tokLine, Column: tokCol}
		}
	}

	return Token{
		Type:   TOKEN_ILLEGAL,
		Lexeme: string(ch),
		Line:   tokLine,
		Column: tokCol,
	}
}

// Tokenize verarbeitet den gesamten Text und liefert eine Token-Liste.
func (l *Lexer) Tokenize() []Token {
	var tokens []Token
	for {
		tok := l.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == TOKEN_EOF {
			break
		}
	}
	return tokens
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' {
		l.readChar()
	}
}

func (l *Lexer) skipLineComment() {
	for l.ch != '\n' && l.ch != '\r' && l.ch != 0 {
		l.readChar()
	}
}

func (l *Lexer) readString() string {
	l.readChar() // Öffnendes Anführungszeichen überspringen
	var sb strings.Builder
	for l.ch != '"' && l.ch != '\n' && l.ch != 0 {
		sb.WriteRune(l.ch)
		l.readChar()
	}
	if l.ch == '"' {
		l.readChar() // Schließendes Anführungszeichen
	}
	return sb.String()
}

func (l *Lexer) readIdentifier() string {
	startPos := l.pos
	for isLetter(l.ch) || unicode.IsDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
	return string(l.source[startPos:l.pos])
}

func (l *Lexer) readNumber() Token {
	tokLine := l.line
	tokCol := l.col
	startPos := l.pos

	// Hex mit &H... (z.B. &H1400)
	if l.ch == '&' {
		l.readChar() // '&'
		l.readChar() // 'H'
		hexStart := l.pos
		for isHexDigit(l.ch) {
			l.readChar()
		}
		hexStr := string(l.source[hexStart:l.pos])
		val, _ := strconv.ParseInt(hexStr, 16, 64)
		return Token{
			Type:   TOKEN_NUMBER,
			Lexeme: "&H" + hexStr,
			Line:   tokLine,
			Column: tokCol,
			Value:  int(val),
		}
	}

	// Hex mit 0x... oder Binär mit 0b...
	if l.ch == '0' && (l.peekChar() == 'x' || l.peekChar() == 'X') {
		l.readChar()
		l.readChar()
		for isHexDigit(l.ch) {
			l.readChar()
		}
		numStr := string(l.source[startPos:l.pos])
		val, _ := strconv.ParseInt(numStr[2:], 16, 64)
		return Token{
			Type:   TOKEN_NUMBER,
			Lexeme: numStr,
			Line:   tokLine,
			Column: tokCol,
			Value:  int(val),
		}
	}

	if l.ch == '0' && (l.peekChar() == 'b' || l.peekChar() == 'B') {
		l.readChar()
		l.readChar()
		for l.ch == '0' || l.ch == '1' {
			l.readChar()
		}
		numStr := string(l.source[startPos:l.pos])
		val, _ := strconv.ParseInt(numStr[2:], 2, 64)
		return Token{
			Type:   TOKEN_NUMBER,
			Lexeme: numStr,
			Line:   tokLine,
			Column: tokCol,
			Value:  int(val),
		}
	}

	// Normale Dezimalzahl
	for unicode.IsDigit(l.ch) {
		l.readChar()
	}
	numStr := string(l.source[startPos:l.pos])
	val, err := strconv.Atoi(numStr)
	if err != nil {
		return Token{Type: TOKEN_ILLEGAL, Lexeme: fmt.Sprintf("Ungültige Zahl: %s", numStr), Line: tokLine, Column: tokCol}
	}

	return Token{
		Type:   TOKEN_NUMBER,
		Lexeme: numStr,
		Line:   tokLine,
		Column: tokCol,
		Value:  val,
	}
}

func isLetter(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

func isHexDigit(ch rune) bool {
	return unicode.IsDigit(ch) ||
		(ch >= 'a' && ch <= 'f') ||
		(ch >= 'A' && ch <= 'F')
}

// Schlüsselwort-Zuordnung (Keywords)
var keywords = map[string]TokenType{
	"REM":     TOKEN_REM,
	"LET":     TOKEN_LET,
	"GOTO":    TOKEN_GOTO,
	"GOSUB":   TOKEN_GOSUB,
	"RETURN":  TOKEN_RETURN,
	"IF":      TOKEN_IF,
	"THEN":    TOKEN_THEN,
	"ELSE":    TOKEN_ELSE,
	"FOR":     TOKEN_FOR,
	"TO":      TOKEN_TO,
	"STEP":    TOKEN_STEP,
	"NEXT":    TOKEN_NEXT,
	"DATA":    TOKEN_DATA,
	"READ":    TOKEN_READ,
	"RESTORE": TOKEN_RESTORE,
	"PEEK":    TOKEN_PEEK,
	"POKE":    TOKEN_POKE,
	"IN":      TOKEN_IN,
	"OUT":     TOKEN_OUT,
	"CLS":       TOKEN_CLS,
	"PSET":      TOKEN_PSET,
	"PLOT":      TOKEN_PLOT,
	"LINE":      TOKEN_LINE,
	"PIXELMOVE": TOKEN_PIXELMOVE,
	"BY":        TOKEN_BY,
	"PRINT":     TOKEN_PRINT,
	"END":       TOKEN_END,
	"STOP":      TOKEN_END,
	"HALT":      TOKEN_HALT,

	// Arithmetik / Logik Keywords
	"AND": TOKEN_AND,
	"OR":  TOKEN_OR,
	"XOR": TOKEN_XOR,
	"NOT": TOKEN_NOT,
	"MOD": TOKEN_MOD,
	"SHL": TOKEN_SHL,
	"SHR": TOKEN_SHR,
}

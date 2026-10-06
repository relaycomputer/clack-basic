package parser

/*
 ==============================================================================
 📜 PARSER FÜR CLACK-BASIC
 ==============================================================================
 Liest die Tokens des Lexers ein und baut den abstrakten Syntaxbaum (AST) auf.
 Unterstützt Zeilennummern, Labels, Prioritäten (Operator Precedence) und
 modulare Erweiterung um neue Befehle.
 ==============================================================================
*/

import (
	"fmt"
	"strconv"

	"clack-basic/pkg/ast"
	"clack-basic/pkg/lexer"
)

// Operator-Präzedenz (Bindungsstärke)
const (
	PREC_LOWEST = iota
	PREC_LOGIC  // AND, OR, XOR
	PREC_COMP   // =, <>, <, <=, >, >=
	PREC_SHIFT  // SHL, SHR
	PREC_SUM    // +, -
	PREC_PROD   // *, /, MOD
	PREC_PREFIX // Unäres -, NOT
	PREC_CALL   // PEEK(), IN()
)

var precedences = map[lexer.TokenType]int{
	lexer.TOKEN_OR:    PREC_LOGIC,
	lexer.TOKEN_XOR:   PREC_LOGIC,
	lexer.TOKEN_AND:   PREC_LOGIC,
	lexer.TOKEN_EQ:    PREC_COMP,
	lexer.TOKEN_NEQ:   PREC_COMP,
	lexer.TOKEN_LT:    PREC_COMP,
	lexer.TOKEN_LTE:   PREC_COMP,
	lexer.TOKEN_GT:    PREC_COMP,
	lexer.TOKEN_GTE:   PREC_COMP,
	lexer.TOKEN_SHL:   PREC_SHIFT,
	lexer.TOKEN_SHR:   PREC_SHIFT,
	lexer.TOKEN_PLUS:  PREC_SUM,
	lexer.TOKEN_MINUS: PREC_SUM,
	lexer.TOKEN_STAR:  PREC_PROD,
	lexer.TOKEN_SLASH: PREC_PROD,
	lexer.TOKEN_MOD:   PREC_PROD,
}

// Parser steuert die syntaktische Analyse.
type Parser struct {
	tokens  []lexer.Token
	pos     int
	curTok  lexer.Token
	peekTok lexer.Token
	errors  []string
}

func NewParser(tokens []lexer.Token) *Parser {
	p := &Parser{
		tokens: tokens,
		pos:    0,
	}
	p.nextToken()
	p.nextToken()
	return p
}

func (p *Parser) nextToken() {
	p.curTok = p.peekTok
	if p.pos < len(p.tokens) {
		p.peekTok = p.tokens[p.pos]
		p.pos++
	} else {
		p.peekTok = lexer.Token{Type: lexer.TOKEN_EOF, Lexeme: "EOF"}
	}
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) addError(msg string) {
	p.errors = append(p.errors, fmt.Sprintf("Zeile %d, Spalte %d: %s", p.curTok.Line, p.curTok.Column, msg))
}

// ParseProgram erzeugt das vollständige AST-Programm.
func (p *Parser) ParseProgram() (*ast.Program, error) {
	program := &ast.Program{
		Statements: []ast.Stmt{},
	}

	for p.curTok.Type != lexer.TOKEN_EOF {
		// Ignoriere führende Leerzeilen / Doppelpunkte
		if p.curTok.Type == lexer.TOKEN_NEWLINE || p.curTok.Type == lexer.TOKEN_COLON {
			p.nextToken()
			continue
		}

		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}

		// Nach Anweisung Zeilenumbruch oder Doppelpunkt erwarten
		if p.curTok.Type == lexer.TOKEN_NEWLINE || p.curTok.Type == lexer.TOKEN_COLON {
			p.nextToken()
		}
	}

	if len(p.errors) > 0 {
		return nil, fmt.Errorf("Syntaxfehler beim Parsen:\n - %s", p.errors[0])
	}

	return program, nil
}

// ============================================================================
// STATEMENTS PARSEN
// ============================================================================

func (p *Parser) parseStatement() ast.Stmt {
	// 1. Zeilennummer als Label (z.B. "10 GOTO 20")
	if p.curTok.Type == lexer.TOKEN_NUMBER {
		lineNum := strconv.Itoa(p.curTok.Value)
		tok := p.curTok
		p.nextToken()

		// Wenn danach direkt ein Doppelpunkt steht (z.B. "10: LET A=1")
		if p.curTok.Type == lexer.TOKEN_COLON {
			p.nextToken()
		}

		// Erzeuge Label-Node für die Zeilennummer
		return &ast.LabelStmt{Name: lineNum, Line: tok.Line}
	}

	// 2. Text-Label (z.B. "START:")
	if p.curTok.Type == lexer.TOKEN_IDENT && p.peekTok.Type == lexer.TOKEN_COLON {
		labelName := p.curTok.Lexeme
		line := p.curTok.Line
		p.nextToken() // Name überspringen
		p.nextToken() // Doppelpunkt überspringen
		return &ast.LabelStmt{Name: labelName, Line: line}
	}

	switch p.curTok.Type {
	case lexer.TOKEN_LET:
		return p.parseLetStatement()
	case lexer.TOKEN_GOTO:
		return p.parseGotoStatement()
	case lexer.TOKEN_GOSUB:
		return p.parseGosubStatement()
	case lexer.TOKEN_RETURN:
		line := p.curTok.Line
		p.nextToken()
		return &ast.ReturnStmt{Line: line}
	case lexer.TOKEN_IF:
		return p.parseIfStatement()
	case lexer.TOKEN_FOR:
		return p.parseForStatement()
	case lexer.TOKEN_NEXT:
		return p.parseNextStatement()
	case lexer.TOKEN_DATA:
		return p.parseDataStatement()
	case lexer.TOKEN_READ:
		return p.parseReadStatement()
	case lexer.TOKEN_RESTORE:
		line := p.curTok.Line
		p.nextToken()
		return &ast.RestoreStmt{Line: line}
	case lexer.TOKEN_POKE:
		return p.parsePokeStatement()
	case lexer.TOKEN_OUT:
		return p.parseOutStatement()
	case lexer.TOKEN_CLS:
		return p.parseClsStatement()
	case lexer.TOKEN_PSET, lexer.TOKEN_PLOT:
		return p.parsePsetStatement()
	case lexer.TOKEN_LINE:
		return p.parseLineStatement()
	case lexer.TOKEN_PIXELMOVE:
		return p.parsePixelMoveStatement()
	case lexer.TOKEN_PRINT:
		return p.parsePrintStatement()
	case lexer.TOKEN_END, lexer.TOKEN_HALT:
		line := p.curTok.Line
		p.nextToken()
		return &ast.EndStmt{Line: line}

	case lexer.TOKEN_IDENT:
		// Implizites LET: "A = 10" ohne das Wort LET
		return p.parseLetStatement()

	default:
		p.addError(fmt.Sprintf("Unerwartetes Token an Zeilenanfang: %s", p.curTok.Lexeme))
		p.nextToken()
		return nil
	}
}

func (p *Parser) parseLetStatement() ast.Stmt {
	line := p.curTok.Line
	if p.curTok.Type == lexer.TOKEN_LET {
		p.nextToken() // "LET" überspringen
	}

	if p.curTok.Type != lexer.TOKEN_IDENT {
		p.addError(fmt.Sprintf("Variablenname nach LET erwartet, erhalten: %s", p.curTok.Lexeme))
		return nil
	}

	varName := p.curTok.Lexeme
	p.nextToken() // Variablenname überspringen

	if p.curTok.Type != lexer.TOKEN_EQ {
		p.addError(fmt.Sprintf("'=' nach Variable %s erwartet", varName))
		return nil
	}
	p.nextToken() // '=' überspringen

	valExpr := p.parseExpression(PREC_LOWEST)
	return &ast.LetStmt{VarName: varName, Value: valExpr, Line: line}
}

func (p *Parser) parseGotoStatement() ast.Stmt {
	line := p.curTok.Line
	p.nextToken() // GOTO überspringen

	var target string
	if p.curTok.Type == lexer.TOKEN_NUMBER || p.curTok.Type == lexer.TOKEN_IDENT {
		target = p.curTok.Lexeme
		p.nextToken()
	} else {
		p.addError("Sprungziel (Zeilennummer oder Label) nach GOTO erwartet")
		return nil
	}
	return &ast.GotoStmt{Target: target, Line: line}
}

func (p *Parser) parseGosubStatement() ast.Stmt {
	line := p.curTok.Line
	p.nextToken() // GOSUB überspringen

	var target string
	if p.curTok.Type == lexer.TOKEN_NUMBER || p.curTok.Type == lexer.TOKEN_IDENT {
		target = p.curTok.Lexeme
		p.nextToken()
	} else {
		p.addError("Sprungziel (Zeilennummer oder Label) nach GOSUB erwartet")
		return nil
	}
	return &ast.GosubStmt{Target: target, Line: line}
}

func (p *Parser) parseIfStatement() ast.Stmt {
	line := p.curTok.Line
	p.nextToken() // IF überspringen

	cond := p.parseExpression(PREC_LOWEST)

	if p.curTok.Type != lexer.TOKEN_THEN {
		p.addError("THEN nach IF-Bedingung erwartet")
		return nil
	}
	p.nextToken() // THEN überspringen

	thenBranch := []ast.Stmt{}
	elseBranch := []ast.Stmt{}

	// Prüfe, ob es sich um einen mehrzeiligen Block handelt (Zeilenumbruch nach THEN)
	isBlock := false
	if p.curTok.Type == lexer.TOKEN_NEWLINE {
		isBlock = true
		p.nextToken()
	}

	if isBlock {
		// Mehrzeiliger THEN-Block bis ELSE, END IF, ENDIF oder EOF
		for p.curTok.Type != lexer.TOKEN_EOF {
			if p.curTok.Type == lexer.TOKEN_NEWLINE || p.curTok.Type == lexer.TOKEN_COLON {
				p.nextToken()
				continue
			}

			if p.curTok.Type == lexer.TOKEN_ELSE {
				break
			}
			if p.curTok.Type == lexer.TOKEN_END && p.peekTok.Type == lexer.TOKEN_IF {
				p.nextToken() // END
				p.nextToken() // IF
				return &ast.IfStmt{Condition: cond, ThenBranch: thenBranch, ElseBranch: elseBranch, Line: line}
			}
			if p.curTok.Type == lexer.TOKEN_IDENT && p.curTok.Lexeme == "ENDIF" {
				p.nextToken()
				return &ast.IfStmt{Condition: cond, ThenBranch: thenBranch, ElseBranch: elseBranch, Line: line}
			}

			stmt := p.parseStatement()
			if stmt != nil {
				thenBranch = append(thenBranch, stmt)
			}
		}

		// Falls ELSE folgt
		if p.curTok.Type == lexer.TOKEN_ELSE {
			p.nextToken() // ELSE überspringen
			if p.curTok.Type == lexer.TOKEN_NEWLINE {
				p.nextToken()
			}
			for p.curTok.Type != lexer.TOKEN_EOF {
				if p.curTok.Type == lexer.TOKEN_NEWLINE || p.curTok.Type == lexer.TOKEN_COLON {
					p.nextToken()
					continue
				}
				if p.curTok.Type == lexer.TOKEN_END && p.peekTok.Type == lexer.TOKEN_IF {
					p.nextToken() // END
					p.nextToken() // IF
					break
				}
				if p.curTok.Type == lexer.TOKEN_IDENT && p.curTok.Lexeme == "ENDIF" {
					p.nextToken()
					break
				}

				stmt := p.parseStatement()
				if stmt != nil {
					elseBranch = append(elseBranch, stmt)
				}
			}
		}
	} else {
		// Einzeiliges IF THEN ... [ELSE ...]
		if p.curTok.Type == lexer.TOKEN_NUMBER {
			target := p.curTok.Lexeme
			p.nextToken()
			thenBranch = append(thenBranch, &ast.GotoStmt{Target: target, Line: line})
		} else {
			stmt := p.parseStatement()
			if stmt != nil {
				thenBranch = append(thenBranch, stmt)
			}
		}

		if p.curTok.Type == lexer.TOKEN_ELSE {
			p.nextToken()
			if p.curTok.Type == lexer.TOKEN_NUMBER {
				target := p.curTok.Lexeme
				p.nextToken()
				elseBranch = append(elseBranch, &ast.GotoStmt{Target: target, Line: line})
			} else {
				stmt := p.parseStatement()
				if stmt != nil {
					elseBranch = append(elseBranch, stmt)
				}
			}
		}
	}

	return &ast.IfStmt{Condition: cond, ThenBranch: thenBranch, ElseBranch: elseBranch, Line: line}
}

func (p *Parser) parseForStatement() ast.Stmt {
	line := p.curTok.Line
	p.nextToken() // FOR überspringen

	if p.curTok.Type != lexer.TOKEN_IDENT {
		p.addError("Zählvariable nach FOR erwartet")
		return nil
	}
	varName := p.curTok.Lexeme
	p.nextToken()

	if p.curTok.Type != lexer.TOKEN_EQ {
		p.addError("'=' nach FOR-Variable erwartet")
		return nil
	}
	p.nextToken()

	startExpr := p.parseExpression(PREC_LOWEST)

	if p.curTok.Type != lexer.TOKEN_TO {
		p.addError("TO nach Startwert im FOR-Statement erwartet")
		return nil
	}
	p.nextToken() // TO überspringen

	endExpr := p.parseExpression(PREC_LOWEST)

	var stepExpr ast.Expr
	if p.curTok.Type == lexer.TOKEN_STEP {
		p.nextToken()
		stepExpr = p.parseExpression(PREC_LOWEST)
	} else {
		stepExpr = &ast.NumberLiteral{Value: 1, Line: line}
	}

	var body []ast.Stmt
	for p.curTok.Type != lexer.TOKEN_EOF {
		if p.curTok.Type == lexer.TOKEN_NEWLINE || p.curTok.Type == lexer.TOKEN_COLON {
			p.nextToken()
			continue
		}

		if p.curTok.Type == lexer.TOKEN_NEXT {
			p.nextToken() // NEXT überspringen
			if p.curTok.Type == lexer.TOKEN_IDENT {
				p.nextToken() // Optionale Variable nach NEXT überspringen
			}
			break
		}

		stmt := p.parseStatement()
		if stmt != nil {
			body = append(body, stmt)
		}
	}

	return &ast.ForStmt{
		VarName: varName,
		Start:   startExpr,
		End:     endExpr,
		Step:    stepExpr,
		Body:    body,
		Line:    line,
	}
}

func (p *Parser) parseNextStatement() ast.Stmt {
	// NEXT erzeugt intern Inkrementierung und Sprung
	line := p.curTok.Line
	p.nextToken() // NEXT überspringen

	var varName string
	if p.curTok.Type == lexer.TOKEN_IDENT {
		varName = p.curTok.Lexeme
		p.nextToken()
	}
	// In unserem Codegen wird NEXT über einen LabelStmt/LoopStack abgewickelt
	return &ast.LabelStmt{Name: fmt.Sprintf("__next_%s", varName), Line: line}
}

func (p *Parser) parseDataStatement() ast.Stmt {
	line := p.curTok.Line
	p.nextToken() // DATA überspringen

	var values []int
	for {
		if p.curTok.Type == lexer.TOKEN_NUMBER {
			values = append(values, p.curTok.Value)
			p.nextToken()
		} else if p.curTok.Type == lexer.TOKEN_MINUS && p.peekTok.Type == lexer.TOKEN_NUMBER {
			p.nextToken()
			values = append(values, -p.curTok.Value)
			p.nextToken()
		} else {
			p.addError("Zahlenwert in DATA-Anweisung erwartet")
			break
		}

		if p.curTok.Type == lexer.TOKEN_COMMA {
			p.nextToken()
		} else {
			break
		}
	}
	return &ast.DataStmt{Values: values, Line: line}
}

func (p *Parser) parseReadStatement() ast.Stmt {
	line := p.curTok.Line
	p.nextToken() // READ überspringen

	var vars []string
	for {
		if p.curTok.Type == lexer.TOKEN_IDENT {
			vars = append(vars, p.curTok.Lexeme)
			p.nextToken()
		} else {
			p.addError("Variablenname in READ-Anweisung erwartet")
			break
		}

		if p.curTok.Type == lexer.TOKEN_COMMA {
			p.nextToken()
		} else {
			break
		}
	}
	return &ast.ReadStmt{VarNames: vars, Line: line}
}

func (p *Parser) parsePokeStatement() ast.Stmt {
	line := p.curTok.Line
	p.nextToken() // POKE überspringen

	addr := p.parseExpression(PREC_LOWEST)
	if p.curTok.Type != lexer.TOKEN_COMMA {
		p.addError("',' zwischen Adresse und Wert in POKE erwartet")
		return nil
	}
	p.nextToken()
	val := p.parseExpression(PREC_LOWEST)

	return &ast.PokeStmt{Address: addr, Value: val, Line: line}
}

func (p *Parser) parseOutStatement() ast.Stmt {
	line := p.curTok.Line
	p.nextToken() // OUT überspringen

	port := p.parseExpression(PREC_LOWEST)
	if p.curTok.Type != lexer.TOKEN_COMMA {
		p.addError("',' zwischen Port und Wert in OUT erwartet")
		return nil
	}
	p.nextToken()
	val := p.parseExpression(PREC_LOWEST)

	return &ast.OutStmt{Port: port, Value: val, Line: line}
}

func (p *Parser) parseClsStatement() ast.Stmt {
	line := p.curTok.Line
	p.nextToken() // CLS überspringen

	var startRow ast.Expr
	var endRow ast.Expr

	if p.curTok.Type != lexer.TOKEN_NEWLINE && p.curTok.Type != lexer.TOKEN_COLON && p.curTok.Type != lexer.TOKEN_EOF {
		startRow = p.parseExpression(PREC_LOWEST)
		if p.curTok.Type == lexer.TOKEN_COMMA {
			p.nextToken() // ',' überspringen
			endRow = p.parseExpression(PREC_LOWEST)
		} else {
			endRow = startRow
		}
	}

	return &ast.ClsStmt{StartRow: startRow, EndRow: endRow, Line: line}
}

func (p *Parser) parsePsetStatement() ast.Stmt {
	line := p.curTok.Line
	p.nextToken() // PSET oder PLOT überspringen

	x := p.parseExpression(PREC_LOWEST)
	if p.curTok.Type != lexer.TOKEN_COMMA {
		p.addError("',' nach X-Koordinate in PSET/PLOT erwartet")
		return nil
	}
	p.nextToken()
	y := p.parseExpression(PREC_LOWEST)

	var color ast.Expr = &ast.NumberLiteral{Value: 1, Line: line}
	if p.curTok.Type == lexer.TOKEN_COMMA {
		p.nextToken()
		color = p.parseExpression(PREC_LOWEST)
	}

	return &ast.PsetStmt{X: x, Y: y, Color: color, Line: line}
}

func (p *Parser) parsePixelMoveStatement() ast.Stmt {
	line := p.curTok.Line
	p.nextToken() // PIXELMOVE überspringen

	fromX := p.parseExpression(PREC_LOWEST)
	if p.curTok.Type != lexer.TOKEN_COMMA {
		p.addError("',' nach Start-X in PIXELMOVE erwartet")
		return nil
	}
	p.nextToken()
	fromY := p.parseExpression(PREC_LOWEST)

	isRelative := false
	if p.curTok.Type == lexer.TOKEN_TO {
		p.nextToken()
	} else if p.curTok.Type == lexer.TOKEN_BY {
		isRelative = true
		p.nextToken()
	} else if p.curTok.Type == lexer.TOKEN_COMMA {
		p.nextToken()
	} else {
		p.addError("'TO' oder 'BY' in PIXELMOVE erwartet")
		return nil
	}

	toX := p.parseExpression(PREC_LOWEST)
	if p.curTok.Type != lexer.TOKEN_COMMA {
		p.addError("',' nach Ziel-X / Delta-X in PIXELMOVE erwartet")
		return nil
	}
	p.nextToken()
	toY := p.parseExpression(PREC_LOWEST)

	var color ast.Expr = &ast.NumberLiteral{Value: 1, Line: line}
	if p.curTok.Type == lexer.TOKEN_COMMA {
		p.nextToken()
		color = p.parseExpression(PREC_LOWEST)
	}

	return &ast.PixelMoveStmt{
		FromX:      fromX,
		FromY:      fromY,
		ToX:        toX,
		ToY:        toY,
		IsRelative: isRelative,
		Color:      color,
		Line:       line,
	}
}

func (p *Parser) parseLineStatement() ast.Stmt {
	line := p.curTok.Line
	p.nextToken() // LINE überspringen

	x1 := p.parseExpression(PREC_LOWEST)
	if p.curTok.Type != lexer.TOKEN_COMMA {
		p.addError("',' nach X1 in LINE erwartet")
		return nil
	}
	p.nextToken()
	y1 := p.parseExpression(PREC_LOWEST)

	if p.curTok.Type == lexer.TOKEN_TO || p.curTok.Type == lexer.TOKEN_COMMA {
		p.nextToken()
	} else {
		p.addError("'TO' oder ',' in LINE erwartet")
		return nil
	}

	x2 := p.parseExpression(PREC_LOWEST)
	if p.curTok.Type != lexer.TOKEN_COMMA {
		p.addError("',' nach X2 in LINE erwartet")
		return nil
	}
	p.nextToken()
	y2 := p.parseExpression(PREC_LOWEST)

	var color ast.Expr = &ast.NumberLiteral{Value: 1, Line: line}
	if p.curTok.Type == lexer.TOKEN_COMMA {
		p.nextToken()
		color = p.parseExpression(PREC_LOWEST)
	}

	return &ast.LineStmt{
		X1:    x1,
		Y1:    y1,
		X2:    x2,
		Y2:    y2,
		Color: color,
		Line:  line,
	}
}

func (p *Parser) parsePrintStatement() ast.Stmt {
	line := p.curTok.Line
	p.nextToken() // PRINT überspringen

	var items []ast.Expr
	for p.curTok.Type != lexer.TOKEN_NEWLINE && p.curTok.Type != lexer.TOKEN_COLON && p.curTok.Type != lexer.TOKEN_EOF {
		expr := p.parseExpression(PREC_LOWEST)
		if expr != nil {
			items = append(items, expr)
		}
		if p.curTok.Type == lexer.TOKEN_COMMA || p.curTok.Type == lexer.TOKEN_COLON {
			if p.curTok.Type == lexer.TOKEN_COMMA {
				p.nextToken()
			}
		} else {
			break
		}
	}
	return &ast.PrintStmt{Items: items, Line: line}
}

// ============================================================================
// AUSDRÜCKE (EXPRESSIONS) PARSEN — PRATT PARSER
// ============================================================================

func (p *Parser) parseExpression(precedence int) ast.Expr {
	prefix := p.parsePrefix()
	if prefix == nil {
		return nil
	}

	left := prefix
	for p.curTok.Type != lexer.TOKEN_NEWLINE && p.curTok.Type != lexer.TOKEN_COLON && p.curTok.Type != lexer.TOKEN_EOF && precedence < p.curPrecedence() {
		left = p.parseInfix(left)
	}

	return left
}

func (p *Parser) parsePrefix() ast.Expr {
	line := p.curTok.Line

	switch p.curTok.Type {
	case lexer.TOKEN_NUMBER:
		val := p.curTok.Value
		p.nextToken()
		return &ast.NumberLiteral{Value: val, Line: line}

	case lexer.TOKEN_IDENT:
		name := p.curTok.Lexeme
		p.nextToken()
		return &ast.VariableExpr{Name: name, Line: line}

	case lexer.TOKEN_MINUS:
		p.nextToken()
		right := p.parseExpression(PREC_PREFIX)
		return &ast.UnaryExpr{Op: "-", Right: right, Line: line}

	case lexer.TOKEN_NOT:
		p.nextToken()
		right := p.parseExpression(PREC_PREFIX)
		return &ast.UnaryExpr{Op: "NOT", Right: right, Line: line}

	case lexer.TOKEN_PEEK:
		p.nextToken()
		if p.curTok.Type == lexer.TOKEN_LPAREN {
			p.nextToken()
		}
		addr := p.parseExpression(PREC_LOWEST)
		if p.curTok.Type == lexer.TOKEN_RPAREN {
			p.nextToken()
		}
		return &ast.PeekExpr{Address: addr, Line: line}

	case lexer.TOKEN_IN:
		p.nextToken()
		if p.curTok.Type == lexer.TOKEN_LPAREN {
			p.nextToken()
		}
		port := p.parseExpression(PREC_LOWEST)
		if p.curTok.Type == lexer.TOKEN_RPAREN {
			p.nextToken()
		}
		return &ast.InExpr{Port: port, Line: line}

	case lexer.TOKEN_LPAREN:
		p.nextToken()
		exp := p.parseExpression(PREC_LOWEST)
		if p.curTok.Type != lexer.TOKEN_RPAREN {
			p.addError("Schließende Klammer ')' erwartet")
			return nil
		}
		p.nextToken()
		return exp

	default:
		p.addError(fmt.Sprintf("Kein gültiger Ausdrucksanfang: %s", p.curTok.Lexeme))
		p.nextToken()
		return nil
	}
}

func (p *Parser) parseInfix(left ast.Expr) ast.Expr {
	opTok := p.curTok
	prec := p.curPrecedence()
	p.nextToken()

	right := p.parseExpression(prec)
	return &ast.BinaryExpr{
		Left:  left,
		Op:    opTok.Lexeme,
		Right: right,
		Line:  opTok.Line,
	}
}

func (p *Parser) curPrecedence() int {
	if prec, ok := precedences[p.curTok.Type]; ok {
		return prec
	}
	return PREC_LOWEST
}

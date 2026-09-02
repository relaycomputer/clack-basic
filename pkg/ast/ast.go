package ast

/*
 ==============================================================================
 🌳 ABSTRAKTER SYNTAXBAUM (AST) FÜR CLACK-BASIC
 ==============================================================================
 Der AST bildet das vom Parser eingelesene BASIC-Programm als Baumstruktur
 von Anweisungen (Statements) und Ausdrücken (Expressions) ab.
 ==============================================================================
*/

import "fmt"

// Node ist die Basisschnittstelle für jeden Knoten im Syntaxbaum.
type Node interface {
	Pos() int
}

// Expr repräsentiert einen Ausdruck, der zu einem 16-Bit-Wert ausgewertet werden kann.
type Expr interface {
	Node
	exprNode()
}

// Stmt repräsentiert eine ausführbare BASIC-Anweisung.
type Stmt interface {
	Node
	stmtNode()
}

// Program enthält die geordnete Liste aller Statements des Programms.
type Program struct {
	Statements []Stmt
}

func (p *Program) Pos() int { return 1 }

// ============================================================================
// AUSDRÜCKE (EXPRESSIONS)
// ============================================================================

// NumberLiteral: 16-Bit Zahlenkonstante (z.B. 100, 0x1400)
type NumberLiteral struct {
	Value int
	Line  int
}

func (n *NumberLiteral) Pos() int    { return n.Line }
func (n *NumberLiteral) exprNode()   {}
func (n *NumberLiteral) String() string { return fmt.Sprintf("%d", n.Value) }

// VariableExpr: Variablenzugriff (z.B. A, X, COUNTER)
type VariableExpr struct {
	Name string
	Line int
}

func (v *VariableExpr) Pos() int    { return v.Line }
func (v *VariableExpr) exprNode()   {}
func (v *VariableExpr) String() string { return v.Name }

// UnaryExpr: Unärer Operator (z.B. -X oder NOT A)
type UnaryExpr struct {
	Op    string
	Right Expr
	Line  int
}

func (u *UnaryExpr) Pos() int    { return u.Line }
func (u *UnaryExpr) exprNode()   {}

// BinaryExpr: Binäre Operation (z.B. A + B, X * 4, C AND 0xFF)
type BinaryExpr struct {
	Left  Expr
	Op    string // +, -, *, /, MOD, AND, OR, XOR, SHL, SHR, =, <>, <, <=, >, >=
	Right Expr
	Line  int
}

func (b *BinaryExpr) Pos() int    { return b.Line }
func (b *BinaryExpr) exprNode()   {}

// PeekExpr: Liest RAM-Zelle aus -> PEEK(Adresse)
type PeekExpr struct {
	Address Expr
	Line    int
}

func (p *PeekExpr) Pos() int    { return p.Line }
func (p *PeekExpr) exprNode()   {}

// InExpr: Liest Hardware-Port aus -> IN(Port)
type InExpr struct {
	Port Expr
	Line int
}

func (i *InExpr) Pos() int    { return i.Line }
func (i *InExpr) exprNode()   {}

// ============================================================================
// ANWEISUNGEN (STATEMENTS)
// ============================================================================

// LabelStmt: Sprungmarke oder Zeilennummer (z.B. "10" oder "START:")
type LabelStmt struct {
	Name string
	Line int
}

func (l *LabelStmt) Pos() int  { return l.Line }
func (l *LabelStmt) stmtNode() {}

// LetStmt: Variablenzuweisung -> [LET] Variable = Ausdruck
type LetStmt struct {
	VarName string
	Value   Expr
	Line    int
}

func (l *LetStmt) Pos() int  { return l.Line }
func (l *LetStmt) stmtNode() {}

// GotoStmt: Unbedingter Sprung -> GOTO Label/Line
type GotoStmt struct {
	Target string
	Line   int
}

func (g *GotoStmt) Pos() int  { return g.Line }
func (g *GotoStmt) stmtNode() {}

// GosubStmt: Unterprogrammaufruf -> GOSUB Label/Line
type GosubStmt struct {
	Target string
	Line   int
}

func (g *GosubStmt) Pos() int  { return g.Line }
func (g *GosubStmt) stmtNode() {}

// ReturnStmt: Rücksprung -> RETURN
type ReturnStmt struct {
	Line int
}

func (r *ReturnStmt) Pos() int  { return r.Line }
func (r *ReturnStmt) stmtNode() {}

// IfStmt: Bedingte Verzweigung -> IF Condition THEN Stmt [ELSE Stmt]
type IfStmt struct {
	Condition  Expr
	ThenBranch []Stmt
	ElseBranch []Stmt
	Line       int
}

func (i *IfStmt) Pos() int  { return i.Line }
func (i *IfStmt) stmtNode() {}

// ForStmt: Zählschleife -> FOR Var = Start TO End [STEP Step] ... NEXT Var
type ForStmt struct {
	VarName string
	Start   Expr
	End     Expr
	Step    Expr // Optional (Standard: 1)
	Body    []Stmt
	Line    int
}

func (f *ForStmt) Pos() int  { return f.Line }
func (f *ForStmt) stmtNode() {}

// DataStmt: Datentabelle -> DATA 10, 20, 30
type DataStmt struct {
	Values []int
	Line   int
}

func (d *DataStmt) Pos() int  { return d.Line }
func (d *DataStmt) stmtNode() {}

// ReadStmt: Liest nächste Werte aus DATA -> READ A, B
type ReadStmt struct {
	VarNames []string
	Line     int
}

func (r *ReadStmt) Pos() int  { return r.Line }
func (r *ReadStmt) stmtNode() {}

// RestoreStmt: Setzt DATA-Zeiger zurück -> RESTORE
type RestoreStmt struct {
	Line int
}

func (r *RestoreStmt) Pos() int  { return r.Line }
func (r *RestoreStmt) stmtNode() {}

// PokeStmt: Schreibt direkt in RAM -> POKE Adresse, Wert
type PokeStmt struct {
	Address Expr
	Value   Expr
	Line    int
}

func (p *PokeStmt) Pos() int  { return p.Line }
func (p *PokeStmt) stmtNode() {}

// OutStmt: Schreibt auf Hardware-Port -> OUT Port, Wert
type OutStmt struct {
	Port  Expr
	Value Expr
	Line  int
}

func (o *OutStmt) Pos() int  { return o.Line }
func (o *OutStmt) stmtNode() {}

// ClsStmt: Löscht Display Matrix (Worte 960..1023) -> CLS
type ClsStmt struct {
	Line int
}

func (c *ClsStmt) Pos() int  { return c.Line }
func (c *ClsStmt) stmtNode() {}

// PsetStmt: Setzt Pixel auf LED-Matrix -> PSET X, Y, Color
type PsetStmt struct {
	X     Expr
	Y     Expr
	Color Expr // 1 = an, 0 = aus
	Line  int
}

func (p *PsetStmt) Pos() int  { return p.Line }
func (p *PsetStmt) stmtNode() {}

// PrintStmt: Ausgabe (für Monitor/Display) -> PRINT "Text", Variable
type PrintStmt struct {
	Items []Expr // String- oder Num-Expressions
	Line  int
}

func (p *PrintStmt) Pos() int  { return p.Line }
func (p *PrintStmt) stmtNode() {}

// EndStmt: Programm anhalten -> END / HALT / STOP
type EndStmt struct {
	Line int
}

func (e *EndStmt) Pos() int  { return e.Line }
func (e *EndStmt) stmtNode() {}

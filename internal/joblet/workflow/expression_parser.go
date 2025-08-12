package workflow

import (
	"fmt"
	"strings"
	"unicode"
)

// Token types for expression parsing
type TokenType int

const (
	TokenEOF TokenType = iota
	TokenIdentifier
	TokenEquals
	TokenNotEquals
	TokenAnd
	TokenOr
	TokenNot
	TokenIn
	TokenNotIn
	TokenLeftParen
	TokenRightParen
	TokenComma
	TokenString
)

// Token represents a lexical token
type Token struct {
	Type  TokenType
	Value string
}

// Lexer tokenizes expression strings
type Lexer struct {
	input   string
	pos     int
	current rune
}

// NewLexer creates a new lexical analyzer for tokenizing dependency expressions.
// The lexer breaks down complex expressions into tokens (identifiers, operators, etc.)
// that can be processed by the expression parser to build an Abstract Syntax Tree (AST).
func NewLexer(input string) *Lexer {
	l := &Lexer{input: input, pos: 0}
	l.readChar()
	return l
}

// readChar advances the lexer's position and updates the current character.
// Handles end-of-input by setting current to 0 (EOF marker).
func (l *Lexer) readChar() {
	if l.pos >= len(l.input) {
		l.current = 0
	} else {
		l.current = rune(l.input[l.pos])
	}
	l.pos++
}

// peekChar looks at the next character without consuming it.
// Used for lookahead during tokenization (e.g., distinguishing '=' from '==').
func (l *Lexer) peekChar() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return rune(l.input[l.pos])
}

// skipWhitespace consumes all whitespace characters (spaces, tabs, newlines).
// Called before tokenizing to ignore whitespace between meaningful tokens.
func (l *Lexer) skipWhitespace() {
	for unicode.IsSpace(l.current) {
		l.readChar()
	}
}

// readIdentifier reads a complete identifier (job names, keywords like AND/OR).
// Identifiers can contain letters, digits, underscores, and hyphens.
func (l *Lexer) readIdentifier() string {
	start := l.pos - 1
	for isIdentifierChar(l.current) {
		l.readChar()
	}
	return l.input[start : l.pos-1]
}

// readString reads a quoted string value (for job statuses in expressions).
// Handles both single and double quotes, returning the content without quotes.
func (l *Lexer) readString() string {
	l.readChar() // Skip opening quote
	start := l.pos - 1
	for l.current != '"' && l.current != '\'' && l.current != 0 {
		l.readChar()
	}
	result := l.input[start : l.pos-1]
	l.readChar() // Skip closing quote
	return result
}

// isIdentifierChar determines if a character is valid within an identifier.
// Allows alphanumeric characters, underscores, and hyphens for job names.
func isIdentifierChar(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' || ch == '-'
}

// NextToken scans the input and returns the next lexical token.
// Recognizes operators (=, !=, AND, OR), parentheses, identifiers, and string literals.
// This is the main method used by the parser to consume tokens sequentially.
func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	tok := Token{}

	switch l.current {
	case 0:
		tok.Type = TokenEOF
	case '(':
		tok.Type = TokenLeftParen
		tok.Value = "("
		l.readChar()
	case ')':
		tok.Type = TokenRightParen
		tok.Value = ")"
		l.readChar()
	case ',':
		tok.Type = TokenComma
		tok.Value = ","
		l.readChar()
	case '=':
		if l.peekChar() == '=' {
			l.readChar()
			l.readChar()
			tok.Type = TokenEquals
			tok.Value = "=="
		} else {
			tok.Type = TokenEquals
			tok.Value = "="
			l.readChar()
		}
	case '!':
		if l.peekChar() == '=' {
			l.readChar()
			l.readChar()
			tok.Type = TokenNotEquals
			tok.Value = "!="
		} else {
			tok.Type = TokenNot
			tok.Value = "!"
			l.readChar()
		}
	case '<':
		if l.peekChar() == '>' {
			l.readChar()
			l.readChar()
			tok.Type = TokenNotEquals
			tok.Value = "<>"
		}
	case '"', '\'':
		tok.Type = TokenString
		tok.Value = l.readString()
	case '&':
		if l.peekChar() == '&' {
			l.readChar()
			l.readChar()
			tok.Type = TokenAnd
			tok.Value = "&&"
		}
	case '|':
		if l.peekChar() == '|' {
			l.readChar()
			l.readChar()
			tok.Type = TokenOr
			tok.Value = "||"
		}
	default:
		if unicode.IsLetter(l.current) {
			ident := l.readIdentifier()
			switch strings.ToUpper(ident) {
			case "AND":
				tok.Type = TokenAnd
				tok.Value = "AND"
			case "OR":
				tok.Type = TokenOr
				tok.Value = "OR"
			case "NOT":
				tok.Type = TokenNot
				tok.Value = "NOT"
			case "IN":
				tok.Type = TokenIn
				tok.Value = "IN"
			case "NOT_IN":
				tok.Type = TokenNotIn
				tok.Value = "NOT_IN"
			default:
				tok.Type = TokenIdentifier
				tok.Value = ident
			}
		} else {
			tok.Type = TokenIdentifier
			tok.Value = string(l.current)
			l.readChar()
		}
	}

	return tok
}

// ExpressionNode represents a node in the expression AST
type ExpressionNode interface {
	Evaluate(jobStates map[string]string) (bool, error)
	CanBeSatisfied(jobStates map[string]string, terminalStates map[string]bool) bool
	GetJobNames() []string
}

// BinaryOpNode represents a binary operation (AND, OR)
type BinaryOpNode struct {
	Left     ExpressionNode
	Right    ExpressionNode
	Operator TokenType
}

// Evaluate computes the boolean result of a binary operation (AND/OR).
// Recursively evaluates both left and right child nodes and applies the operator.
// Used during dependency checking to determine if complex expressions are satisfied.
func (n *BinaryOpNode) Evaluate(jobStates map[string]string) (bool, error) {
	leftVal, err := n.Left.Evaluate(jobStates)
	if err != nil {
		return false, err
	}

	rightVal, err := n.Right.Evaluate(jobStates)
	if err != nil {
		return false, err
	}

	switch n.Operator {
	case TokenAnd:
		return leftVal && rightVal, nil
	case TokenOr:
		return leftVal || rightVal, nil
	default:
		return false, fmt.Errorf("unknown binary operator: %v", n.Operator)
	}
}

// CanBeSatisfied determines if a binary expression can potentially be satisfied.
// For AND operations, both sides must be satisfiable.
// For OR operations, at least one side must be satisfiable.
// Used to detect impossible dependencies early and cancel dependent jobs.
func (n *BinaryOpNode) CanBeSatisfied(jobStates map[string]string, terminalStates map[string]bool) bool {
	switch n.Operator {
	case TokenAnd:
		// Both sides must be satisfiable
		return n.Left.CanBeSatisfied(jobStates, terminalStates) &&
			n.Right.CanBeSatisfied(jobStates, terminalStates)
	case TokenOr:
		// At least one side must be satisfiable
		return n.Left.CanBeSatisfied(jobStates, terminalStates) ||
			n.Right.CanBeSatisfied(jobStates, terminalStates)
	default:
		return false
	}
}

// GetJobNames extracts all job names referenced in the binary expression.
// Recursively collects job names from both left and right child nodes.
// Used for dependency analysis and workflow planning.
func (n *BinaryOpNode) GetJobNames() []string {
	var names []string
	names = append(names, n.Left.GetJobNames()...)
	names = append(names, n.Right.GetJobNames()...)
	return names
}

// UnaryOpNode represents a unary operation (NOT)
type UnaryOpNode struct {
	Child    ExpressionNode
	Operator TokenType
}

// Evaluate computes the boolean result of a unary operation (NOT).
// Evaluates the child node and applies the negation operator.
// Supports negative dependency conditions (e.g., "NOT jobA=FAILED").
func (n *UnaryOpNode) Evaluate(jobStates map[string]string) (bool, error) {
	childVal, err := n.Child.Evaluate(jobStates)
	if err != nil {
		return false, err
	}

	switch n.Operator {
	case TokenNot:
		return !childVal, nil
	default:
		return false, fmt.Errorf("unknown unary operator: %v", n.Operator)
	}
}

// CanBeSatisfied determines if a unary expression can potentially be satisfied.
// For NOT operations, this is generally true since jobs can reach various states.
// Simplified implementation - could be enhanced for more precise analysis.
func (n *UnaryOpNode) CanBeSatisfied(jobStates map[string]string, terminalStates map[string]bool) bool {
	// NOT can be satisfied if the child can reach a state that makes it false
	return true // Simplified implementation
}

// GetJobNames extracts job names from the unary expression's child node.
// Delegates to the child node since NOT doesn't introduce new job references.
func (n *UnaryOpNode) GetJobNames() []string {
	return n.Child.GetJobNames()
}

// ComparisonNode represents a comparison (job=status, job IN (...))
type ComparisonNode struct {
	JobName  string
	Operator TokenType
	Values   []string
}

// Evaluate checks if a job comparison is satisfied (e.g., jobA=COMPLETED).
// Supports equality (=), inequality (!=), and set membership (IN, NOT_IN) operations.
// Returns false if the referenced job doesn't exist or hasn't started yet.
func (n *ComparisonNode) Evaluate(jobStates map[string]string) (bool, error) {
	currentStatus, exists := jobStates[n.JobName]
	if !exists {
		return false, nil // Job doesn't exist or hasn't started
	}

	switch n.Operator {
	case TokenEquals:
		return currentStatus == n.Values[0], nil
	case TokenNotEquals:
		return currentStatus != n.Values[0], nil
	case TokenIn:
		for _, v := range n.Values {
			if currentStatus == v {
				return true, nil
			}
		}
		return false, nil
	case TokenNotIn:
		for _, v := range n.Values {
			if currentStatus == v {
				return false, nil
			}
		}
		return true, nil
	default:
		return false, fmt.Errorf("unknown comparison operator: %v", n.Operator)
	}
}

// CanBeSatisfied determines if a job comparison can potentially be satisfied.
// If a job is in a terminal state, checks if the requirement is already met.
// If a job hasn't reached a terminal state, assumes it can potentially satisfy the requirement.
func (n *ComparisonNode) CanBeSatisfied(jobStates map[string]string, terminalStates map[string]bool) bool {
	_, exists := jobStates[n.JobName]

	// If job is in terminal state, check if requirement is satisfied
	if exists && terminalStates[n.JobName] {
		satisfied, _ := n.Evaluate(jobStates)
		return satisfied
	}

	// If job hasn't reached terminal state, it can potentially satisfy the requirement
	return true
}

// GetJobNames returns the single job name referenced in this comparison.
// Each comparison node references exactly one job for status checking.
func (n *ComparisonNode) GetJobNames() []string {
	return []string{n.JobName}
}

// ExpressionParser parses boolean expressions into AST
type ExpressionParser struct {
	lexer   *Lexer
	current Token
}

// NewExpressionParser creates a new recursive descent parser for dependency expressions.
// The parser builds an Abstract Syntax Tree (AST) from tokenized input using operator precedence:
// 1. OR (lowest precedence)
// 2. AND (higher precedence)
// 3. NOT (highest precedence)
// 4. Comparisons and parentheses (primary expressions)
func NewExpressionParser(input string) *ExpressionParser {
	lexer := NewLexer(input)
	parser := &ExpressionParser{lexer: lexer}
	parser.nextToken()
	return parser
}

// nextToken advances the parser to the next token from the lexer.
// Called throughout the parsing process to consume tokens sequentially.
func (p *ExpressionParser) nextToken() {
	p.current = p.lexer.NextToken()
}

// Parse performs the complete parsing of the expression into an AST.
// Starts with the lowest precedence operator (OR) and builds the tree bottom-up.
// Returns the root node of the AST for later evaluation or analysis.
func (p *ExpressionParser) Parse() (ExpressionNode, error) {
	return p.parseOr()
}

// parseOr handles OR operations (lowest precedence level).
// Creates left-associative binary operations for multiple OR expressions.
// Example: "A OR B OR C" becomes "(A OR B) OR C"
func (p *ExpressionParser) parseOr() (ExpressionNode, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	for p.current.Type == TokenOr {
		p.nextToken()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &BinaryOpNode{
			Left:     left,
			Right:    right,
			Operator: TokenOr,
		}
	}

	return left, nil
}

// parseAnd handles AND operations (higher precedence than OR).
// Creates left-associative binary operations for multiple AND expressions.
// Example: "A AND B AND C" becomes "(A AND B) AND C"
func (p *ExpressionParser) parseAnd() (ExpressionNode, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}

	for p.current.Type == TokenAnd {
		p.nextToken()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &BinaryOpNode{
			Left:     left,
			Right:    right,
			Operator: TokenAnd,
		}
	}

	return left, nil
}

// parseNot handles NOT operations (highest precedence).
// Creates unary operation nodes for negation of primary expressions.
// Example: "NOT jobA=FAILED" negates the comparison result.
func (p *ExpressionParser) parseNot() (ExpressionNode, error) {
	if p.current.Type == TokenNot {
		p.nextToken()
		child, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return &UnaryOpNode{
			Child:    child,
			Operator: TokenNot,
		}, nil
	}

	return p.parsePrimary()
}

// parsePrimary handles primary expressions: comparisons and parenthesized expressions.
// Supports job comparisons (job=status), IN operations (job IN (status1,status2)),
// and parenthesized sub-expressions for grouping.
func (p *ExpressionParser) parsePrimary() (ExpressionNode, error) {
	// Handle parentheses
	if p.current.Type == TokenLeftParen {
		p.nextToken()
		node, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.current.Type != TokenRightParen {
			return nil, fmt.Errorf("expected closing parenthesis")
		}
		p.nextToken()
		return node, nil
	}

	// Handle job comparisons
	if p.current.Type == TokenIdentifier {
		jobName := p.current.Value
		p.nextToken()

		switch p.current.Type {
		case TokenEquals, TokenNotEquals:
			op := p.current.Type
			p.nextToken()
			if p.current.Type != TokenIdentifier && p.current.Type != TokenString {
				return nil, fmt.Errorf("expected status value after operator")
			}
			value := p.current.Value
			p.nextToken()
			return &ComparisonNode{
				JobName:  jobName,
				Operator: op,
				Values:   []string{value},
			}, nil

		case TokenIn, TokenNotIn:
			op := p.current.Type
			p.nextToken()
			if p.current.Type != TokenLeftParen {
				return nil, fmt.Errorf("expected '(' after IN/NOT_IN")
			}
			p.nextToken()

			var values []string
			for p.current.Type != TokenRightParen {
				if p.current.Type != TokenIdentifier && p.current.Type != TokenString {
					return nil, fmt.Errorf("expected status value in list")
				}
				values = append(values, p.current.Value)
				p.nextToken()

				if p.current.Type == TokenComma {
					p.nextToken()
				} else if p.current.Type != TokenRightParen {
					return nil, fmt.Errorf("expected ',' or ')' in list")
				}
			}
			p.nextToken() // Skip closing paren

			return &ComparisonNode{
				JobName:  jobName,
				Operator: op,
				Values:   values,
			}, nil

		default:
			return nil, fmt.Errorf("expected comparison operator after job name")
		}
	}

	return nil, fmt.Errorf("unexpected token: %v", p.current)
}

// ParseExpression is a convenience function that creates a parser and parses an expression.
// Returns the root AST node for the given expression string.
// This is the main entry point for parsing dependency expressions.
func ParseExpression(expr string) (ExpressionNode, error) {
	parser := NewExpressionParser(expr)
	return parser.Parse()
}

// EvaluateExpression parses and evaluates an expression in one step.
// Takes an expression string and current job states, returns the boolean result.
// Combines parsing and evaluation for simple one-time expression checks.
func EvaluateExpression(expr string, jobStates map[string]string) (bool, error) {
	node, err := ParseExpression(expr)
	if err != nil {
		return false, err
	}
	return node.Evaluate(jobStates)
}

// CanExpressionBeSatisfied determines if an expression can potentially become true.
// Analyzes the expression with current job states and terminal state information
// to detect impossible dependencies early in workflow execution.
func CanExpressionBeSatisfied(expr string, jobStates map[string]string, terminalStates map[string]bool) (bool, error) {
	node, err := ParseExpression(expr)
	if err != nil {
		return false, err
	}
	return node.CanBeSatisfied(jobStates, terminalStates), nil
}

// GetExpressionJobNames parses an expression and extracts all referenced job names.
// Used for dependency analysis, workflow planning, and validation.
// Helps identify which jobs need to be monitored for dependency resolution.
func GetExpressionJobNames(expr string) ([]string, error) {
	node, err := ParseExpression(expr)
	if err != nil {
		return nil, err
	}
	return node.GetJobNames(), nil
}

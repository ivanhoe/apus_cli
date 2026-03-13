package pbxproj

import (
	"fmt"
	"strings"
)

// Lexer tokenizes a pbxproj (OpenStep ASCII plist) source string.
type Lexer struct {
	src  string
	pos  int
	line int
	col  int
}

// NewLexer creates a lexer for the given source.
func NewLexer(src string) *Lexer {
	return &Lexer{src: src, line: 1, col: 1}
}

// Next returns the next token, advancing the position.
// Whitespace is silently skipped. Returns TokenEOF at end of input.
func (l *Lexer) Next() (Token, error) {
	l.skipWhitespace()

	if l.pos >= len(l.src) {
		return Token{Kind: TokenEOF, Pos: l.pos, Line: l.line, Col: l.col}, nil
	}

	ch := l.src[l.pos]

	// Single-character tokens
	if kind, ok := singleCharToken(ch); ok {
		tok := Token{Kind: kind, Value: string(ch), Pos: l.pos, Line: l.line, Col: l.col}
		l.advance(1)
		return tok, nil
	}

	// Comment: /* ... */
	if ch == '/' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '*' {
		return l.scanComment()
	}

	// Line comment: // ... (the first line of pbxproj files: // !$*UTF8*$!)
	if ch == '/' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '/' {
		return l.scanLineComment()
	}

	// Quoted string: "..."
	if ch == '"' {
		return l.scanQuotedString()
	}

	// Data: <hex>
	if ch == '<' {
		return l.scanData()
	}

	// Unquoted string: alphanumeric, dots, slashes, hyphens, underscores
	if isUnquotedChar(ch) {
		return l.scanUnquotedString()
	}

	return Token{}, l.errorf("unexpected character %q", ch)
}

func singleCharToken(ch byte) (TokenKind, bool) {
	switch ch {
	case '{':
		return TokenLBrace, true
	case '}':
		return TokenRBrace, true
	case '(':
		return TokenLParen, true
	case ')':
		return TokenRParen, true
	case '=':
		return TokenEquals, true
	case ';':
		return TokenSemicolon, true
	case ',':
		return TokenComma, true
	default:
		return 0, false
	}
}

func (l *Lexer) scanComment() (Token, error) {
	start := l.pos
	startLine := l.line
	startCol := l.col
	l.advance(2) // skip /*

	end := strings.Index(l.src[l.pos:], "*/")
	if end == -1 {
		return Token{}, l.errorf("unterminated block comment")
	}

	body := l.src[l.pos : l.pos+end]
	l.advanceString(l.src[l.pos : l.pos+end+2]) // skip body + */

	return Token{
		Kind:  TokenComment,
		Value: strings.TrimSpace(body),
		Pos:   start,
		Line:  startLine,
		Col:   startCol,
	}, nil
}

func (l *Lexer) scanLineComment() (Token, error) {
	start := l.pos
	startLine := l.line
	startCol := l.col
	l.advance(2) // skip //

	end := strings.IndexByte(l.src[l.pos:], '\n')
	var body string
	if end == -1 {
		body = l.src[l.pos:]
		l.pos = len(l.src)
	} else {
		body = l.src[l.pos : l.pos+end]
		l.advanceString(l.src[l.pos : l.pos+end+1]) // include \n
	}

	return Token{
		Kind:  TokenComment,
		Value: strings.TrimSpace(body),
		Pos:   start,
		Line:  startLine,
		Col:   startCol,
	}, nil
}

func (l *Lexer) scanQuotedString() (Token, error) {
	start := l.pos
	startLine := l.line
	startCol := l.col
	l.advance(1) // skip opening "

	var b strings.Builder
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == '\\' && l.pos+1 < len(l.src) {
			next := l.src[l.pos+1]
			switch next {
			case '"', '\\', '\'':
				b.WriteByte(next)
				l.advance(2)
			case 'n':
				b.WriteByte('\n')
				l.advance(2)
			case 't':
				b.WriteByte('\t')
				l.advance(2)
			default:
				b.WriteByte('\\')
				b.WriteByte(next)
				l.advance(2)
			}
			continue
		}
		if ch == '"' {
			l.advance(1) // skip closing "
			return Token{
				Kind:   TokenString,
				Value:  b.String(),
				Pos:    start,
				Line:   startLine,
				Col:    startCol,
				Quoted: true,
			}, nil
		}
		if ch == '\n' {
			l.line++
			l.col = 0
		}
		b.WriteByte(ch)
		l.advance(1)
	}
	return Token{}, l.errorf("unterminated quoted string")
}

func (l *Lexer) scanData() (Token, error) {
	start := l.pos
	startLine := l.line
	startCol := l.col
	l.advance(1) // skip <

	end := strings.IndexByte(l.src[l.pos:], '>')
	if end == -1 {
		return Token{}, l.errorf("unterminated data literal")
	}
	hex := strings.TrimSpace(l.src[l.pos : l.pos+end])
	l.advance(end + 1) // skip content + >

	return Token{
		Kind:  TokenData,
		Value: hex,
		Pos:   start,
		Line:  startLine,
		Col:   startCol,
	}, nil
}

func (l *Lexer) scanUnquotedString() (Token, error) {
	start := l.pos
	startLine := l.line
	startCol := l.col

	for l.pos < len(l.src) && isUnquotedChar(l.src[l.pos]) {
		l.pos++
		l.col++
	}

	return Token{
		Kind:  TokenString,
		Value: l.src[start:l.pos],
		Pos:   start,
		Line:  startLine,
		Col:   startCol,
	}, nil
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == '\n' {
			l.pos++
			l.line++
			l.col = 1
		} else if ch == ' ' || ch == '\t' || ch == '\r' {
			l.pos++
			l.col++
		} else {
			return
		}
	}
}

func (l *Lexer) advance(n int) {
	for i := 0; i < n && l.pos < len(l.src); i++ {
		if l.src[l.pos] == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
		l.pos++
	}
}

func (l *Lexer) advanceString(s string) {
	l.advance(len(s))
}

func (l *Lexer) errorf(format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("lexer: line %d col %d: %s", l.line, l.col, msg)
}

// isUnquotedChar returns true for characters valid in an unquoted plist string.
// This includes letters, digits, and common path/identifier characters that
// appear unquoted in real pbxproj files.
func isUnquotedChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') ||
		ch == '_' || ch == '.' || ch == '/' || ch == '-'
}

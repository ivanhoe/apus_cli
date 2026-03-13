// Package pbxproj provides a parser, AST, and serializer for Xcode
// project.pbxproj files (OpenStep ASCII plist format).
package pbxproj

import "fmt"

// TokenKind classifies a lexical token.
type TokenKind int

const (
	TokenLBrace    TokenKind = iota // {
	TokenRBrace                     // }
	TokenLParen                     // (
	TokenRParen                     // )
	TokenEquals                     // =
	TokenSemicolon                  // ;
	TokenComma                      // ,
	TokenString                     // quoted or unquoted string value
	TokenComment                    // /* ... */
	TokenData                       // < hex bytes >
	TokenEOF
)

func (k TokenKind) String() string {
	switch k {
	case TokenLBrace:
		return "{"
	case TokenRBrace:
		return "}"
	case TokenLParen:
		return "("
	case TokenRParen:
		return ")"
	case TokenEquals:
		return "="
	case TokenSemicolon:
		return ";"
	case TokenComma:
		return ","
	case TokenString:
		return "string"
	case TokenComment:
		return "comment"
	case TokenData:
		return "data"
	case TokenEOF:
		return "EOF"
	default:
		return fmt.Sprintf("TokenKind(%d)", k)
	}
}

// Token represents a single lexical element from a pbxproj source file.
type Token struct {
	Kind   TokenKind
	Value  string // string content (unquoted), comment body, or hex data
	Pos    int    // byte offset in source
	Line   int
	Col    int
	Quoted bool // for TokenString: true if the original was "quoted"
}

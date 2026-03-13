package pbxproj

import (
	"testing"
)

func TestLexer_SingleCharTokens(t *testing.T) {
	input := "{}()=;,"
	l := NewLexer(input)

	expected := []TokenKind{
		TokenLBrace, TokenRBrace, TokenLParen, TokenRParen,
		TokenEquals, TokenSemicolon, TokenComma, TokenEOF,
	}

	for _, want := range expected {
		tok, err := l.Next()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tok.Kind != want {
			t.Fatalf("expected %s, got %s", want, tok.Kind)
		}
	}
}

func TestLexer_UnquotedString(t *testing.T) {
	l := NewLexer("AABBCCDD1122334455667788")
	tok, err := l.Next()
	if err != nil {
		t.Fatal(err)
	}
	if tok.Kind != TokenString || tok.Value != "AABBCCDD1122334455667788" {
		t.Fatalf("got %+v", tok)
	}
	if tok.Quoted {
		t.Fatal("expected unquoted")
	}
}

func TestLexer_QuotedString(t *testing.T) {
	l := NewLexer(`"hello world"`)
	tok, err := l.Next()
	if err != nil {
		t.Fatal(err)
	}
	if tok.Kind != TokenString || tok.Value != "hello world" {
		t.Fatalf("got %+v", tok)
	}
	if !tok.Quoted {
		t.Fatal("expected quoted")
	}
}

func TestLexer_QuotedStringWithEscapes(t *testing.T) {
	l := NewLexer(`"path/to/\"file\""`)
	tok, err := l.Next()
	if err != nil {
		t.Fatal(err)
	}
	if tok.Value != `path/to/"file"` {
		t.Fatalf("got %q", tok.Value)
	}
}

func TestLexer_BlockComment(t *testing.T) {
	l := NewLexer("/* Begin PBXBuildFile section */")
	tok, err := l.Next()
	if err != nil {
		t.Fatal(err)
	}
	if tok.Kind != TokenComment || tok.Value != "Begin PBXBuildFile section" {
		t.Fatalf("got %+v", tok)
	}
}

func TestLexer_LineComment(t *testing.T) {
	l := NewLexer("// !$*UTF8*$!\n{")
	tok, err := l.Next()
	if err != nil {
		t.Fatal(err)
	}
	if tok.Kind != TokenComment || tok.Value != "!$*UTF8*$!" {
		t.Fatalf("got %+v", tok)
	}
	tok, err = l.Next()
	if err != nil {
		t.Fatal(err)
	}
	if tok.Kind != TokenLBrace {
		t.Fatalf("expected lbrace after line comment, got %s", tok.Kind)
	}
}

func TestLexer_DataLiteral(t *testing.T) {
	l := NewLexer("<0fbd777 1c2735ae>")
	tok, err := l.Next()
	if err != nil {
		t.Fatal(err)
	}
	if tok.Kind != TokenData || tok.Value != "0fbd777 1c2735ae" {
		t.Fatalf("got %+v", tok)
	}
}

func TestLexer_WhitespaceHandling(t *testing.T) {
	l := NewLexer("  \t\n  key  \n  =  \n  val  ")

	tok, _ := l.Next()
	if tok.Kind != TokenString || tok.Value != "key" {
		t.Fatalf("expected key, got %+v", tok)
	}

	tok, _ = l.Next()
	if tok.Kind != TokenEquals {
		t.Fatalf("expected =, got %s", tok.Kind)
	}

	tok, _ = l.Next()
	if tok.Kind != TokenString || tok.Value != "val" {
		t.Fatalf("expected val, got %+v", tok)
	}
}

func TestLexer_LineTracking(t *testing.T) {
	l := NewLexer("a\nb\nc")

	tok, _ := l.Next()
	if tok.Line != 1 {
		t.Fatalf("expected line 1, got %d", tok.Line)
	}

	tok, _ = l.Next()
	if tok.Line != 2 {
		t.Fatalf("expected line 2, got %d", tok.Line)
	}

	tok, _ = l.Next()
	if tok.Line != 3 {
		t.Fatalf("expected line 3, got %d", tok.Line)
	}
}

func TestLexer_UnterminatedString(t *testing.T) {
	l := NewLexer(`"unterminated`)
	_, err := l.Next()
	if err == nil {
		t.Fatal("expected error for unterminated string")
	}
}

func TestLexer_UnterminatedComment(t *testing.T) {
	l := NewLexer("/* unterminated")
	_, err := l.Next()
	if err == nil {
		t.Fatal("expected error for unterminated comment")
	}
}

func TestLexer_PathCharsInUnquotedString(t *testing.T) {
	l := NewLexer("com.apple.product-type.application")
	tok, _ := l.Next()
	if tok.Value != "com.apple.product-type.application" {
		t.Fatalf("expected full dotted path, got %q", tok.Value)
	}
}

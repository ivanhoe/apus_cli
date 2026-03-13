package pbxproj

import "fmt"

// Parse parses a pbxproj (OpenStep ASCII plist) source into an AST Dict.
// The source typically starts with a // comment line followed by the root dict.
func Parse(src string) (*Dict, error) {
	p := &parser{lexer: NewLexer(src)}
	if err := p.readToken(); err != nil {
		return nil, err
	}

	// Skip leading line comments (e.g. // !$*UTF8*$!)
	for p.cur.Kind == TokenComment {
		if err := p.readToken(); err != nil {
			return nil, err
		}
	}

	dict, err := p.parseDict()
	if err != nil {
		return nil, err
	}
	return dict, nil
}

type parser struct {
	lexer *Lexer
	cur   Token
}

func (p *parser) readToken() error {
	tok, err := p.lexer.Next()
	if err != nil {
		return err
	}
	p.cur = tok
	return nil
}

func (p *parser) expect(kind TokenKind) error {
	if p.cur.Kind != kind {
		return p.errorf("expected %s, got %s (%q)", kind, p.cur.Kind, p.cur.Value)
	}
	return nil
}

func (p *parser) consumeComment() string {
	if p.cur.Kind != TokenComment {
		return ""
	}
	comment := p.cur.Value
	_ = p.readToken()
	return comment
}

// parseDict parses { entry; entry; ... }
func (p *parser) parseDict() (*Dict, error) {
	if err := p.expect(TokenLBrace); err != nil {
		return nil, err
	}
	if err := p.readToken(); err != nil {
		return nil, err
	}

	dict := &Dict{}
	for p.cur.Kind != TokenRBrace && p.cur.Kind != TokenEOF {
		// Skip section comments like /* Begin/End PBXBuildFile section */
		for p.cur.Kind == TokenComment {
			_ = p.consumeComment()
		}
		if p.cur.Kind == TokenRBrace {
			break
		}

		entry, err := p.parseDictEntry()
		if err != nil {
			return nil, err
		}
		dict.Entries = append(dict.Entries, entry)
	}

	if err := p.expect(TokenRBrace); err != nil {
		return nil, err
	}
	if err := p.readToken(); err != nil {
		return nil, err
	}
	return dict, nil
}

// parseDictEntry parses: key /* comment */ = value ;
func (p *parser) parseDictEntry() (DictEntry, error) {
	// Key
	if err := p.expect(TokenString); err != nil {
		return DictEntry{}, fmt.Errorf("dict entry key: %w", err)
	}
	key := p.cur.Value
	if err := p.readToken(); err != nil {
		return DictEntry{}, err
	}

	// Optional comment after key
	keyComment := p.consumeComment()

	// =
	if err := p.expect(TokenEquals); err != nil {
		return DictEntry{}, fmt.Errorf("dict entry %q: %w", key, err)
	}
	if err := p.readToken(); err != nil {
		return DictEntry{}, err
	}

	// Value
	value, err := p.parseValue()
	if err != nil {
		return DictEntry{}, fmt.Errorf("dict entry %q value: %w", key, err)
	}

	// Optional comment after value (e.g. fileRef = AABB /* MyFile.swift */;)
	valueComment := p.consumeComment()

	// ;
	if err := p.expect(TokenSemicolon); err != nil {
		return DictEntry{}, fmt.Errorf("dict entry %q semicolon: %w", key, err)
	}
	if err := p.readToken(); err != nil {
		return DictEntry{}, err
	}

	return DictEntry{
		Key:          key,
		KeyComment:   keyComment,
		Value:        value,
		ValueComment: valueComment,
	}, nil
}

// parseValue parses a dict, array, string, or data value (with optional leading comment).
func (p *parser) parseValue() (Node, error) {
	// Skip leading comment
	_ = p.consumeComment()

	switch p.cur.Kind {
	case TokenLBrace:
		return p.parseDict()
	case TokenLParen:
		return p.parseArray()
	case TokenString:
		return p.parseString()
	case TokenData:
		return p.parseData()
	default:
		return nil, p.errorf("expected value, got %s (%q)", p.cur.Kind, p.cur.Value)
	}
}

// parseArray parses ( item, item, )
func (p *parser) parseArray() (*Array, error) {
	if err := p.expect(TokenLParen); err != nil {
		return nil, err
	}
	if err := p.readToken(); err != nil {
		return nil, err
	}

	arr := &Array{}
	for p.cur.Kind != TokenRParen && p.cur.Kind != TokenEOF {
		// Skip leading comments
		_ = p.consumeComment()
		if p.cur.Kind == TokenRParen {
			break
		}

		value, err := p.parseValue()
		if err != nil {
			return nil, fmt.Errorf("array item: %w", err)
		}

		// Optional inline comment after value
		comment := p.consumeComment()
		arr.Items = append(arr.Items, ArrayItem{Value: value, Comment: comment})

		// Optional trailing comma
		if p.cur.Kind == TokenComma {
			if err := p.readToken(); err != nil {
				return nil, err
			}
		}
	}

	if err := p.expect(TokenRParen); err != nil {
		return nil, err
	}
	if err := p.readToken(); err != nil {
		return nil, err
	}
	return arr, nil
}

func (p *parser) parseString() (*String, error) {
	s := &String{Value: p.cur.Value, Quoted: p.cur.Quoted}
	if err := p.readToken(); err != nil {
		return nil, err
	}

	// Consume inline comment that follows the string (e.g. "AABB /* name */")
	// but only if the next token is a comment and not a structural token.
	// We peek but don't consume here — the caller (parseDictEntry, parseArray)
	// handles comment consumption based on context.
	return s, nil
}

func (p *parser) parseData() (*Data, error) {
	d := &Data{Hex: p.cur.Value}
	if err := p.readToken(); err != nil {
		return nil, err
	}
	return d, nil
}

func (p *parser) errorf(format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("parser: line %d col %d: %s", p.cur.Line, p.cur.Col, msg)
}

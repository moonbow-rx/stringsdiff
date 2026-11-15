package main

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

// Entry is one key/value pair of a .strings file.
type Entry struct {
	Key   string
	Value string
	Line  int // 1-based line of the key
}

// ParseError describes a syntax error at a position in the decoded text.
type ParseError struct {
	Line int
	Col  int
	Msg  string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%d:%d: %s", e.Line, e.Col, e.Msg)
}

const eof rune = -1

// parse reads the entries of a decoded .strings file.
//
// The accepted grammar is the old-style property list dictionary without
// braces:
//
//	file    = { entry }
//	entry   = string [ "=" string ] ";"
//	string  = quoted | unquoted
//
// with /* block */ and // line comments allowed between tokens. An entry
// without a value ("key;") gets its key as value. Line breaks may be LF,
// CRLF or a lone CR.
func parse(text string) ([]Entry, error) {
	p := &parser{src: []rune(text), line: 1, col: 1}
	var entries []Entry
	for {
		if err := p.skipBlank(); err != nil {
			return nil, err
		}
		if p.peek() == eof {
			return entries, nil
		}
		line := p.line
		key, err := p.readString("key")
		if err != nil {
			return nil, err
		}
		if err := p.skipBlank(); err != nil {
			return nil, err
		}
		value := key
		switch p.peek() {
		case ';':
			p.next()
		case '=':
			p.next()
			if err := p.skipBlank(); err != nil {
				return nil, err
			}
			if value, err = p.readString("value"); err != nil {
				return nil, err
			}
			if err := p.skipBlank(); err != nil {
				return nil, err
			}
			if p.peek() != ';' {
				return nil, p.errorf("expected ';' after value, found %s", describe(p.peek()))
			}
			p.next()
		default:
			return nil, p.errorf("expected '=' or ';' after key, found %s", describe(p.peek()))
		}
		entries = append(entries, Entry{Key: key, Value: value, Line: line})
	}
}

type parser struct {
	src  []rune
	pos  int
	line int
	col  int
}

func (p *parser) peek() rune {
	return p.peekAt(0)
}

func (p *parser) peekAt(offset int) rune {
	if p.pos+offset >= len(p.src) {
		return eof
	}
	return p.src[p.pos+offset]
}

// next consumes one rune and keeps the line and column counters up to date.
// CR, LF and CRLF each count as a single line break.
func (p *parser) next() rune {
	r := p.src[p.pos]
	p.pos++
	switch r {
	case '\r':
		p.line++
		p.col = 1
	case '\n':
		if p.pos < 2 || p.src[p.pos-2] != '\r' {
			p.line++
		}
		p.col = 1
	default:
		p.col++
	}
	return r
}

func (p *parser) errorf(format string, args ...any) error {
	return &ParseError{Line: p.line, Col: p.col, Msg: fmt.Sprintf(format, args...)}
}

// skipBlank skips whitespace and comments.
func (p *parser) skipBlank() error {
	for {
		r := p.peek()
		switch {
		case r == eof:
			return nil
		case unicode.IsSpace(r) || r == '\uFEFF':
			p.next()
		case r == '/' && p.peekAt(1) == '/':
			for p.peek() != eof && p.peek() != '\n' && p.peek() != '\r' {
				p.next()
			}
		case r == '/' && p.peekAt(1) == '*':
			start := p.errorf("unterminated comment")
			p.next()
			p.next()
			for {
				if p.peek() == eof {
					return start
				}
				if p.next() == '*' && p.peek() == '/' {
					p.next()
					break
				}
			}
		default:
			return nil
		}
	}
}

// readString reads a quoted or unquoted string; what names its role in
// error messages.
func (p *parser) readString(what string) (string, error) {
	r := p.peek()
	switch {
	case r == '"':
		return p.readQuoted()
	case isUnquoted(r):
		var sb strings.Builder
		for isUnquoted(p.peek()) {
			sb.WriteRune(p.next())
		}
		return sb.String(), nil
	default:
		return "", p.errorf("expected %s, found %s", what, describe(r))
	}
}

// isUnquoted reports whether r may appear in a string written without
// quotes.
func isUnquoted(r rune) bool {
	switch {
	case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', '0' <= r && r <= '9':
		return true
	case r == '_', r == '$', r == '/', r == ':', r == '.', r == '-':
		return true
	}
	return false
}

func (p *parser) readQuoted() (string, error) {
	start := p.errorf("unterminated string")
	p.next() // opening quote
	var sb strings.Builder
	for {
		if p.peek() == eof {
			return "", start
		}
		r := p.next()
		switch r {
		case '"':
			return sb.String(), nil
		case '\\':
			if p.peek() == eof {
				return "", start
			}
			if err := p.readEscape(&sb); err != nil {
				return "", err
			}
		default:
			sb.WriteRune(r)
		}
	}
}

// readEscape decodes the escape sequence following a backslash.
func (p *parser) readEscape(sb *strings.Builder) error {
	r := p.next()
	switch r {
	case 'n':
		sb.WriteRune('\n')
	case 't':
		sb.WriteRune('\t')
	case 'r':
		sb.WriteRune('\r')
	case 'a':
		sb.WriteRune('\a')
	case 'b':
		sb.WriteRune('\b')
	case 'f':
		sb.WriteRune('\f')
	case 'v':
		sb.WriteRune('\v')
	case 'U', 'u':
		u, err := p.readHex4()
		if err != nil {
			return err
		}
		// Characters outside the BMP are written as two \U escapes
		// forming a UTF-16 surrogate pair.
		if utf16.IsSurrogate(u) {
			if p.peek() == '\\' && (p.peekAt(1) == 'U' || p.peekAt(1) == 'u') {
				save := *p
				p.next()
				p.next()
				if lo, err := p.readHex4(); err == nil {
					if c := utf16.DecodeRune(u, lo); c != utf8.RuneError {
						sb.WriteRune(c)
						return nil
					}
				}
				*p = save
			}
			u = utf8.RuneError
		}
		sb.WriteRune(u)
	case '0', '1', '2', '3', '4', '5', '6', '7':
		// Octal escape of up to three digits.
		v := r - '0'
		for i := 0; i < 2 && '0' <= p.peek() && p.peek() <= '7'; i++ {
			v = v*8 + (p.next() - '0')
		}
		sb.WriteRune(v)
	default:
		// \" \\ \' and any unknown escape stand for the character itself.
		sb.WriteRune(r)
	}
	return nil
}

func (p *parser) readHex4() (rune, error) {
	var v rune
	for i := 0; i < 4; i++ {
		d, ok := hexValue(p.peek())
		if !ok {
			return 0, p.errorf("expected 4 hexadecimal digits in \\U escape, found %s", describe(p.peek()))
		}
		p.next()
		v = v<<4 | d
	}
	return v, nil
}

func hexValue(r rune) (rune, bool) {
	switch {
	case '0' <= r && r <= '9':
		return r - '0', true
	case 'a' <= r && r <= 'f':
		return r - 'a' + 10, true
	case 'A' <= r && r <= 'F':
		return r - 'A' + 10, true
	}
	return 0, false
}

func describe(r rune) string {
	if r == eof {
		return "end of file"
	}
	return fmt.Sprintf("%q", r)
}

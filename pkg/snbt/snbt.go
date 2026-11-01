package snbt

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/twlve-secs/bdfmt/pkg/bdf"
)

// PrintOptions configures SNBT formatting
type PrintOptions struct {
	Indent bool
	Depth  int
}

// Print formats a Tag as Stringified NBT (SNBT)
func Print(t bdf.Tag, indent bool) string {
	var sb strings.Builder
	writeTagSNBT(&sb, t, indent, 0)
	return sb.String()
}

func writeIndent(sb *strings.Builder, depth int) {
	for i := 0; i < depth; i++ {
		sb.WriteString("  ")
	}
}

func writeTagSNBT(sb *strings.Builder, t bdf.Tag, indent bool, depth int) {
	switch v := t.(type) {
	case bdf.TagByte:
		sb.WriteString(strconv.FormatInt(int64(v), 10))
		sb.WriteByte('b')
	case bdf.TagShort:
		sb.WriteString(strconv.FormatInt(int64(v), 10))
		sb.WriteByte('s')
	case bdf.TagInt:
		sb.WriteString(strconv.FormatInt(int64(v), 10))
	case bdf.TagLong:
		sb.WriteString(strconv.FormatInt(int64(v), 10))
		sb.WriteByte('L')
	case bdf.TagFloat:
		sb.WriteString(strconv.FormatFloat(float64(v), 'f', -1, 32))
		sb.WriteByte('f')
	case bdf.TagDouble:
		sb.WriteString(strconv.FormatFloat(float64(v), 'f', -1, 64))
		sb.WriteByte('d')
	case bdf.TagString:
		sb.WriteString(strconv.Quote(string(v)))
	case bdf.TagByteArray:
		sb.WriteString("[B;")
		for i, b := range v {
			if i > 0 {
				sb.WriteString(", ")
			} else {
				sb.WriteByte(' ')
			}
			sb.WriteString(strconv.FormatInt(int64(int8(b)), 10))
			sb.WriteByte('b')
		}
		sb.WriteString("]")
	case bdf.TagIntArray:
		sb.WriteString("[I;")
		for i, n := range v {
			if i > 0 {
				sb.WriteString(", ")
			} else {
				sb.WriteByte(' ')
			}
			sb.WriteString(strconv.FormatInt(int64(n), 10))
		}
		sb.WriteString("]")
	case bdf.TagLongArray:
		sb.WriteString("[L;")
		for i, n := range v {
			if i > 0 {
				sb.WriteString(", ")
			} else {
				sb.WriteByte(' ')
			}
			sb.WriteString(strconv.FormatInt(n, 10))
			sb.WriteByte('L')
		}
		sb.WriteString("]")
	case *bdf.TagList:
		if len(v.Elements) == 0 {
			sb.WriteString("[]")
			return
		}
		sb.WriteByte('[')
		if indent {
			sb.WriteByte('\n')
		}
		for i, el := range v.Elements {
			if indent {
				writeIndent(sb, depth+1)
			}
			writeTagSNBT(sb, el, indent, depth+1)
			if i < len(v.Elements)-1 {
				sb.WriteByte(',')
			}
			if indent {
				sb.WriteByte('\n')
			} else if i < len(v.Elements)-1 {
				sb.WriteByte(' ')
			}
		}
		if indent {
			writeIndent(sb, depth)
		}
		sb.WriteByte(']')
	case *bdf.TagCompound:
		entries := v.Entries()
		if len(entries) == 0 {
			sb.WriteString("{}")
			return
		}
		sb.WriteByte('{')
		if indent {
			sb.WriteByte('\n')
		}
		for i, entry := range entries {
			if indent {
				writeIndent(sb, depth+1)
			}
			if isSimpleKey(entry.Key) {
				sb.WriteString(entry.Key)
			} else {
				sb.WriteString(strconv.Quote(entry.Key))
			}
			sb.WriteString(": ")
			writeTagSNBT(sb, entry.Value, indent, depth+1)
			if i < len(entries)-1 {
				sb.WriteByte(',')
			}
			if indent {
				sb.WriteByte('\n')
			} else if i < len(entries)-1 {
				sb.WriteByte(' ')
			}
		}
		if indent {
			writeIndent(sb, depth)
		}
		sb.WriteByte('}')
	}
}

func isSimpleKey(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 && (unicode.IsDigit(r) || r == '-' || r == '+') {
			return false
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' && r != '.' && r != '+' {
			return false
		}
	}
	return true
}

// Parse parses SNBT string into a Tag
func Parse(input string) (bdf.Tag, error) {
	p := newParser(input)
	return p.parseTag()
}

type parser struct {
	input []rune
	pos   int
}

func newParser(s string) *parser {
	return &parser{
		input: []rune(s),
		pos:   0,
	}
}

func (p *parser) skipWhitespace() {
	for p.pos < len(p.input) && unicode.IsSpace(p.input[p.pos]) {
		p.pos++
	}
}

func (p *parser) peek() rune {
	p.skipWhitespace()
	if p.pos >= len(p.input) {
		return 0
	}
	return p.input[p.pos]
}

func (p *parser) parseTag() (bdf.Tag, error) {
	p.skipWhitespace()
	ch := p.peek()
	if ch == 0 {
		return nil, fmt.Errorf("unexpected EOF")
	}

	switch ch {
	case '{':
		return p.parseCompound()
	case '[':
		return p.parseListOrArray()
	case '"', '\'':
		return p.parseQuotedString()
	default:
		return p.parsePrimitiveOrKey()
	}
}

func (p *parser) parseCompound() (*bdf.TagCompound, error) {
	p.pos++ // skip '{'
	comp := bdf.NewTagCompound()

	for {
		p.skipWhitespace()
		if p.pos >= len(p.input) {
			return nil, fmt.Errorf("unclosed compound tag")
		}
		if p.input[p.pos] == '}' {
			p.pos++
			return comp, nil
		}

		key, err := p.parseKey()
		if err != nil {
			return nil, err
		}

		p.skipWhitespace()
		if p.pos >= len(p.input) || p.input[p.pos] != ':' {
			return nil, fmt.Errorf("expected ':' after key %q at pos %d", key, p.pos)
		}
		p.pos++ // skip ':'

		val, err := p.parseTag()
		if err != nil {
			return nil, err
		}
		comp.Set(key, val)

		p.skipWhitespace()
		if p.pos < len(p.input) && p.input[p.pos] == ',' {
			p.pos++
		}
	}
}

func (p *parser) parseKey() (string, error) {
	p.skipWhitespace()
	if p.pos >= len(p.input) {
		return "", fmt.Errorf("expected key, got EOF")
	}
	ch := p.input[p.pos]
	if ch == '"' || ch == '\'' {
		s, err := p.parseQuotedString()
		if err != nil {
			return "", err
		}
		return string(s.(bdf.TagString)), nil
	}

	start := p.pos
	for p.pos < len(p.input) {
		c := p.input[p.pos]
		if unicode.IsSpace(c) || c == ':' || c == '}' || c == ',' {
			break
		}
		p.pos++
	}
	if start == p.pos {
		return "", fmt.Errorf("empty key at pos %d", p.pos)
	}
	return string(p.input[start:p.pos]), nil
}

func (p *parser) parseQuotedString() (bdf.Tag, error) {
	quote := p.input[p.pos]
	p.pos++ // skip opening quote
	var buf bytes.Buffer

	escaped := false
	for p.pos < len(p.input) {
		c := p.input[p.pos]
		p.pos++
		if escaped {
			buf.WriteRune(c)
			escaped = false
		} else if c == '\\' {
			escaped = true
		} else if c == quote {
			return bdf.TagString(buf.String()), nil
		} else {
			buf.WriteRune(c)
		}
	}
	return nil, fmt.Errorf("unclosed quoted string")
}

func (p *parser) parseListOrArray() (bdf.Tag, error) {
	p.pos++ // skip '['
	p.skipWhitespace()

	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("unclosed list tag")
	}

	if p.input[p.pos] == ']' {
		p.pos++
		return &bdf.TagList{ElementType: bdf.TagTypeEnd, Elements: nil}, nil
	}

	// Check for typed array prefix: B;, I;, L;
	if p.pos+1 < len(p.input) && p.input[p.pos+1] == ';' {
		arrayType := p.input[p.pos]
		p.pos += 2 // skip type and ';'
		switch arrayType {
		case 'B', 'b':
			return p.parseByteArray()
		case 'I', 'i':
			return p.parseIntArray()
		case 'L', 'l':
			return p.parseLongArray()
		default:
			return nil, fmt.Errorf("unknown array type '%c;'", arrayType)
		}
	}

	// Standard TagList
	var elements []bdf.Tag
	for {
		p.skipWhitespace()
		if p.pos >= len(p.input) {
			return nil, fmt.Errorf("unclosed list")
		}
		if p.input[p.pos] == ']' {
			p.pos++
			break
		}

		tag, err := p.parseTag()
		if err != nil {
			return nil, err
		}
		elements = append(elements, tag)

		p.skipWhitespace()
		if p.pos < len(p.input) && p.input[p.pos] == ',' {
			p.pos++
		}
	}

	var elemType bdf.TagType = bdf.TagTypeEnd
	if len(elements) > 0 {
		elemType = elements[0].Type()
	}
	return &bdf.TagList{
		ElementType: elemType,
		Elements:    elements,
	}, nil
}

func (p *parser) parseByteArray() (bdf.Tag, error) {
	var bytes []byte
	for {
		p.skipWhitespace()
		if p.pos >= len(p.input) {
			return nil, fmt.Errorf("unclosed byte array")
		}
		if p.input[p.pos] == ']' {
			p.pos++
			return bdf.TagByteArray(bytes), nil
		}
		tag, err := p.parseTag()
		if err != nil {
			return nil, err
		}
		switch v := tag.(type) {
		case bdf.TagByte:
			bytes = append(bytes, byte(v))
		case bdf.TagInt:
			bytes = append(bytes, byte(v))
		default:
			return nil, fmt.Errorf("invalid byte array element: %T", tag)
		}

		p.skipWhitespace()
		if p.pos < len(p.input) && p.input[p.pos] == ',' {
			p.pos++
		}
	}
}

func (p *parser) parseIntArray() (bdf.Tag, error) {
	var ints []int32
	for {
		p.skipWhitespace()
		if p.pos >= len(p.input) {
			return nil, fmt.Errorf("unclosed int array")
		}
		if p.input[p.pos] == ']' {
			p.pos++
			return bdf.TagIntArray(ints), nil
		}
		tag, err := p.parseTag()
		if err != nil {
			return nil, err
		}
		switch v := tag.(type) {
		case bdf.TagInt:
			ints = append(ints, int32(v))
		default:
			return nil, fmt.Errorf("invalid int array element: %T", tag)
		}

		p.skipWhitespace()
		if p.pos < len(p.input) && p.input[p.pos] == ',' {
			p.pos++
		}
	}
}

func (p *parser) parseLongArray() (bdf.Tag, error) {
	var longs []int64
	for {
		p.skipWhitespace()
		if p.pos >= len(p.input) {
			return nil, fmt.Errorf("unclosed long array")
		}
		if p.input[p.pos] == ']' {
			p.pos++
			return bdf.TagLongArray(longs), nil
		}
		tag, err := p.parseTag()
		if err != nil {
			return nil, err
		}
		switch v := tag.(type) {
		case bdf.TagLong:
			longs = append(longs, int64(v))
		case bdf.TagInt:
			longs = append(longs, int64(v))
		default:
			return nil, fmt.Errorf("invalid long array element: %T", tag)
		}

		p.skipWhitespace()
		if p.pos < len(p.input) && p.input[p.pos] == ',' {
			p.pos++
		}
	}
}

func (p *parser) parsePrimitiveOrKey() (bdf.Tag, error) {
	start := p.pos
	for p.pos < len(p.input) {
		c := p.input[p.pos]
		if unicode.IsSpace(c) || c == ',' || c == '}' || c == ']' || c == ':' {
			break
		}
		p.pos++
	}

	raw := string(p.input[start:p.pos])
	if raw == "true" {
		return bdf.TagByte(1), nil
	}
	if raw == "false" {
		return bdf.TagByte(0), nil
	}

	// Try numeric formats with suffixes
	lastChar := raw[len(raw)-1]
	body := raw[:len(raw)-1]

	switch lastChar {
	case 'b', 'B':
		if v, err := strconv.ParseInt(body, 10, 8); err == nil {
			return bdf.TagByte(int8(v)), nil
		}
	case 's', 'S':
		if v, err := strconv.ParseInt(body, 10, 16); err == nil {
			return bdf.TagShort(int16(v)), nil
		}
	case 'l', 'L':
		if v, err := strconv.ParseInt(body, 10, 64); err == nil {
			return bdf.TagLong(v), nil
		}
	case 'f', 'F':
		if v, err := strconv.ParseFloat(body, 32); err == nil {
			return bdf.TagFloat(float32(v)), nil
		}
	case 'd', 'D':
		if v, err := strconv.ParseFloat(body, 64); err == nil {
			return bdf.TagDouble(v), nil
		}
	}

	// Try integer (int32)
	if v, err := strconv.ParseInt(raw, 10, 32); err == nil {
		return bdf.TagInt(int32(v)), nil
	}

	// Try float (double)
	if strings.Contains(raw, ".") {
		if v, err := strconv.ParseFloat(raw, 64); err == nil {
			return bdf.TagDouble(v), nil
		}
	}

	// Fallback to unquoted string
	return bdf.TagString(raw), nil
}

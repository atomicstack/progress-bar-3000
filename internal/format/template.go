package format

import (
	"fmt"
	"strconv"
	"strings"
)

type Resolver interface {
	Resolve(name string, width int) string
}

type segmentKind int

const (
	segmentLiteral segmentKind = iota
	segmentToken
)

type segment struct {
	kind  segmentKind
	lit   string
	name  string
	width int
}

type Template struct {
	segments []segment
}

func Parse(in string) (Template, error) {
	var segments []segment
	var literal strings.Builder

	flushLiteral := func() {
		if literal.Len() == 0 {
			return
		}
		segments = append(segments, segment{kind: segmentLiteral, lit: literal.String()})
		literal.Reset()
	}

	for i := 0; i < len(in); i++ {
		ch := in[i]
		if ch != '%' {
			literal.WriteByte(ch)
			continue
		}

		if i+1 >= len(in) {
			return Template{}, fmt.Errorf("template: unterminated token at position %d", i)
		}

		next := in[i+1]
		if next == '%' {
			literal.WriteByte('%')
			i++
			continue
		}

		flushLiteral()

		if next >= '0' && next <= '9' {
			width, end, err := parseWidth(in, i+1)
			if err != nil {
				return Template{}, err
			}
			if end >= len(in) || in[end] != '{' {
				return Template{}, fmt.Errorf("template: expected '{' after width at position %d", i)
			}
			name, nextIndex, err := parseBracedName(in, end)
			if err != nil {
				return Template{}, err
			}
			segments = append(segments, segment{kind: segmentToken, name: name, width: width})
			i = nextIndex - 1
			continue
		}

		if next == '{' {
			name, nextIndex, err := parseBracedName(in, i+1)
			if err != nil {
				return Template{}, err
			}
			segments = append(segments, segment{kind: segmentToken, name: name})
			i = nextIndex - 1
			continue
		}

		name, ok := aliasForRune(next)
		if !ok {
			return Template{}, fmt.Errorf("template: unknown token %q at position %d", string(next), i)
		}
		segments = append(segments, segment{kind: segmentToken, name: name})
		i++
	}

	flushLiteral()

	return Template{segments: segments}, nil
}

func (t Template) Render(r Resolver) string {
	var out strings.Builder
	for _, seg := range t.segments {
		switch seg.kind {
		case segmentLiteral:
			out.WriteString(seg.lit)
		case segmentToken:
			out.WriteString(r.Resolve(seg.name, seg.width))
		}
	}
	return out.String()
}

func parseWidth(in string, start int) (int, int, error) {
	end := start
	for end < len(in) && in[end] >= '0' && in[end] <= '9' {
		end++
	}
	width, err := strconv.Atoi(in[start:end])
	if err != nil {
		return 0, 0, fmt.Errorf("template: parse width at position %d: %w", start-1, err)
	}
	return width, end, nil
}

func parseBracedName(in string, open int) (string, int, error) {
	if open >= len(in) || in[open] != '{' {
		return "", 0, fmt.Errorf("template: expected '{' at position %d", open)
	}

	close := strings.IndexByte(in[open+1:], '}')
	if close < 0 {
		return "", 0, fmt.Errorf("template: unterminated token at position %d", open)
	}

	name := in[open+1 : open+1+close]
	if name == "" {
		return "", 0, fmt.Errorf("template: empty token at position %d", open)
	}

	return name, open + 1 + close + 1, nil
}

func aliasForRune(ch byte) (string, bool) {
	switch ch {
	case 'p':
		return "progress", true
	case 't':
		return "timer", true
	case 'e':
		return "eta", true
	default:
		return "", false
	}
}

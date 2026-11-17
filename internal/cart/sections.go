package cart

import (
	"strconv"
	"strings"

	"github.com/asimmons91/pico-lsp/internal/source"
)

type lineRef struct {
	start int
	end   int // includes the line terminator when present
}

func splitLines(text string) []lineRef {
	lines := make([]lineRef, 0, 1+strings.Count(text, "\n"))
	start := 0
	for i := 0; i < len(text); {
		switch text[i] {
		case '\r':
			i++
			if i < len(text) && text[i] == '\n' {
				i++
			}
			lines = append(lines, lineRef{start: start, end: i})
			start = i
		case '\n':
			i++
			lines = append(lines, lineRef{start: start, end: i})
			start = i
		default:
			i++
		}
	}
	lines = append(lines, lineRef{start: start, end: len(text)})
	return lines
}

func lineText(text string, line lineRef) string {
	return text[line.start:line.end]
}

func lineContent(text string, line lineRef) string {
	return strings.TrimRight(text[line.start:line.end], "\r\n")
}

// parseSections scans a text .p8 cart. It returns section metadata and the
// lines belonging to __lua__. If there is no __lua__ section, code is empty.
func parseSections(text string) (sections []Section, code []lineRef, version int) {
	lines := splitLines(text)
	if len(lines) == 0 {
		return nil, nil, 0
	}

	firstLine := strings.TrimPrefix(lineContent(text, lines[0]), "\ufeff")
	if strings.HasPrefix(strings.TrimSpace(firstLine), "pico-8 cartridge") && len(lines) > 1 {
		versionLine := strings.TrimSpace(lineContent(text, lines[1]))
		if rest, ok := strings.CutPrefix(versionLine, "version"); ok {
			version, _ = strconv.Atoi(strings.TrimSpace(rest))
		}
	}

	var current *Section
	for i, line := range lines {
		content := lineContent(text, line)
		headerContent := content
		if i == 0 {
			headerContent = strings.TrimPrefix(content, "\ufeff")
		}
		trimmed := strings.TrimSpace(headerContent)
		if strings.HasPrefix(headerContent, "__") && strings.HasSuffix(trimmed, "__") && len(trimmed) > 4 {
			if current != nil {
				current.EndLine = i - 1
				current.SpanVal = current.SpanVal.Merge(source.Span{
					FileID: current.SpanVal.FileID,
					Start:  current.ContentStart,
					End:    line.start,
				})
			}
			name := trimmed[2 : len(trimmed)-2]
			headerStart := line.start
			if i == 0 {
				headerStart += len(content) - len(headerContent)
			}
			sec := Section{
				Name:       name,
				Known:      knownSection(name),
				StartLine:  i,
				EndLine:    i,
				HeaderSpan: source.Span{Start: headerStart, End: contentEnd(text, line)},
			}
			if i+1 < len(lines) {
				sec.ContentStart = lines[i+1].start
			} else {
				sec.ContentStart = line.end
			}
			sec.ContentEnd = sec.ContentStart
			sec.SpanVal = source.Span{Start: headerStart, End: line.end}
			sections = append(sections, sec)
			current = &sections[len(sections)-1]
			continue
		}

		if current != nil {
			current.EndLine = i
			current.ContentEnd = line.end
			current.SpanVal = current.SpanVal.Merge(source.Span{Start: line.start, End: line.end})
			if current.Name == "lua" {
				code = append(code, line)
			}
		}
	}

	return sections, code, version
}

func contentEnd(text string, line lineRef) int {
	return line.start + len(lineContent(text, line))
}

func knownSection(name string) bool {
	switch name {
	case "lua", "gfx", "gff", "label", "map", "sfx", "music":
		return true
	default:
		return false
	}
}

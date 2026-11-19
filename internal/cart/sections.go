package cart

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/asimmons91/pico-lsp/internal/source"
)

type lineRef struct {
	start int
	end   int // includes the line terminator when present
}

// splitLines splits text on LF and CRLF only. A lone CR is P8SCII glyph 13
// data, not a line terminator.
func splitLines(text string) []lineRef {
	lines := make([]lineRef, 0, 1+strings.Count(text, "\n"))
	start := 0
	for i := 0; i < len(text); {
		switch {
		case text[i] == '\n':
			i++
			lines = append(lines, lineRef{start: start, end: i})
			start = i
		case text[i] == '\r' && i+1 < len(text) && text[i+1] == '\n':
			i += 2
			lines = append(lines, lineRef{start: start, end: i})
			start = i
		default:
			i++
		}
	}
	lines = append(lines, lineRef{start: start, end: len(text)})
	return lines
}

// lineContent returns the line without its LF or CRLF terminator. A trailing
// lone CR is data and is preserved.
func lineContent(text string, line lineRef) string {
	end := line.end
	if end > line.start && text[end-1] == '\n' {
		end--
		if end > line.start && text[end-1] == '\r' {
			end--
		}
	}
	return text[line.start:end]
}

type sectionScan struct {
	sections         []Section
	code             []lineRef
	version          int
	malformedLines   []int
	duplicateLuaLine int
}

// parseSections scans a text .p8 cart. It returns section metadata and the
// lines belonging to __lua__. If there is no __lua__ section, code is empty.
func parseSections(text string) (sections []Section, code []lineRef, version int) {
	scan := scanSections(text)
	return scan.sections, scan.code, scan.version
}

func scanSections(text string) sectionScan {
	scan := sectionScan{duplicateLuaLine: -1}
	lines := splitLines(text)
	if len(lines) == 0 {
		return scan
	}

	firstLine := strings.TrimPrefix(lineContent(text, lines[0]), "\ufeff")
	if strings.HasPrefix(strings.TrimSpace(firstLine), "pico-8 cartridge") && len(lines) > 1 {
		versionLine := strings.TrimSpace(lineContent(text, lines[1]))
		if rest, ok := strings.CutPrefix(versionLine, "version"); ok {
			scan.version, _ = strconv.Atoi(strings.TrimSpace(rest))
		}
	}

	var current *Section
	seenLua := false
	for i, line := range lines {
		virtualFinal := i == len(lines)-1 && line.start == line.end && line.start == len(text)
		content := lineContent(text, line)
		headerContent := content
		if i == 0 {
			headerContent = strings.TrimPrefix(content, "\ufeff")
		}
		trimmed := strings.TrimSpace(headerContent)
		if name, ok := sectionName(headerContent, trimmed); ok {
			if name == "" || strings.IndexFunc(name, unicode.IsSpace) >= 0 {
				scan.malformedLines = append(scan.malformedLines, i)
			} else {
				if current != nil {
					current.EndLine = i - 1
					current.SpanVal = current.SpanVal.Merge(source.Span{
						FileID: current.SpanVal.FileID,
						Start:  current.ContentStart,
						End:    line.start,
					})
				}
				headerStart := line.start
				if i == 0 {
					headerStart += len(content) - len(headerContent)
				}
				headerEnd := contentEnd(text, line)
				sec := Section{
					Name:             name,
					Known:            knownSection(name),
					StartLine:        i,
					EndLine:          i,
					ContentStartLine: i + 1,
					HeaderSpan:       source.Span{Start: headerStart, End: headerEnd},
					HeaderTerminated: headerEnd < line.end,
				}
				if i+1 < len(lines) {
					sec.ContentStart = lines[i+1].start
				} else {
					sec.ContentStart = line.end
				}
				sec.ContentEnd = sec.ContentStart
				sec.SpanVal = source.Span{Start: headerStart, End: line.end}
				if name == "lua" {
					if seenLua && scan.duplicateLuaLine < 0 {
						scan.duplicateLuaLine = i
					}
					seenLua = true
				}
				scan.sections = append(scan.sections, sec)
				current = &scan.sections[len(scan.sections)-1]
				continue
			}
		}

		if current != nil {
			if !virtualFinal {
				current.EndLine = i
			}
			current.ContentEnd = line.end
			current.SpanVal = current.SpanVal.Merge(source.Span{Start: line.start, End: line.end})
			if current.Name == "lua" {
				scan.code = append(scan.code, line)
			}
		}
	}

	return scan
}

// sectionName recognizes a section header at column 0. The returned name may be
// empty or contain whitespace; callers decide whether that is malformed.
func sectionName(headerContent, trimmed string) (string, bool) {
	if !strings.HasPrefix(headerContent, "__") || !strings.HasSuffix(trimmed, "__") {
		return "", false
	}
	if len(trimmed) < 2 {
		return "", false
	}
	if len(trimmed) <= 4 {
		return "", true
	}
	return trimmed[2 : len(trimmed)-2], true
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

// countCodeLines counts physical lines in a code region. A trailing LF or CRLF
// terminates the last line rather than adding an empty one.
func countCodeLines(code string) int {
	if code == "" {
		return 0
	}
	count := strings.Count(code, "\n")
	if code[len(code)-1] != '\n' {
		count++
	}
	return count
}

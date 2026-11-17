package cart

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"go.lsp.dev/uri"

	"github.com/asimmons91/pico-lsp/internal/source"
)

type scanState struct {
	kind  byte // 0, 's' (long string), 'c' (long comment), 'q' (continued short string)
	level int
	quote byte
}

func (s *scanState) outside() bool { return s.kind == 0 }

func (s *scanState) scan(line string) {
	for i := 0; i < len(line); {
		if s.kind == 's' || s.kind == 'c' {
			close := "]" + strings.Repeat("=", s.level) + "]"
			idx := strings.Index(line[i:], close)
			if idx < 0 {
				return
			}
			i += idx + len(close)
			s.kind = 0
			s.level = 0
			continue
		}
		if s.kind == 'q' {
			continued := scanShortString(line, &i, s.quote)
			if continued {
				return
			}
			s.kind = 0
			if i >= len(line) {
				return
			}
			continue
		}

		if i+1 < len(line) && line[i] == '-' && line[i+1] == '-' {
			if level, ok := longBracketLevel(line, i+2); ok {
				s.kind = 'c'
				s.level = level
				i += 2 + level + 2
				continue
			}
			return
		}
		if i+1 < len(line) && line[i] == '/' && line[i+1] == '/' {
			return
		}
		if level, ok := longBracketLevel(line, i); ok {
			s.kind = 's'
			s.level = level
			i += level + 2
			continue
		}
		if line[i] == '\'' || line[i] == '"' {
			quote := line[i]
			i++
			if scanShortString(line, &i, quote) {
				s.kind = 'q'
				s.quote = quote
				return
			}
			continue
		}
		i++
	}
}

// scanShortString advances i past a short string. It returns true when the
// line ends with an escaped newline, so the string continues on the next line.
func scanShortString(line string, i *int, quote byte) (continued bool) {
	for *i < len(line) {
		switch line[*i] {
		case '\\':
			if *i+1 == len(line) {
				*i = len(line)
				return true
			}
			*i += 2
		default:
			if line[*i] == quote {
				*i++
				return false
			}
			*i++
		}
	}
	return false
}

func longBracketLevel(text string, pos int) (int, bool) {
	if pos >= len(text) || text[pos] != '[' {
		return 0, false
	}
	i := pos + 1
	for i < len(text) && text[i] == '=' {
		i++
	}
	if i < len(text) && text[i] == '[' {
		return i - pos - 1, true
	}
	return 0, false
}

func includeDirective(content string) (string, bool) {
	trimmed := strings.TrimLeft(content, " \t")
	const directive = "#include"
	if !strings.HasPrefix(trimmed, directive) {
		return "", false
	}
	rest := trimmed[len(directive):]
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return "", false
	}
	path := strings.TrimSpace(rest)
	if len(path) >= 2 {
		if (path[0] == '"' && path[len(path)-1] == '"') || (path[0] == '\'' && path[len(path)-1] == '\'') {
			path = path[1 : len(path)-1]
		}
	}
	return path, true
}

// splitIncludeTarget separates a PICO-8 tab suffix (a single hex digit) from
// a cartridge include path. A tab of -1 means all tabs.
func splitIncludeTarget(path string) (string, int) {
	if len(path) < 2 || path[len(path)-2] != ':' {
		return path, -1
	}
	c := path[len(path)-1]
	var tab int
	switch {
	case c >= '0' && c <= '9':
		tab = int(c - '0')
	case c >= 'a' && c <= 'f':
		tab = int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		tab = int(c-'A') + 10
	default:
		return path, -1
	}
	name := path[:len(path)-2]
	lower := strings.ToLower(name)
	if !strings.HasSuffix(lower, ".p8") && !strings.HasSuffix(lower, ".p8.png") {
		return path, -1
	}
	return name, tab
}

type flattener struct {
	loader   *Loader
	rootDir  string
	files    []*File
	byURI    map[uri.URI]source.FileID
	flat     []byte
	segments []Segment
	includes []Include
	diags    []Diagnostic
	state    scanState

	limitHit      bool
	needSeparator bool
}

func (f *flattener) addFile(u uri.URI, text string) source.FileID {
	if id, ok := f.byURI[u]; ok {
		return id
	}
	id := source.FileID(len(f.files))
	file := &File{
		ID:    id,
		URI:   u,
		Text:  text,
		Index: source.New([]byte(text), f.loader.Encoding),
	}
	f.files = append(f.files, file)
	f.byURI[u] = id
	return id
}

func (f *flattener) limit(span source.Span, message string) {
	if f.limitHit {
		return
	}
	f.limitHit = true
	f.diags = append(f.diags, Diagnostic{
		Code:     CodeIncludeUnreadable,
		Message:  message,
		Span:     span,
		Severity: SeverityError,
	})
}

func (f *flattener) appendData(fileID source.FileID, fileOffset int, data []byte, span source.Span) bool {
	if f.limitHit || len(data) == 0 {
		return !f.limitHit
	}
	if limit := f.loader.maxFlatSize(); limit > 0 && len(f.flat)+len(data) > limit {
		f.limit(span, fmt.Sprintf("flattened cart exceeds the supported size (%d bytes)", limit))
		return false
	}
	f.flat, f.segments = appendFlat(f.flat, f.segments, fileID, fileOffset, data)
	return true
}

// flushSeparator appends the newline that followed a root #include directive.
// PICO-8 replaces the directive text but not the line terminator. The newline
// is synthetic and deliberately has no source-map segment.
func (f *flattener) flushSeparator(span source.Span) bool {
	if !f.needSeparator {
		return !f.limitHit
	}
	f.needSeparator = false
	if limit := f.loader.maxFlatSize(); limit > 0 && len(f.flat)+1 > limit {
		f.limit(span, fmt.Sprintf("flattened cart exceeds the supported size (%d bytes)", limit))
		return false
	}
	f.flat = append(f.flat, '\n')
	return true
}

func (f *flattener) appendLine(text string, line lineRef, fileID source.FileID, trimTerminator bool, span source.Span) bool {
	if !f.flushSeparator(span) {
		return false
	}
	end := line.end
	if trimTerminator {
		end = contentEnd(text, line)
	}
	return f.appendData(fileID, line.start, []byte(text[line.start:end]), span)
}

func (f *flattener) includeCountLimit(span source.Span) bool {
	limit := f.loader.maxIncludes()
	if limit > 0 && len(f.includes) >= limit {
		f.limit(span, fmt.Sprintf("too many #include directives (limit %d)", limit))
		return true
	}
	return false
}

func (f *flattener) appendLines(text string, lines []lineRef, fileID source.FileID, trimFinalTerminator bool) {
	for i, line := range lines {
		if f.limitHit {
			return
		}
		content := lineContent(text, line)
		span := source.Span{FileID: fileID, Start: line.start, End: contentEnd(text, line)}
		if path, ok := includeDirective(content); ok && f.state.outside() {
			if f.includeCountLimit(span) {
				return
			}
			if f.processRootInclude(path, line) {
				// The directive's line terminator remains after the replacement.
				f.needSeparator = true
			}
			continue
		}
		trim := trimFinalTerminator && i == len(lines)-1
		if !f.appendLine(text, line, fileID, trim, span) {
			return
		}
		f.state.scan(content)
	}
}

func (f *flattener) appendIncluded(text string, lines []lineRef, fileID source.FileID, dir string, trimFinalTerminator bool) {
	if len(lines) > 0 && lines[0].start == 0 && strings.HasPrefix(text, "\ufeff") {
		lines[0].start += len("\ufeff")
	}
	for i, line := range lines {
		if f.limitHit {
			return
		}
		content := lineContent(text, line)
		span := source.Span{FileID: fileID, Start: line.start, End: contentEnd(text, line)}
		if path, ok := includeDirective(content); ok && f.state.outside() {
			if f.includeCountLimit(span) {
				return
			}
			resolved, err := resolveInclude(dir, path)
			if err != nil {
				f.diags = append(f.diags, Diagnostic{
					Code:     CodeIncludeUnreadable,
					Message:  fmt.Sprintf("invalid nested #include path %q: %v", path, err),
					Span:     span,
					Severity: SeverityError,
				})
			}
			f.includes = append(f.includes, Include{
				Path:          path,
				ResolvedURI:   resolved,
				DirectiveSpan: span,
				Nested:        true,
			})
			f.diags = append(f.diags, Diagnostic{
				Code:     CodeIncludeNested,
				Message:  fmt.Sprintf("nested #include is not supported: %s", path),
				Span:     span,
				Severity: SeverityWarning,
			})
			// Pasted text keeps the nested directive; PICO-8 does not recurse.
			trim := trimFinalTerminator && i == len(lines)-1
			if !f.appendLine(text, line, fileID, trim, span) {
				return
			}
			f.state.scan(content)
			continue
		}
		trim := trimFinalTerminator && i == len(lines)-1
		if !f.appendLine(text, line, fileID, trim, span) {
			return
		}
		f.state.scan(content)
	}
}

func (f *flattener) processRootInclude(path string, directive lineRef) bool {
	span := source.Span{FileID: 0, Start: directive.start, End: directive.end}
	// The directive line's end includes a newline; point at the content.
	if span.End > span.Start {
		if f.files[0].Text[span.End-1] == '\n' {
			span.End--
		}
		if span.End > span.Start && f.files[0].Text[span.End-1] == '\r' {
			span.End--
		}
	}
	return f.loadRootInclude(path, span, f.rootDir)
}

func (f *flattener) loadRootInclude(path string, span source.Span, dir string) bool {
	name, tab := splitIncludeTarget(path)
	resolved, err := resolveInclude(dir, name)
	if err != nil {
		f.diags = append(f.diags, Diagnostic{
			Code:     CodeIncludeUnreadable,
			Message:  fmt.Sprintf("invalid #include path %q: %v", path, err),
			Span:     span,
			Severity: SeverityError,
		})
		f.includes = append(f.includes, Include{Path: path, DirectiveSpan: span, Unreadable: true})
		return false
	}
	if IsP8PNG(resolved) {
		f.includes = append(f.includes, Include{Path: path, ResolvedURI: resolved, DirectiveSpan: span, Unreadable: true})
		f.diags = append(f.diags, Diagnostic{
			Code:     CodeIncludeUnreadable,
			Message:  fmt.Sprintf("PICO-8 PNG includes are not supported yet: %s", path),
			Span:     span,
			Severity: SeverityError,
		})
		return false
	}

	data, readErr := f.loader.Provider.Read(resolved)
	if readErr != nil {
		record := Include{Path: path, ResolvedURI: resolved, DirectiveSpan: span}
		if errors.Is(readErr, os.ErrNotExist) {
			record.Missing = true
			f.diags = append(f.diags, Diagnostic{
				Code:     CodeIncludeMissing,
				Message:  fmt.Sprintf("included file not found: %s", path),
				Span:     span,
				Severity: SeverityError,
			})
		} else {
			record.Unreadable = true
			f.diags = append(f.diags, Diagnostic{
				Code:     CodeIncludeUnreadable,
				Message:  fmt.Sprintf("cannot read included file %s: %v", path, readErr),
				Span:     span,
				Severity: SeverityError,
			})
		}
		f.includes = append(f.includes, record)
		return false
	}

	if max := f.loader.maxSize(); len(data) > max {
		f.includes = append(f.includes, Include{Path: path, ResolvedURI: resolved, DirectiveSpan: span, Unreadable: true})
		f.diags = append(f.diags, Diagnostic{
			Code:     CodeIncludeUnreadable,
			Message:  fmt.Sprintf("included file is too large: %s (%d bytes)", path, len(data)),
			Span:     span,
			Severity: SeverityError,
		})
		return false
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		f.includes = append(f.includes, Include{Path: path, ResolvedURI: resolved, DirectiveSpan: span, Unreadable: true})
		f.diags = append(f.diags, Diagnostic{
			Code:     CodeIncludeUnreadable,
			Message:  fmt.Sprintf("included file is binary or not UTF-8: %s", path),
			Span:     span,
			Severity: SeverityError,
		})
		return false
	}

	text := string(data)
	fileID := f.addFile(resolved, text)
	record := Include{
		Path:          path,
		ResolvedURI:   resolved,
		DirectiveSpan: span,
	}
	if strings.HasSuffix(strings.ToLower(resolved.Path()), ".p8") {
		sections, code, _ := parseSections(text)
		selected, trimFinal, ok := selectCartCode(text, sections, code, tab)
		if !ok {
			f.includes = append(f.includes, Include{Path: path, ResolvedURI: resolved, DirectiveSpan: span, Unreadable: true})
			f.diags = append(f.diags, Diagnostic{
				Code:     CodeIncludeUnreadable,
				Message:  fmt.Sprintf("tab %d not found in included cart %s", tab, path),
				Span:     span,
				Severity: SeverityError,
			})
			return false
		}
		record.FileSpan = source.Span{FileID: fileID}
		if len(selected) > 0 {
			record.FileSpan.Start = selected[0].start
			record.FileSpan.End = selected[len(selected)-1].end
		}
		f.includes = append(f.includes, record)
		f.appendIncluded(text, selected, fileID, filepath.Dir(resolved.FsPath()), trimFinal)
		return true
	}

	record.FileSpan = source.Span{FileID: fileID, Start: 0, End: len(text)}
	f.includes = append(f.includes, record)
	f.appendIncluded(text, splitLines(text), fileID, filepath.Dir(resolved.FsPath()), false)
	return true
}

// selectCartCode returns the __lua__ lines for a tab (-1 for all tabs) and
// whether the final line terminator must be trimmed because the selection ends
// at the next tab marker or at another section.
func selectCartCode(text string, sections []Section, code []lineRef, tab int) ([]lineRef, bool, bool) {
	sectionFollows := luaFollowedBySection(sections)
	if tab < 0 {
		return code, sectionFollows, true
	}
	var markers []int
	for i, line := range code {
		left := strings.TrimLeft(lineContent(text, line), " \t")
		if strings.HasPrefix(left, "-->8") {
			markers = append(markers, i)
		}
	}
	if tab == 0 {
		if len(markers) == 0 {
			return code, sectionFollows, true
		}
		return code[:markers[0]], true, true
	}
	if tab-1 >= len(markers) {
		return nil, false, false
	}
	start := markers[tab-1] + 1
	end := len(code)
	trimFinal := sectionFollows
	if tab < len(markers) {
		end = markers[tab]
		trimFinal = true
	}
	return code[start:end], trimFinal, true
}

func resolveInclude(dir, path string) (uri.URI, error) {
	if strings.ContainsRune(path, 0) {
		return "", errors.New("path contains NUL")
	}
	slashed := strings.ReplaceAll(path, "\\", "/")
	if slashed == "" {
		return "", errors.New("empty path")
	}
	full := filepath.Join(dir, filepath.FromSlash(slashed))
	return uri.File(filepath.Clean(full)), nil
}

func (l *Loader) maxSize() int {
	if l.MaxFileSize > 0 {
		return l.MaxFileSize
	}
	return MaxFileSize
}

func (l *Loader) maxFlatSize() int {
	if l.MaxFlatSize > 0 {
		return l.MaxFlatSize
	}
	return MaxFlatSize
}

func (l *Loader) maxIncludes() int {
	if l.MaxIncludes > 0 {
		return l.MaxIncludes
	}
	return MaxIncludes
}

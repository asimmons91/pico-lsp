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
	kind  byte // 0, 's' (long string), 'c' (long comment)
	level int
}

func (s *scanState) outside() bool { return s.kind == 0 }

func (s *scanState) scan(line string) {
	for i := 0; i < len(line); {
		if s.kind != 0 {
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
			for i < len(line) {
				if line[i] == '\\' {
					i += 2
					continue
				}
				if line[i] == quote {
					i++
					break
				}
				i++
			}
			continue
		}
		i++
	}
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

func (f *flattener) appendLines(text string, lines []lineRef, fileID source.FileID) {
	for _, line := range lines {
		content := lineContent(text, line)
		if path, ok := includeDirective(content); ok && f.state.outside() {
			f.processRootInclude(path, line)
			continue
		}
		f.flat, f.segments = appendFlat(f.flat, f.segments, fileID, line.start, []byte(text[line.start:line.end]))
		f.state.scan(content)
	}
}

func (f *flattener) appendIncluded(text string, lines []lineRef, fileID source.FileID, dir string) {
	for _, line := range lines {
		content := lineContent(text, line)
		if path, ok := includeDirective(content); ok && f.state.outside() {
			span := source.Span{FileID: fileID, Start: line.start, End: contentEnd(text, line)}
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
			f.flat, f.segments = appendFlat(f.flat, f.segments, fileID, line.start, []byte(text[line.start:line.end]))
			f.state.scan(content)
			continue
		}
		f.flat, f.segments = appendFlat(f.flat, f.segments, fileID, line.start, []byte(text[line.start:line.end]))
		f.state.scan(content)
	}
	if len(text) > 0 && !strings.HasSuffix(text, "\n") && !strings.HasSuffix(text, "\r") {
		f.flat, f.segments = appendFlat(f.flat, f.segments, fileID, len(text), []byte("\n"))
	}
}

func (f *flattener) processRootInclude(path string, directive lineRef) {
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
	f.loadRootInclude(path, span, f.rootDir)
}

func (f *flattener) loadRootInclude(path string, span source.Span, dir string) {
	resolved, err := resolveInclude(dir, path)
	if err != nil {
		f.diags = append(f.diags, Diagnostic{
			Code:     CodeIncludeUnreadable,
			Message:  fmt.Sprintf("invalid #include path %q: %v", path, err),
			Span:     span,
			Severity: SeverityError,
		})
		f.includes = append(f.includes, Include{Path: path, DirectiveSpan: span, Unreadable: true})
		return
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
		return
	}

	if max := f.loader.maxSize(); len(data) > max {
		f.includes = append(f.includes, Include{Path: path, ResolvedURI: resolved, DirectiveSpan: span, Unreadable: true})
		f.diags = append(f.diags, Diagnostic{
			Code:     CodeIncludeUnreadable,
			Message:  fmt.Sprintf("included file is too large: %s (%d bytes)", path, len(data)),
			Span:     span,
			Severity: SeverityError,
		})
		return
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		f.includes = append(f.includes, Include{Path: path, ResolvedURI: resolved, DirectiveSpan: span, Unreadable: true})
		f.diags = append(f.diags, Diagnostic{
			Code:     CodeIncludeUnreadable,
			Message:  fmt.Sprintf("included file is binary or not UTF-8: %s", path),
			Span:     span,
			Severity: SeverityError,
		})
		return
	}

	text := string(data)
	fileID := f.addFile(resolved, text)
	f.includes = append(f.includes, Include{
		Path:          path,
		ResolvedURI:   resolved,
		DirectiveSpan: span,
		FileSpan:      source.Span{FileID: fileID, Start: 0, End: len(text)},
	})
	f.appendIncluded(text, splitLines(text), fileID, filepath.Dir(resolved.FsPath()))
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

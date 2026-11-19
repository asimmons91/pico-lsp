// Package format implements a conservative, AST-guided formatter for PICO-8
// Lua code.
package format

import (
	"errors"
	"sort"
	"strings"

	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// Options controls document formatting.
type Options struct {
	TabSize                int
	InsertSpaces           bool
	TrimTrailingWhitespace bool
	InsertFinalNewline     bool
	TrimFinalNewlines      bool
	MaxBlankLines          int
}

// DefaultOptions returns the defaults documented for pico-lsp.
func DefaultOptions() Options {
	return Options{
		TabSize:                2,
		InsertSpaces:           true,
		TrimTrailingWhitespace: true,
		InsertFinalNewline:     true,
		TrimFinalNewlines:      true,
		MaxBlankLines:          2,
	}
}

// ErrSyntax reports that a document could not be formatted because it has
// syntax errors.
var ErrSyntax = errors.New("format: document has syntax errors")

// ErrUnsafe reports that formatting would change the token or comment stream.
var ErrUnsafe = errors.New("format: refusing to change the token stream")

// Lua formats a standalone or included Lua file.
func Lua(src string, opts Options) (string, error) {
	return formatLua(src, opts, false)
}

// P8 formats the __lua__ section of a text cart and preserves every other
// section byte-for-byte.
func P8(src string, opts Options) (string, error) {
	lines := splitLines(src)
	lua := -1
	for i, line := range lines {
		if name, ok := sectionName(line.text); ok && strings.EqualFold(name, "lua") {
			lua = i
			break
		}
	}
	if lua < 0 {
		return src, nil
	}
	end := len(lines)
	for i := lua + 1; i < len(lines); i++ {
		if _, ok := sectionName(lines[i].text); ok {
			end = i
			break
		}
	}
	if end <= lua+1 {
		return src, nil
	}
	codeStart := lines[lua+1].start
	codeEnd := lines[end-1].start + len(lines[end-1].text) + len(lines[end-1].eol)
	codeOpts := opts
	codeOpts.InsertFinalNewline = false
	codeOpts.TrimFinalNewlines = false
	formatted, err := formatLua(src[codeStart:codeEnd], codeOpts, true)
	if err != nil {
		return "", err
	}
	return src[:codeStart] + formatted + src[codeEnd:], nil
}

// Document formats src according to whether it is a .p8 cart.
func Document(src string, opts Options, cart bool) (string, error) {
	if cart {
		return P8(src, opts)
	}
	return Lua(src, opts)
}

func formatLua(src string, opts Options, allowIncludes bool) (string, error) {
	opts = normalizeOptions(opts)
	lines := splitLines(src)
	if len(lines) == 0 {
		return src, nil
	}
	masked := src
	if allowIncludes {
		masked = maskIncludes(src, lines)
	}
	lexed := syntax.Lex([]byte(masked), nil)
	if len(lexed.Errors) > 0 {
		return "", ErrSyntax
	}
	chunk, parseErrors := syntax.Parse(lexed.Tokens)
	if len(parseErrors) > 0 {
		return "", ErrSyntax
	}
	if chunk == nil || chunk.Block == nil {
		return src, nil
	}

	lineStarts := make([]int, len(lines))
	for i, line := range lines {
		lineStarts[i] = line.start
	}
	lineAt := func(offset int) int {
		if offset < 0 {
			return 0
		}
		if offset > len(src) {
			offset = len(src)
		}
		i := sort.Search(len(lineStarts), func(i int) bool { return lineStarts[i] > offset }) - 1
		if i < 0 {
			return 0
		}
		if i >= len(lines) {
			return len(lines) - 1
		}
		return i
	}

	depth := make([]int, len(lines))
	assigned := make([]bool, len(lines))
	raise := func(start, end, value int) {
		if start < 0 {
			start = 0
		}
		if end > len(lines) {
			end = len(lines)
		}
		for i := start; i < end; i++ {
			if depth[i] < value {
				depth[i] = value
			}
			assigned[i] = true
		}
	}
	var markExpr func(syntax.Expr, int)
	var markBlock func(*syntax.Block, int)
	var markFunction func(*syntax.FunctionExpr, int)
	var markStmt func(syntax.Stmt, int)
	markFunction = func(fn *syntax.FunctionExpr, value int) {
		if fn == nil {
			return
		}
		endLine := lineAt(fn.End.Start)
		startLine := lineAt(fn.Span().Start) + 1
		if startLine < endLine {
			raise(startLine, endLine, value+1)
			markBlock(fn.Body, value+1)
		} else {
			// Inline function bodies stay on their declaration line.
			markBlock(fn.Body, value)
		}
	}
	markExpr = func(expr syntax.Expr, value int) {
		switch e := expr.(type) {
		case nil:
		case *syntax.UnaryExpr:
			markExpr(e.X, value)
		case *syntax.BinaryExpr:
			markExpr(e.Left, value)
			markExpr(e.Right, value)
		case *syntax.FieldExpr:
			markExpr(e.Base, value)
		case *syntax.IndexExpr:
			markExpr(e.Base, value)
			markExpr(e.Index, value)
		case *syntax.CallExpr:
			markExpr(e.Func, value)
			for _, arg := range e.Args {
				markExpr(arg, value)
			}
		case *syntax.MethodCallExpr:
			markExpr(e.Receiver, value)
			for _, arg := range e.Args {
				markExpr(arg, value)
			}
		case *syntax.FunctionExpr:
			markFunction(e, value)
		case *syntax.ParenExpr:
			markExpr(e.X, value)
		case *syntax.TableExpr:
			for _, field := range e.Fields {
				markExpr(field.Key, value+1)
				markExpr(field.Value, value+1)
			}
		}
	}
	markBlock = func(block *syntax.Block, value int) {
		if block == nil {
			return
		}
		for _, stmt := range block.Stmts {
			markStmt(stmt, value)
		}
	}
	markStmt = func(stmt syntax.Stmt, value int) {
		if stmt == nil {
			return
		}
		span := stmt.Span()
		startLine := lineAt(span.Start)
		endLine := lineAt(span.End - 1)
		if endLine < startLine {
			endLine = startLine
		}
		raise(startLine, endLine+1, value)
		switch s := stmt.(type) {
		case *syntax.LocalStmt:
			for _, v := range s.Values {
				markExpr(v, value)
			}
		case *syntax.LocalFunctionStmt:
			markFunction(s.Func, value)
		case *syntax.FunctionDeclStmt:
			markFunction(s.Func, value)
		case *syntax.AssignStmt:
			for _, target := range s.Targets {
				markExpr(target, value)
			}
			for _, v := range s.Values {
				markExpr(v, value)
			}
		case *syntax.CallStmt:
			markExpr(s.Call, value)
		case *syntax.DoStmt:
			if s.Body != nil {
				raise(lineAt(s.Do.Start)+1, lineAt(s.End.Start), value+1)
			}
			markBlock(s.Body, value+1)
		case *syntax.WhileStmt:
			markExpr(s.Cond, value)
			if s.SingleLine {
				markBlock(s.Body, value)
				break
			}
			if s.Body != nil {
				raise(lineAt(s.Do.Start)+1, lineAt(s.End.Start), value+1)
			}
			markBlock(s.Body, value+1)
		case *syntax.RepeatStmt:
			if s.Body != nil {
				raise(lineAt(s.Repeat.Start)+1, lineAt(s.Until.Start), value+1)
			}
			markBlock(s.Body, value+1)
			markExpr(s.Cond, value)
		case *syntax.IfStmt:
			for i, branch := range s.Branches {
				markExpr(branch.Cond, value)
				if s.SingleLine {
					continue
				}
				nextLine := endLine
				if i+1 < len(s.Branches) {
					nextLine = lineAt(s.Branches[i+1].Keyword.Start)
				} else if s.Else != nil {
					nextLine = lineAt(s.ElseSpan.Start)
				}
				if branch.Body != nil {
					raise(lineAt(branch.Keyword.Start)+1, nextLine, value+1)
				}
				markBlock(branch.Body, value+1)
			}
			if s.Else != nil {
				raise(lineAt(s.ElseSpan.Start)+1, lineAt(s.End.Start), value+1)
				markBlock(s.Else, value+1)
			}
		case *syntax.NumericForStmt:
			markExpr(s.Init, value)
			markExpr(s.Limit, value)
			markExpr(s.Step, value)
			if s.Body != nil {
				raise(lineAt(s.Do.Start)+1, lineAt(s.End.Start), value+1)
			}
			markBlock(s.Body, value+1)
		case *syntax.GenericForStmt:
			for _, v := range s.Values {
				markExpr(v, value)
			}
			if s.Body != nil {
				raise(lineAt(s.Do.Start)+1, lineAt(s.End.Start), value+1)
			}
			markBlock(s.Body, value+1)
		case *syntax.ReturnStmt:
			for _, v := range s.Values {
				markExpr(v, value)
			}
		case *syntax.PrintStmt:
			for _, v := range s.Args {
				markExpr(v, value)
			}
		}
	}
	markBlock(chunk.Block, 0)

	protected := make([]bool, len(lines))
	lineTokens := make([][]syntax.Token, len(lines))
	for _, token := range lexed.Tokens {
		if token.Kind == syntax.TokenEOF || token.Kind == syntax.TokenComment || token.Kind == syntax.TokenInclude {
			continue
		}
		line := token.Line
		if line < 0 || line >= len(lineTokens) {
			continue
		}
		lineTokens[line] = append(lineTokens[line], token)
		if token.Kind == syntax.TokenString {
			end := lineAt(token.Span.End)
			if end > line {
				for i := line; i <= end && i < len(protected); i++ {
					protected[i] = true
				}
			}
		}
	}
	comments := make(map[int][]syntax.Comment)
	for _, comment := range lexed.Comments {
		line := comment.Line
		if line < 0 || line >= len(lines) {
			continue
		}
		comments[line] = append(comments[line], comment)
		if comment.Kind == syntax.CommentBlock {
			end := lineAt(comment.Span.End)
			if end > line {
				for i := line; i <= end && i < len(protected); i++ {
					protected[i] = true
				}
			}
		}
		if comment.Kind == syntax.CommentTab {
			protected[line] = true
		}
	}
	shorthand := make([]bool, len(lines))
	var markShorthand func(syntax.Stmt)
	var markShorthandBlock func(*syntax.Block)
	markShorthand = func(stmt syntax.Stmt) {
		switch s := stmt.(type) {
		case *syntax.IfStmt:
			if s.SingleLine {
				shorthand[lineAt(s.Span().Start)] = true
			}
			for _, b := range s.Branches {
				markShorthandBlock(b.Body)
			}
			markShorthandBlock(s.Else)
		case *syntax.WhileStmt:
			if s.SingleLine {
				shorthand[lineAt(s.Span().Start)] = true
			}
			markShorthandBlock(s.Body)
		case *syntax.PrintStmt:
			shorthand[lineAt(s.Span().Start)] = true
		case *syntax.DoStmt:
			markShorthandBlock(s.Body)
		case *syntax.RepeatStmt:
			markShorthandBlock(s.Body)
		case *syntax.NumericForStmt:
			markShorthandBlock(s.Body)
		case *syntax.GenericForStmt:
			markShorthandBlock(s.Body)
		}
	}
	markShorthandBlock = func(block *syntax.Block) {
		if block == nil {
			return
		}
		for _, stmt := range block.Stmts {
			markShorthand(stmt)
		}
	}
	for _, stmt := range chunk.Block.Stmts {
		markShorthand(stmt)
	}

	for i := 0; i < len(lines); i++ {
		if protected[i] {
			continue
		}
		first := firstToken(lineTokens[i])
		if first == nil {
			continue
		}
		if first.Is("end") || first.Is("else") || first.Is("elseif") || first.Is("until") {
			// Closing keywords belong to the surrounding block.
			depth[i] = maxInt(0, depth[i])
		}
	}
	// Fill comment-only/blank gaps with the nearest structural indentation.
	prev := -1
	for i := 0; i < len(lines); i++ {
		if assigned[i] {
			prev = i
			continue
		}
		next := -1
		for j := i + 1; j < len(lines); j++ {
			if assigned[j] {
				next = j
				break
			}
		}
		if next >= 0 {
			t := strings.TrimSpace(lines[next].text)
			if strings.HasPrefix(t, "end") || strings.HasPrefix(t, "else") || strings.HasPrefix(t, "elseif") || strings.HasPrefix(t, "until") {
				if prev >= 0 {
					depth[i] = depth[prev]
				}
			} else {
				depth[i] = depth[next]
			}
		} else if prev >= 0 {
			depth[i] = depth[prev]
		}
	}

	var out strings.Builder
	openDelims := 0
	blankRun := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line.text)
		first := firstToken(lineTokens[i])
		closeAdjust := 0
		if first != nil && (first.Is(")") || first.Is("]") || first.Is("}")) && openDelims > 0 {
			closeAdjust = 1
		}
		indentDepth := depth[i] + openDelims - closeAdjust
		if indentDepth < 0 {
			indentDepth = 0
		}

		var content string
		switch {
		case trimmed == "":
			blankRun++
			if opts.MaxBlankLines >= 0 && blankRun > opts.MaxBlankLines {
				continue
			}
			content = ""
		case isIncludeLine(trimmed) || strings.HasPrefix(trimmed, "-->8"):
			blankRun = 0
			content = strings.TrimRight(line.text, " \t")
		case protected[i]:
			blankRun = 0
			content = line.text
		case shorthand[i]:
			blankRun = 0
			content = strings.TrimSpace(line.text)
		case first == nil:
			blankRun = 0
			content = strings.TrimSpace(line.text)
		default:
			blankRun = 0
			content = renderLine(lineTokens[i], comments[i], opts)
		}
		if trimmed != "" && !protected[i] && !isIncludeLine(trimmed) && !strings.HasPrefix(trimmed, "-->8") {
			content = indentString(indentDepth, opts) + content
			if opts.TrimTrailingWhitespace {
				content = strings.TrimRight(content, " \t")
			}
		}
		out.WriteString(content)
		if len(line.eol) > 0 {
			out.WriteString(line.eol)
		}
		for _, token := range lineTokens[i] {
			switch token.Text {
			case "(", "[", "{":
				openDelims++
			case ")", "]", "}":
				if openDelims > 0 {
					openDelims--
				}
			}
		}
	}
	result := out.String()
	if opts.TrimFinalNewlines || opts.InsertFinalNewline {
		result = trimExtraFinalNewlines(result)
	}
	if opts.InsertFinalNewline && result != "" && !hasFinalEOL(result) {
		result += dominantEOL(lines)
	}
	if result == "" {
		return result, nil
	}
	// Formatting only changes whitespace. Refuse to return an edit if the
	// token or comment stream changed unexpectedly.
	verify := result
	if allowIncludes {
		verify = maskIncludes(result, splitLines(result))
	}
	verifyLexed := syntax.Lex([]byte(verify), nil)
	if len(verifyLexed.Errors) > 0 || !sameStream(lexed, verifyLexed) {
		return "", ErrUnsafe
	}
	return result, nil
}

func sameStream(before, after syntax.Result) bool {
	if len(before.Comments) != len(after.Comments) {
		return false
	}
	for i := range before.Comments {
		if before.Comments[i].Kind != after.Comments[i].Kind ||
			strings.TrimSpace(before.Comments[i].Text) != strings.TrimSpace(after.Comments[i].Text) {
			return false
		}
	}
	bi, ai := 0, 0
	for {
		for bi < len(before.Tokens) && before.Tokens[bi].Kind == syntax.TokenEOF {
			bi++
		}
		for ai < len(after.Tokens) && after.Tokens[ai].Kind == syntax.TokenEOF {
			ai++
		}
		if bi >= len(before.Tokens) || ai >= len(after.Tokens) {
			return bi >= len(before.Tokens) && ai >= len(after.Tokens)
		}
		if before.Tokens[bi].Kind != after.Tokens[ai].Kind || before.Tokens[bi].Text != after.Tokens[ai].Text {
			return false
		}
		bi++
		ai++
	}
}

func normalizeOptions(opts Options) Options {
	if opts.TabSize <= 0 {
		opts.TabSize = 2
	}
	if opts.MaxBlankLines < 0 {
		opts.MaxBlankLines = 2
	}
	return opts
}

func indentString(depth int, opts Options) string {
	if opts.InsertSpaces {
		return strings.Repeat(" ", depth*opts.TabSize)
	}
	return strings.Repeat("\t", depth)
}

func dominantEOL(lines []sourceLine) string {
	for _, line := range lines {
		if line.eol != "" {
			return line.eol
		}
	}
	return "\n"
}

// trimExtraFinalNewlines collapses a trailing run of line terminators to one.
// It keeps the final newline itself; use InsertFinalNewline to add one when the
// formatted text has none.
func trimExtraFinalNewlines(s string) string {
	end := len(s)
	start := end
	for start > 0 {
		if start >= 2 && s[start-2] == '\r' && s[start-1] == '\n' {
			start -= 2
			continue
		}
		if s[start-1] == '\n' || s[start-1] == '\r' {
			start--
			continue
		}
		break
	}
	if start == end {
		return s
	}
	keep := start + 1
	if start+1 < end && s[start] == '\r' && s[start+1] == '\n' {
		keep = start + 2
	}
	return s[:start] + s[start:keep]
}

func hasFinalEOL(s string) bool {
	return strings.HasSuffix(s, "\n") || strings.HasSuffix(s, "\r")
}

func renderLine(tokens []syntax.Token, comments []syntax.Comment, opts Options) string {
	if len(tokens) == 0 {
		return ""
	}
	var b strings.Builder
	for i, token := range tokens {
		if i > 0 && needsSpace(tokens, i) {
			b.WriteByte(' ')
		}
		b.WriteString(token.Text)
	}
	line := strings.TrimRight(b.String(), " \t")
	for _, comment := range comments {
		text := strings.TrimSpace(comment.Text)
		if text == "" {
			continue
		}
		if line != "" {
			line += "  "
		}
		line += text
	}
	return line
}

func needsSpace(tokens []syntax.Token, i int) bool {
	prev, cur := tokens[i-1], tokens[i]
	if cur.Is(",") || cur.Is(";") || cur.Is(")") || cur.Is("]") || cur.Is("}") ||
		cur.Is(".") || cur.Is(":") || cur.Is("::") {
		return false
	}
	if prev.Is("(") || prev.Is("[") || prev.Is("{") || prev.Is(".") || prev.Is(":") || prev.Is("::") {
		return false
	}
	if cur.Is("(") {
		if prev.Kind == syntax.TokenKeyword {
			switch strings.ToLower(prev.Text) {
			case "if", "while", "for":
				return true
			case "function":
				return false
			}
		}
		if prev.Kind == syntax.TokenName || prev.Kind == syntax.TokenString || prev.Kind == syntax.TokenNumber ||
			prev.Is(")") || prev.Is("]") || prev.Is("}") {
			return false
		}
		return true
	}
	if cur.Is("[") {
		if prev.Kind == syntax.TokenName || prev.Kind == syntax.TokenString || prev.Kind == syntax.TokenNumber ||
			prev.Is(")") || prev.Is("]") || prev.Is("}") {
			return false
		}
		return true
	}
	if isUnary(tokens, i) {
		return !prev.Is("(") && !prev.Is("[") && !prev.Is("{")
	}
	if isUnary(tokens, i-1) {
		return false
	}
	if prev.Is(",") || prev.Is(";") {
		return true
	}
	if prev.Kind == syntax.TokenKeyword {
		return true
	}
	if isOperator(prev) || isOperator(cur) {
		return true
	}
	return true
}

func isUnary(tokens []syntax.Token, i int) bool {
	t := tokens[i]
	if t.Is("#") || t.Is("@") || t.Is("$") {
		return true
	}
	if t.Text != "-" && t.Text != "~" && t.Text != "+" && t.Text != "%" {
		return false
	}
	if i == 0 {
		return true
	}
	prev := tokens[i-1]
	if prev.Is("(") || prev.Is("[") || prev.Is("{") || prev.Is(",") || prev.Is(";") || prev.Is("=") || isOperator(prev) {
		return true
	}
	if prev.Kind == syntax.TokenKeyword {
		switch strings.ToLower(prev.Text) {
		case "return", "then", "do", "else", "elseif", "local":
			return true
		}
	}
	return false
}

func isOperator(t syntax.Token) bool {
	if t.Kind == syntax.TokenKeyword {
		switch strings.ToLower(t.Text) {
		case "and", "or", "not":
			return true
		}
		return false
	}
	if t.Kind != syntax.TokenPunct {
		return false
	}
	switch t.Text {
	case "=", "==", "~=", "!=", "<", ">", "<=", ">=", "+", "-", "*", "/", "\\", "%", "^", "..", "//",
		"+=", "-=", "*=", "/=", "%=", "^=", "..=", "|", "&", "~":
		return true
	}
	return false
}

func firstToken(tokens []syntax.Token) *syntax.Token {
	if len(tokens) == 0 {
		return nil
	}
	return &tokens[0]
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func isIncludeLine(trimmed string) bool {
	return strings.HasPrefix(trimmed, "#include") && (len(trimmed) == len("#include") || trimmed[len("#include")] == ' ' || trimmed[len("#include")] == '\t')
}

func maskIncludes(src string, lines []sourceLine) string {
	b := []byte(src)
	for _, line := range lines {
		if !isIncludeLine(strings.TrimLeft(line.text, " \t")) {
			continue
		}
		for i := line.start; i < line.start+len(line.text); i++ {
			b[i] = ' '
		}
	}
	return string(b)
}

func sectionName(text string) (string, bool) {
	t := strings.TrimSpace(text)
	if len(t) < 4 || !strings.HasPrefix(t, "__") || !strings.HasSuffix(t, "__") {
		return "", false
	}
	name := t[2 : len(t)-2]
	if name == "" {
		return "", false
	}
	for _, r := range name {
		if r != '_' && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return "", false
		}
	}
	return name, true
}

type sourceLine struct {
	text  string
	eol   string
	start int
}

func splitLines(src string) []sourceLine {
	var out []sourceLine
	for i := 0; i < len(src); {
		start := i
		for i < len(src) && src[i] != '\n' && src[i] != '\r' {
			i++
		}
		contentEnd := i
		if i < len(src) {
			if src[i] == '\r' {
				i++
				if i < len(src) && src[i] == '\n' {
					i++
				}
			} else {
				i++
			}
		}
		out = append(out, sourceLine{text: src[start:contentEnd], eol: src[contentEnd:i], start: start})
	}
	return out
}

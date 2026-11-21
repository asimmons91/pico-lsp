// Package refactor implements the conservative, previewable source
// transformations exposed as LSP code actions. Every function returns edits in
// original-file coordinates and never writes to disk or mutates a buffer.
package refactor

import (
	"strings"

	"github.com/asimmons91/pico-lsp/internal/analysis"
	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// Code action kinds shared with the LSP layer.
const (
	KindExtractFunction  = "refactor.extract.function"
	KindGenerateClass    = "refactor.rewrite.class"
	KindGlobalToLocal    = "refactor.rewrite.local"
	KindShorthand        = "refactor.rewrite.shorthand"
	KindAddMissingFields = "quickfix.pico-lsp.add-missing-fields"
	KindRemoveUnused     = "quickfix.pico-lsp.remove-unused"
)

// Context is the analyzed state a refactor runs against.
type Context struct {
	Snap   *cart.Snapshot
	Result *analysis.Result
	FileID source.FileID
	File   *cart.File

	nodesBuilt bool
	nodeList   []syntax.Node
	parents    map[syntax.Node]syntax.Node
}

// Edit replaces [Start, End) in FileID with NewText.
type Edit struct {
	FileID  source.FileID
	Start   int
	End     int
	NewText string
}

// Confirmation describes an action that must be executed as a client command
// instead of applying an edit embedded in the code action. The LSP layer turns
// this into a workspace/executeCommand call so the editor can ask the user
// before any buffer changes.
type Confirmation struct {
	Span source.Span
}

// Action is one previewable refactor.
type Action struct {
	Title        string
	Kind         string
	Edits        []Edit
	Confirmation *Confirmation
	// TokenSavings is the exact positive token delta for token-saving actions.
	// Other actions leave it at zero.
	TokenSavings int
}

func (c *Context) currentText() string {
	if c == nil || c.File == nil {
		return ""
	}
	return c.File.Text
}

func (c *Context) valid() bool {
	return c != nil && c.Snap != nil && c.Snap.Cart != nil && c.Snap.Chunk != nil &&
		c.Result != nil && c.File != nil && c.File.Index != nil
}

func (c *Context) nodes() []syntax.Node {
	if c == nil || c.Snap == nil || c.Snap.Chunk == nil {
		return nil
	}
	if c.nodesBuilt {
		return c.nodeList
	}
	c.nodesBuilt = true
	c.parents = make(map[syntax.Node]syntax.Node)

	// Walk is pre-order, so a span stack recovers direct parents even when a
	// block and its only statement share exactly the same span.
	var stack []syntax.Node
	syntax.Walk(c.Snap.Chunk, func(node syntax.Node) bool {
		if node == nil {
			return false
		}
		span := node.Span()
		if span.FileID != c.FileID || span.IsEmpty() {
			return true
		}
		for len(stack) > 0 && !contains(stack[len(stack)-1].Span(), span) {
			stack = stack[:len(stack)-1]
		}
		if len(stack) > 0 {
			c.parents[node] = stack[len(stack)-1]
		}
		stack = append(stack, node)
		c.nodeList = append(c.nodeList, node)
		return true
	})
	return c.nodeList
}

func contains(outer, inner source.Span) bool {
	return outer.FileID == inner.FileID && outer.Start <= inner.Start && outer.End >= inner.End
}

func (c *Context) exactNode(start, end int) syntax.Node {
	if start >= end {
		return nil
	}
	var best syntax.Node
	bestLen := int(^uint(0) >> 1)
	for _, node := range c.nodes() {
		span := node.Span()
		if span.Start != start || span.End != end {
			continue
		}
		switch node.(type) {
		case syntax.Expr, syntax.Stmt:
		default:
			continue
		}
		if length := span.Len(); length < bestLen {
			best = node
			bestLen = length
		}
	}
	return best
}

func (c *Context) enclosingStatement(offset int) syntax.Stmt {
	var best syntax.Stmt
	bestLen := int(^uint(0) >> 1)
	for _, node := range c.nodes() {
		stmt, ok := node.(syntax.Stmt)
		if !ok {
			continue
		}
		span := stmt.Span()
		if offset < span.Start || offset > span.End {
			continue
		}
		if length := span.Len(); length < bestLen {
			best = stmt
			bestLen = length
		}
	}
	return best
}

func (c *Context) innermostTable(offset int) *syntax.TableExpr {
	var best *syntax.TableExpr
	bestLen := int(^uint(0) >> 1)
	for _, node := range c.nodes() {
		table, ok := node.(*syntax.TableExpr)
		if !ok || table == nil {
			continue
		}
		span := table.Span()
		if offset < span.Start || offset > span.End {
			continue
		}
		if length := span.Len(); length < bestLen {
			best = table
			bestLen = length
		}
	}
	return best
}

func (c *Context) parentOf(target syntax.Node) syntax.Node {
	if target == nil {
		return nil
	}
	c.nodes()
	return c.parents[target]
}

func (c *Context) parentBlock(stmt syntax.Stmt) *syntax.Block {
	if stmt == nil {
		return nil
	}
	if c.Snap != nil && c.Snap.Chunk != nil && c.Snap.Chunk.Block != nil {
		for _, candidate := range c.Snap.Chunk.Block.Stmts {
			if candidate == stmt {
				return c.Snap.Chunk.Block
			}
		}
	}
	var best *syntax.Block
	bestLen := int(^uint(0) >> 1)
	for _, node := range c.nodes() {
		block, ok := node.(*syntax.Block)
		if !ok || block == nil {
			continue
		}
		for _, candidate := range block.Stmts {
			if candidate == stmt {
				span := block.Span()
				if length := span.Len(); length < bestLen {
					best = block
					bestLen = length
				}
				break
			}
		}
	}
	return best
}

func (c *Context) smallestContainingStmt(span source.Span, not syntax.Stmt) syntax.Stmt {
	var best syntax.Stmt
	bestLen := int(^uint(0) >> 1)
	for _, node := range c.nodes() {
		stmt, ok := node.(syntax.Stmt)
		if !ok || stmt == not {
			continue
		}
		nodeSpan := stmt.Span()
		if !contains(nodeSpan, span) || nodeSpan == span {
			continue
		}
		if length := nodeSpan.Len(); length < bestLen {
			best = stmt
			bestLen = length
		}
	}
	return best
}

func (c *Context) lineStart(offset int) int {
	text := c.currentText()
	if offset < 0 {
		return 0
	}
	if offset > len(text) {
		offset = len(text)
	}
	for offset > 0 {
		switch text[offset-1] {
		case '\n', '\r':
			return offset
		}
		offset--
	}
	return offset
}

func (c *Context) lineEnd(offset int) int {
	text := c.currentText()
	if offset < 0 {
		return 0
	}
	if offset > len(text) {
		return len(text)
	}
	for offset < len(text) {
		switch text[offset] {
		case '\n', '\r':
			return offset
		}
		offset++
	}
	return offset
}

func (c *Context) lineAt(offset int) string {
	return c.currentText()[c.lineStart(offset):c.lineEnd(offset)]
}

func (c *Context) indentAt(offset int) string {
	line := c.lineAt(offset)
	trimmed := strings.TrimLeft(line, " \t")
	return line[:len(line)-len(trimmed)]
}

func (c *Context) indentUnit() string {
	text := c.currentText()
	seenTab := false
	minSpaces := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "\t") {
			seenTab = true
			continue
		}
		spaces := 0
		for spaces < len(line) && line[spaces] == ' ' {
			spaces++
		}
		if spaces > 0 && spaces < len(line) && (minSpaces == 0 || spaces < minSpaces) {
			minSpaces = spaces
		}
	}
	if seenTab && minSpaces == 0 {
		return "\t"
	}
	if minSpaces >= 4 {
		return "    "
	}
	return "  "
}

func (c *Context) hasComment(start, end int) bool {
	if c == nil || c.Snap == nil {
		return false
	}
	for _, comment := range c.Snap.Comments {
		if comment.Span.FileID != c.FileID {
			continue
		}
		if comment.Span.Start < end && comment.Span.End > start {
			return true
		}
	}
	return false
}

func (c *Context) hasCommentOnLines(start, end int) bool {
	lineStart := c.lineStart(start)
	lineEnd := c.lineEnd(end)
	return c.hasComment(lineStart, lineEnd)
}

// reindent moves text selected from baseIndent so that its first line starts at
// newIndent and continuation lines keep their relative indentation.
func reindent(text, baseIndent, newIndent string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		if i == 0 {
			line = strings.TrimLeft(line, " \t")
		} else {
			if strings.HasPrefix(line, baseIndent) {
				line = line[len(baseIndent):]
			} else {
				line = strings.TrimLeft(line, " \t")
			}
		}
		if line == "" {
			lines[i] = ""
			continue
		}
		lines[i] = newIndent + line
	}
	return strings.Join(lines, "\n")
}

func spanText(text string, span source.Span) string {
	if span.Start < 0 || span.End > len(text) || span.Start > span.End {
		return ""
	}
	return text[span.Start:span.End]
}

func validIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if r == '_' || r >= '0' && r <= '9' {
			if i == 0 && r >= '0' && r <= '9' {
				return false
			}
			continue
		}
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			continue
		}
		return false
	}
	return true
}

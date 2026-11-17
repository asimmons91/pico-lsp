package syntax

// CommentIndex indexes preserved comments by source line. Doc comments are
// also available separately for later annotation features.
type CommentIndex struct {
	All   []Comment
	Doc   []Comment
	lines map[int][]Comment
}

// NewCommentIndex builds an index over comments in source order.
func NewCommentIndex(comments []Comment) *CommentIndex {
	idx := &CommentIndex{
		All:   comments,
		lines: make(map[int][]Comment),
	}
	for _, comment := range comments {
		idx.lines[comment.Line] = append(idx.lines[comment.Line], comment)
		if comment.Kind == CommentDoc {
			idx.Doc = append(idx.Doc, comment)
		}
	}
	return idx
}

// ForLine returns comments whose first line is line.
func (i *CommentIndex) ForLine(line int) []Comment {
	if i == nil {
		return nil
	}
	return i.lines[line]
}

// DocBefore returns doc comments immediately preceding line, allowing blank
// lines between the last comment and the declaration line.
func (i *CommentIndex) DocBefore(line int) []Comment {
	if i == nil {
		return nil
	}
	var out []Comment
	for l := line - 1; l >= 0; l-- {
		comments := i.lines[l]
		if len(comments) == 0 {
			if len(out) > 0 {
				break
			}
			continue
		}
		for _, comment := range comments {
			if comment.Kind == CommentDoc {
				out = append([]Comment{comment}, out...)
			} else if len(out) > 0 {
				return out
			}
		}
	}
	return out
}

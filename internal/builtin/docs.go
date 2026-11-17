package builtin

import (
	"strings"

	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// Param is one documented API parameter.
type Param struct {
	Name     string
	Type     string
	Doc      string
	Optional bool
}

// Docs is the user-facing documentation attached to a built-in declaration.
type Docs struct {
	Summary    string
	Type       string
	Params     []Param
	Returns    []string
	Deprecated bool
}

func parseDocs(comments []syntax.Comment) *Docs {
	if len(comments) == 0 {
		return nil
	}
	docs := &Docs{}
	var summary []string
	for _, comment := range comments {
		if comment.Kind != syntax.CommentDoc {
			continue
		}
		text := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(comment.Text), "---"))
		if text == "" {
			if len(summary) > 0 {
				summary = append(summary, "")
			}
			continue
		}
		if strings.HasPrefix(text, "@") {
			fields := strings.Fields(text)
			if len(fields) == 0 {
				continue
			}
			switch fields[0] {
			case "@param":
				if len(fields) < 3 {
					continue
				}
				param := Param{
					Name:     strings.TrimSuffix(fields[1], "?"),
					Optional: strings.HasSuffix(fields[1], "?"),
					Type:     fields[2],
				}
				if len(fields) > 3 {
					param.Doc = strings.Join(fields[3:], " ")
				}
				docs.Params = append(docs.Params, param)
			case "@type":
				if len(fields) > 1 {
					docs.Type = strings.Join(fields[1:], " ")
				}
			case "@return":
				if len(fields) > 1 {
					docs.Returns = append(docs.Returns, strings.Join(fields[1:], " "))
				}
			case "@deprecated":
				docs.Deprecated = true
			}
			continue
		}
		summary = append(summary, text)
	}
	docs.Summary = strings.TrimSpace(strings.Join(summary, "\n"))
	return docs
}

func paramByName(docs *Docs, name string) (Param, bool) {
	if docs == nil {
		return Param{}, false
	}
	for _, param := range docs.Params {
		if strings.EqualFold(param.Name, name) {
			return param, true
		}
	}
	return Param{}, false
}

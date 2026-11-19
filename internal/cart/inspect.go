package cart

import "strings"

// Inspection is the pure text model of a .p8 cart. It never reads the
// filesystem, resolves includes, or flattens includes.
type Inspection struct {
	Version            int
	Sections           []Section
	Tabs               []Tab
	Includes           []IncludeDirective
	HasCode            bool
	Code               string
	CodeStart          int
	CodeEnd            int
	CodeStartLine      int
	CodeEndLine        int
	CodeUsesCRLF       bool
	HeaderUnterminated bool
	MalformedLines     []int
	DuplicateLuaLine   int
}

// IncludeDirective is one lexical #include directive in the root code body.
// Path has any .p8/.p8.png tab selector removed; Tab is -1 when there is no
// selector.
type IncludeDirective struct {
	Path string
	Tab  int
	Line int
}

// InspectText builds the pure text model for a .p8 cart.
func InspectText(text string) Inspection {
	scan := scanSections(text)
	inspection := Inspection{
		Version:          scan.version,
		Sections:         scan.sections,
		Tabs:             buildTabs(text, scan.sections, scan.code),
		DuplicateLuaLine: scan.duplicateLuaLine,
		MalformedLines:   scan.malformedLines,
	}
	for _, section := range scan.sections {
		if section.Name != "lua" {
			continue
		}
		inspection.HasCode = true
		inspection.CodeStart = section.ContentStart
		inspection.CodeEnd = section.SpanVal.End
		if inspection.CodeStart <= inspection.CodeEnd && inspection.CodeEnd <= len(text) {
			inspection.Code = text[inspection.CodeStart:inspection.CodeEnd]
		}
		inspection.CodeStartLine = section.ContentStartLine
		inspection.CodeEndLine = inspection.CodeStartLine + countCodeLines(inspection.Code)
		inspection.CodeUsesCRLF = strings.Contains(inspection.Code, "\r\n")
		inspection.HeaderUnterminated = !section.HeaderTerminated
		break
	}
	if inspection.HasCode {
		inspection.Includes = inspectIncludes(text, scan.code, inspection.CodeStartLine)
	}
	return inspection
}

func inspectIncludes(text string, code []lineRef, codeStartLine int) []IncludeDirective {
	var state scanState
	var out []IncludeDirective
	for i, line := range code {
		content := lineContent(text, line)
		if path, ok := includeDirective(content); ok && state.outside() {
			name, tab := splitIncludeTarget(path)
			out = append(out, IncludeDirective{Path: name, Tab: tab, Line: codeStartLine + i})
			continue
		}
		state.scan(content)
	}
	return out
}

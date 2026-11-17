package analysis

import (
	"errors"
	"fmt"
	"strings"

	"github.com/asimmons91/pico-lsp/internal/analysis/types"
	"github.com/asimmons91/pico-lsp/internal/cart"
	"github.com/asimmons91/pico-lsp/internal/source"
	"github.com/asimmons91/pico-lsp/internal/syntax"
)

// CastMode determines how a @cast directive changes a symbol's type.
type CastMode string

const (
	CastSet    CastMode = "set"
	CastAdd    CastMode = "add"
	CastRemove CastMode = "remove"
)

// CastDirective narrows or widens a named symbol from its line onward.
type CastDirective struct {
	Name   string
	Type   types.Type
	Mode   CastMode
	Span   source.Span
	Line   int
	FileID source.FileID
}

// AsDirective forces the preceding expression on its line to a type.
type AsDirective struct {
	Type   types.Type
	Span   source.Span
	Line   int
	FileID source.FileID
}

// DiagnosticDirective controls ---@diagnostic suppression.
type DiagnosticDirective struct {
	Action string
	Codes  []string
	Span   source.Span
	Line   int
	FileID source.FileID
}

type typeDeclarationSet struct {
	aliases   map[string]*types.Alias
	enums     map[string]*types.Record
	aliasRaw  map[string]string
	aliasSpan map[string]source.Span
	enumSpan  map[string]source.Span
}

func attachTypeAnnotations(snap *cart.Snapshot, result *Result, opts Options) {
	if snap == nil || snap.Cart == nil || result == nil || result.Table == nil {
		return
	}
	mode := opts.typeCheckingMode()
	decls := collectTypeDeclarations(snap)
	result.Aliases = decls.aliases
	result.Enums = decls.enums
	result.aliasRaw = decls.aliasRaw

	buildTypeNamespace(result)
	resolver := result.resolveType

	for canonical, raw := range decls.aliasRaw {
		alias := result.Aliases[canonical]
		if alias == nil {
			continue
		}
		if strings.TrimSpace(raw) == "" {
			alias.Underlying = types.AnyType
			result.addTypeDiagnostic(mode, CodeAnnotationInvalid, fmt.Sprintf("alias %q is missing a type", alias.Name), decls.aliasSpan[canonical], SeverityWarning)
			continue
		}
		parsed, err := parseType(raw, resolver)
		if err != nil {
			alias.Underlying = types.AnyType
			result.addTypeParseError(mode, err, decls.aliasSpan[canonical], fmt.Sprintf("alias %q", alias.Name))
			continue
		}
		alias.Underlying = parsed
	}

	for _, class := range result.Classes {
		if class == nil {
			continue
		}
		record := class.Record
		if record == nil {
			record = types.NewRecord(class.Name)
			class.Record = record
		}
		record.Exact = class.Exact
		record.Open = !class.Exact
		for _, parent := range class.Parents {
			parentType, ok := resolver(parent)
			if !ok {
				result.addTypeDiagnostic(mode, CodeAnnotationUnknownType, fmt.Sprintf("unknown parent class %q", parent), class.DeclSpan, SeverityWarning)
				continue
			}
			parentRecord, _ := types.Resolve(parentType).(*types.Record)
			if parentRecord == nil {
				result.addTypeDiagnostic(mode, CodeAnnotationInvalid, fmt.Sprintf("%q is not a class", parent), class.DeclSpan, SeverityWarning)
				continue
			}
			record.Parents = append(record.Parents, parentRecord)
		}
		for i := range class.Fields {
			field := &class.Fields[i]
			if strings.TrimSpace(field.Type) == "" {
				field.ParsedType = types.AnyType
				result.addTypeDiagnostic(mode, CodeAnnotationInvalid, fmt.Sprintf("field %q is missing a type", field.Name), field.Span, SeverityWarning)
				continue
			}
			parsed, err := parseType(field.Type, resolver)
			if err != nil {
				field.ParsedType = types.AnyType
				result.addTypeParseError(mode, err, field.Span, fmt.Sprintf("field %q", field.Name))
				continue
			}
			field.ParsedType = parsed
			record.AddField(&types.Field{
				Name:     field.Name,
				Type:     parsed,
				Optional: field.Optional,
				Decl:     field.Span,
			})
		}
	}

	attachSymbolTypes(result, resolver, mode)
	collectAnnotationDirectives(snap, result, resolver, mode)
	validateAnnotations(snap, result, mode)
}

func collectTypeDeclarations(snap *cart.Snapshot) *typeDeclarationSet {
	set := &typeDeclarationSet{
		aliases:   make(map[string]*types.Alias),
		enums:     make(map[string]*types.Record),
		aliasRaw:  make(map[string]string),
		aliasSpan: make(map[string]source.Span),
		enumSpan:  make(map[string]source.Span),
	}
	if snap == nil {
		return set
	}
	for _, comment := range snap.Comments {
		if comment.Kind != syntax.CommentDoc {
			continue
		}
		text := annotationText(comment)
		fields := strings.Fields(text)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "@alias":
			rest := strings.TrimSpace(text[len("@alias"):])
			name, typeText := cutFirstField(rest)
			name = strings.TrimSuffix(name, ":")
			if name == "" {
				continue
			}
			canonical := CanonicalName(name)
			if _, exists := set.aliases[canonical]; exists {
				continue
			}
			set.aliases[canonical] = &types.Alias{Name: name, Underlying: types.AnyType}
			set.aliasRaw[canonical] = typeText
			set.aliasSpan[canonical] = comment.Span
		case "@enum":
			rest := strings.TrimSpace(text[len("@enum"):])
			name, _ := cutFirstField(rest)
			name = strings.TrimSuffix(name, ":")
			if name == "" {
				continue
			}
			canonical := CanonicalName(name)
			if _, exists := set.enums[canonical]; exists {
				continue
			}
			record := types.NewRecord(name)
			record.Enum = true
			set.enums[canonical] = record
			set.enumSpan[canonical] = comment.Span
		}
	}
	return set
}

func buildTypeNamespace(result *Result) {
	result.typeNames = make(map[string]types.Type)
	for name, class := range result.Classes {
		if class == nil {
			continue
		}
		record := class.Record
		if record == nil {
			record = types.NewRecord(class.Name)
			class.Record = record
		}
		result.typeNames[name] = record
	}
	for name, alias := range result.Aliases {
		result.typeNames[name] = alias
	}
	for name, enum := range result.Enums {
		result.typeNames[name] = enum
	}
}

func (r *Result) resolveType(name string) (types.Type, bool) {
	if r == nil || r.typeNames == nil || name == "" {
		return nil, false
	}
	typ, ok := r.typeNames[CanonicalName(name)]
	return typ, ok
}

func attachSymbolTypes(result *Result, resolver TypeResolver, mode TypeCheckingMode) {
	for _, sym := range result.Table.Symbols {
		if sym == nil || sym.Doc == nil {
			continue
		}
		doc := sym.Doc
		if doc.Type != "" {
			parsed, err := parseType(doc.Type, resolver)
			if err != nil {
				result.addTypeParseError(mode, err, docSpan(doc, sym.DeclSpan), fmt.Sprintf("type for %q", sym.Name))
			} else {
				doc.ParsedType = parsed
			}
		}
		for i := range doc.Params {
			param := &doc.Params[i]
			if strings.TrimSpace(param.Type) == "" {
				continue
			}
			parsed, err := parseType(param.Type, resolver)
			if err != nil {
				result.addTypeParseError(mode, err, docSpan(doc, sym.DeclSpan), fmt.Sprintf("parameter %q", param.Name))
				continue
			}
			param.ParsedType = parsed
		}
		for _, raw := range doc.Returns {
			parsed, _, err := parseTypeListPrefix(raw, resolver)
			if err != nil {
				result.addTypeParseError(mode, err, docSpan(doc, sym.DeclSpan), "return type")
				continue
			}
			doc.ParsedReturns = append(doc.ParsedReturns, parsed...)
		}
		if doc.Class != "" {
			class := result.Classes[CanonicalName(doc.Class)]
			if class != nil && class.Record != nil {
				doc.ParsedType = class.Record
			} else {
				result.addTypeDiagnostic(mode, CodeAnnotationUnknownType, fmt.Sprintf("unknown class %q", doc.Class), docSpan(doc, sym.DeclSpan), SeverityWarning)
			}
		}
		if doc.Enum != "" {
			enum := result.Enums[CanonicalName(doc.Enum)]
			if enum != nil {
				doc.ParsedType = enum
			} else {
				result.addTypeDiagnostic(mode, CodeAnnotationUnknownType, fmt.Sprintf("unknown enum %q", doc.Enum), docSpan(doc, sym.DeclSpan), SeverityWarning)
			}
		}

		if isFunctionSymbol(sym) {
			attachFunctionType(result, sym, doc, resolver, mode)
			continue
		}
		if doc.ParsedType != nil {
			sym.Type = doc.ParsedType
			sym.TypeExplicit = true
		}
	}
}

func isFunctionSymbol(sym *Symbol) bool {
	if sym == nil {
		return false
	}
	return sym.Kind == SymbolFunction || sym.Kind == SymbolMethod || sym.Signature != ""
}

func attachFunctionType(result *Result, sym *Symbol, doc *DocComment, resolver TypeResolver, mode TypeCheckingMode) {
	if fnType, ok := types.AsFunction(doc.ParsedType); ok {
		for i := range fnType.Params {
			fnType.Params[i].Explicit = true
		}
		sym.Type = fnType
		sym.TypeExplicit = true
		setParamSymbolTypes(result, sym, fnType)
		return
	}
	fn := &types.Function{}
	explicit := false
	for _, paramName := range sym.Params {
		name := paramName
		paramDoc := findParamDoc(doc, name)
		var paramType types.Type = types.AnyType
		optional := false
		paramExplicit := false
		if paramDoc != nil {
			if paramDoc.ParsedType != nil {
				paramType = paramDoc.ParsedType
			}
			optional = paramDoc.Optional
			paramExplicit = paramDoc.Type != ""
		}
		if name == "..." {
			fn.Vararg = paramType
			explicit = explicit || paramExplicit
			continue
		}
		fn.Params = append(fn.Params, types.Param{
			Name:     name,
			Type:     paramType,
			Optional: optional,
			Doc:      paramDocDoc(paramDoc),
			Explicit: paramExplicit,
		})
		explicit = explicit || paramExplicit
	}
	if len(doc.ParsedReturns) > 0 {
		fn.Returns = types.TupleOf(doc.ParsedReturns...)
		explicit = true
	}
	sym.Type = fn
	sym.TypeExplicit = explicit
	setParamSymbolTypes(result, sym, fn)
}

func docSpan(doc *DocComment, fallback source.Span) source.Span {
	if doc != nil && !doc.Span.IsEmpty() {
		return doc.Span
	}
	return fallback
}

func findParamDoc(doc *DocComment, name string) *ParamDoc {
	if doc == nil {
		return nil
	}
	for i := range doc.Params {
		if strings.EqualFold(doc.Params[i].Name, name) {
			return &doc.Params[i]
		}
	}
	return nil
}

func paramDocDoc(param *ParamDoc) string {
	if param == nil {
		return ""
	}
	return param.Doc
}

func setParamSymbolTypes(result *Result, function *Symbol, fn *types.Function) {
	if result == nil || function == nil || fn == nil {
		return
	}
	byName := make(map[string]types.Param, len(fn.Params))
	for _, param := range fn.Params {
		byName[CanonicalName(param.Name)] = param
	}
	for _, sym := range result.Table.Symbols {
		if sym == nil || sym.Kind != SymbolParam || sym.Function != function {
			continue
		}
		if sym.Name == "..." {
			if fn.Vararg != nil {
				sym.Type = fn.Vararg
			}
			sym.TypeExplicit = function.TypeExplicit
			continue
		}
		if param, ok := byName[CanonicalName(sym.Name)]; ok {
			if param.Optional {
				sym.Type = types.Optional(param.Type)
			} else {
				sym.Type = param.Type
			}
			sym.TypeExplicit = param.Explicit
		}
	}
}

func collectAnnotationDirectives(snap *cart.Snapshot, result *Result, resolver TypeResolver, mode TypeCheckingMode) {
	if snap == nil {
		return
	}
	for _, comment := range snap.Comments {
		text := annotationText(comment)
		if strings.HasPrefix(text, "@cast") {
			collectCastDirective(result, text, comment, resolver, mode)
			continue
		}
		if strings.HasPrefix(text, "@diagnostic") {
			collectDiagnosticDirective(result, text, comment, mode)
			continue
		}
		if comment.Kind == syntax.CommentBlock {
			collectAsDirective(result, comment, resolver, mode)
		}
	}
}

func collectCastDirective(result *Result, text string, comment syntax.Comment, resolver TypeResolver, mode TypeCheckingMode) {
	rest := strings.TrimSpace(text[len("@cast"):])
	name, rest := cutFirstField(rest)
	name = strings.TrimSuffix(name, ":")
	if name == "" || rest == "" {
		result.addTypeDiagnostic(mode, CodeAnnotationInvalid, "cast requires a name and a type", comment.Span, SeverityWarning)
		return
	}
	for {
		rest = strings.TrimSpace(rest)
		if rest == "" {
			break
		}
		castMode := CastSet
		switch rest[0] {
		case '+':
			castMode = CastAdd
			rest = strings.TrimSpace(rest[1:])
		case '-':
			castMode = CastRemove
			rest = strings.TrimSpace(rest[1:])
		}
		parsed, end, err := parseTypePrefix(rest, resolver)
		if err != nil {
			result.addTypeParseError(mode, err, comment.Span, "cast type")
			return
		}
		result.Casts = append(result.Casts, CastDirective{
			Name:   name,
			Type:   parsed,
			Mode:   castMode,
			Span:   comment.Span,
			Line:   result.lineForSpan(comment.Span),
			FileID: comment.Span.FileID,
		})
		rest = strings.TrimSpace(rest[end:])
		if !strings.HasPrefix(rest, ",") {
			break
		}
		rest = strings.TrimSpace(rest[1:])
	}
}

func collectDiagnosticDirective(result *Result, text string, comment syntax.Comment, mode TypeCheckingMode) {
	rest := strings.TrimSpace(text[len("@diagnostic"):])
	state, codesText := rest, ""
	if idx := strings.Index(rest, ":"); idx >= 0 {
		state = strings.TrimSpace(rest[:idx])
		codesText = strings.TrimSpace(rest[idx+1:])
	} else if idx := strings.IndexAny(rest, " \t"); idx >= 0 {
		state = rest[:idx]
		codesText = strings.TrimSpace(rest[idx:])
	}
	state = strings.ToLower(state)
	switch state {
	case "disable", "enable", "disable-line", "disable-next-line":
	default:
		result.addTypeDiagnostic(mode, CodeAnnotationInvalid, fmt.Sprintf("unknown @diagnostic state %q", state), comment.Span, SeverityWarning)
		return
	}
	var codes []string
	for _, code := range strings.Split(codesText, ",") {
		code = strings.TrimSpace(code)
		if code != "" {
			codes = append(codes, code)
		}
	}
	result.DiagnosticDirectives = append(result.DiagnosticDirectives, DiagnosticDirective{
		Action: state,
		Codes:  codes,
		Span:   comment.Span,
		Line:   result.lineForSpan(comment.Span),
		FileID: comment.Span.FileID,
	})
}

func collectAsDirective(result *Result, comment syntax.Comment, resolver TypeResolver, mode TypeCheckingMode) {
	text := comment.Text
	lower := strings.ToLower(text)
	idx := strings.Index(lower, "@as")
	if idx < 0 {
		return
	}
	rest := strings.TrimSpace(text[idx+len("@as"):])
	rest = strings.TrimSuffix(rest, "]]")
	rest = strings.TrimSuffix(rest, "]]")
	rest = strings.TrimSpace(rest)
	parsed, _, err := parseTypePrefix(rest, resolver)
	if err != nil {
		result.addTypeParseError(mode, err, comment.Span, "@as")
		return
	}
	result.AsCasts = append(result.AsCasts, AsDirective{
		Type:   parsed,
		Span:   comment.Span,
		Line:   result.lineForSpan(comment.Span),
		FileID: comment.Span.FileID,
	})
}

func annotationText(comment syntax.Comment) string {
	text := strings.TrimSpace(comment.Text)
	text = strings.TrimPrefix(text, "---")
	text = strings.TrimPrefix(text, "--[[")
	text = strings.TrimPrefix(text, "--[=[")
	return strings.TrimSpace(text)
}

func validateAnnotations(snap *cart.Snapshot, result *Result, mode TypeCheckingMode) {
	if snap == nil {
		return
	}
	for _, comment := range snap.Comments {
		if comment.Kind != syntax.CommentDoc {
			continue
		}
		text := annotationText(comment)
		fields := strings.Fields(text)
		if len(fields) == 0 || !strings.HasPrefix(fields[0], "@") {
			continue
		}
		tag := strings.ToLower(strings.TrimSuffix(fields[0], ":"))
		switch tag {
		case "@class":
			if len(fields) < 2 {
				result.addTypeDiagnostic(mode, CodeAnnotationInvalid, "class annotation is missing a name", comment.Span, SeverityWarning)
			}
		case "@field":
			if !fieldAnnotationHasName(text) {
				result.addTypeDiagnostic(mode, CodeAnnotationMissingName, "field annotation is missing a name", comment.Span, SeverityHint)
			}
		case "@alias", "@enum", "@param", "@cast":
			if len(fields) < 2 {
				result.addTypeDiagnostic(mode, CodeAnnotationInvalid, fmt.Sprintf("%s annotation is missing a name", tag), comment.Span, SeverityWarning)
			}
		case "@return", "@type", "@diagnostic", "@deprecated", "@meta":
			// Recognized.
		case "@generic", "@overload", "@operator", "@module", "@package",
			"@private", "@protected", "@public", "@version", "@async",
			"@nodiscard", "@see", "@source", "@vararg":
			// Deferred annotations parse and are intentionally ignored.
		default:
			result.addTypeDiagnostic(mode, CodeAnnotationInvalid, fmt.Sprintf("unknown annotation %s", fields[0]), comment.Span, SeverityWarning)
		}
	}
	validateParamAnnotations(result, mode)
}

func fieldAnnotationHasName(text string) bool {
	fields := strings.Fields(strings.TrimSpace(text[len("@field"):]))
	if len(fields) == 0 {
		return false
	}
	if strings.HasPrefix(fields[0], "[") && strings.HasSuffix(fields[0], "]") {
		return len(fields) > 1
	}
	return true
}

func validateParamAnnotations(result *Result, mode TypeCheckingMode) {
	if result == nil {
		return
	}
	for _, sym := range result.Table.Symbols {
		if sym == nil || sym.Doc == nil || len(sym.Doc.Params) == 0 {
			continue
		}
		seen := make(map[string]struct{})
		for _, param := range sym.Doc.Params {
			canonical := CanonicalName(param.Name)
			if _, ok := seen[canonical]; ok {
				result.addTypeDiagnostic(mode, CodeAnnotationDuplicate, fmt.Sprintf("duplicate @param %q", param.Name), docSpan(sym.Doc, sym.DeclSpan), SeverityWarning)
			}
			seen[canonical] = struct{}{}
			if !symbolHasParam(sym, param.Name) {
				result.addTypeDiagnostic(mode, CodeAnnotationUnknownParam, fmt.Sprintf("unknown @param %q", param.Name), docSpan(sym.Doc, sym.DeclSpan), SeverityWarning)
			}
		}
	}
}

func symbolHasParam(sym *Symbol, name string) bool {
	for _, param := range sym.Params {
		if strings.EqualFold(param, name) || (param == "..." && name == "...") {
			return true
		}
	}
	return false
}

func (r *Result) addTypeParseError(mode TypeCheckingMode, err error, span source.Span, context string) {
	if err == nil {
		return
	}
	var unknown *unknownTypeError
	if errors.As(err, &unknown) {
		r.addTypeDiagnostic(mode, CodeAnnotationUnknownType, fmt.Sprintf("%s: unknown type %q", context, unknown.Name), span, SeverityWarning)
		return
	}
	r.addTypeDiagnostic(mode, CodeAnnotationInvalid, fmt.Sprintf("%s: %v", context, err), span, SeverityWarning)
}

func (r *Result) addTypeDiagnostic(mode TypeCheckingMode, code, message string, span source.Span, severity Severity) {
	if r == nil || mode == TypeCheckingOff || span.FileID < 0 || r.meta[span.FileID] {
		return
	}
	for _, existing := range r.Diagnostics {
		if existing.Code == code && existing.Span == span && existing.Message == message {
			return
		}
	}
	diag := Diagnostic{
		Code:     code,
		Message:  message,
		Span:     span,
		Severity: severity,
	}
	if r.diagLines == nil {
		r.diagLines = make(map[Diagnostic]int)
	}
	if r.cart != nil {
		if file := r.cart.File(span.FileID); file != nil && file.Index != nil {
			if pos, err := file.Index.Position(span.Start); err == nil {
				r.diagLines[diag] = int(pos.Line)
			}
		}
	}
	r.Diagnostics = append(r.Diagnostics, diag)
}

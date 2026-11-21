package analysis

import (
	"strings"
	"testing"
)

func occurrenceTexts(text string, occurrences []Occurrence) []string {
	out := make([]string, 0, len(occurrences))
	for _, occ := range occurrences {
		out = append(out, text[occ.Span.Start:occ.Span.End])
	}
	return out
}

func TestAnnotationParamOccurrences(t *testing.T) {
	text := strings.Join([]string{
		"---@param dx? number",
		"function move(dx)",
		"  return dx",
		"end",
		"",
	}, "\n")
	result, _ := analyzeText(t, text)
	move := result.Table.Globals["move"]
	if move == nil {
		t.Fatal("missing move")
	}
	param := result.paramSymbolsFor(move)["dx"]
	if param == nil {
		t.Fatal("missing dx parameter")
	}
	occs := result.OccurrencesForSymbol(param)
	if len(occs) != 3 {
		t.Fatalf("occurrences = %+v (%v), want annotation, declaration, use", occs, occurrenceTexts(text, occs))
	}
	if !occs[0].Annotation || occs[0].Text != "dx" {
		t.Fatalf("annotation occurrence = %+v, want dx", occs[0])
	}
	if got := text[occs[0].Span.Start:occs[0].Span.End]; got != "dx" {
		t.Fatalf("annotation span = %q, want dx (optional marker excluded)", got)
	}
	if span := result.Definitions(param); len(span) != 1 || text[span[0].Start:span[0].End] != "dx" {
		t.Fatalf("definitions = %+v", span)
	}
	refs := result.References(param, true)
	if len(refs) != 3 {
		t.Fatalf("references = %d, want 3", len(refs))
	}
	annotationOffset := strings.Index(text, "dx?")
	occ := result.OccurrenceAt(0, annotationOffset)
	if occ == nil || occ.Symbol != param || !occ.Annotation {
		t.Fatalf("OccurrenceAt(annotation) = %+v", occ)
	}
	if !result.Renameable(param) {
		t.Fatal("parameter should be renameable")
	}
	if conflict := result.RenameConflict(param, "dy"); conflict != nil {
		t.Fatalf("unexpected conflict: %+v", conflict)
	}
}

func TestAnnotationFieldOccurrencesAndRenameFamily(t *testing.T) {
	text := strings.Join([]string{
		"---@class Player",
		"---@field [public] hp? number",
		"---@field mp number",
		"local Player = {}",
		"Player.hp = 1",
		"---@type Player",
		"local p = Player",
		"p.hp = 2",
		"---@type Player",
		"local q = Player",
		"print(q.hp)",
		"",
	}, "\n")
	result, _ := analyzeText(t, text)

	classField := result.ClassFields["player"]["hp"]
	if classField == nil {
		t.Fatal("missing canonical hp field")
	}
	if len(classField.Decls) != 1 {
		t.Fatalf("canonical declarations = %+v", classField.Decls)
	}
	if got := text[classField.Decls[0].Start:classField.Decls[0].End]; got != "hp" {
		t.Fatalf("field annotation span = %q, want hp (optional marker excluded)", got)
	}
	player := findSymbolByCanonical(result, "player")
	if player == nil {
		t.Fatal("missing Player symbol")
	}
	runtime := player.Fields["hp"]
	if runtime == nil || runtime.ClassField != classField {
		t.Fatalf("Player.hp = %+v, want linked runtime field", runtime)
	}
	// The read on q has no code declaration and falls back to `---@field`.
	q := findSymbolByCanonical(result, "q")
	if q == nil {
		t.Fatal("missing q")
	}
	qhp := q.Fields["hp"]
	if qhp == nil || qhp.ClassField != classField {
		t.Fatalf("q.hp = %+v, want linked runtime field", qhp)
	}
	if defs := result.Definitions(qhp); len(defs) != 1 || defs[0] != classField.Decls[0] {
		t.Fatalf("q.hp definitions = %+v, want annotation declaration", defs)
	}
	if !result.SameSymbol(qhp, runtime) || !result.SameSymbol(qhp, classField) {
		t.Fatal("class field family should compare equal")
	}

	occs := result.OccurrencesForSymbol(runtime)
	if len(occs) != 4 {
		t.Fatalf("field occurrences = %v, want annotation, Player.hp declaration, p.hp write, q.hp read", occurrenceTexts(text, occs))
	}
	for _, occ := range occs {
		if got := text[occ.Span.Start:occ.Span.End]; got != "hp" {
			t.Fatalf("field occurrence span = %q, want hp", got)
		}
	}
	if !result.Renameable(runtime) {
		t.Fatal("class field should be renameable")
	}
	if conflict := result.RenameConflict(runtime, "mp"); conflict == nil {
		t.Fatal("renaming hp to mp should conflict with the mp field")
	}
	if conflict := result.RenameConflict(runtime, "armor"); conflict != nil {
		t.Fatalf("unexpected conflict: %+v", conflict)
	}
}

func TestAnnotationDuplicateClassDeclarations(t *testing.T) {
	text := strings.Join([]string{
		"---@class Player",
		"local Player = {}",
		"---@class Player",
		"local Alias = Player",
		"",
	}, "\n")
	result, _ := analyzeText(t, text)
	class := result.TypeSymbols["player"]
	if class == nil {
		t.Fatal("missing class symbol")
	}
	if len(class.Decls) != 2 {
		t.Fatalf("class declarations = %v, want both @class lines", class.Decls)
	}
}

func TestAnnotationDuplicateFieldDeclarations(t *testing.T) {
	text := strings.Join([]string{
		"---@class Player",
		"---@field hp number",
		"---@field hp number",
		"local Player = {}",
		"",
	}, "\n")
	result, _ := analyzeText(t, text)
	classField := result.ClassFields["player"]["hp"]
	if classField == nil {
		t.Fatal("missing canonical hp field")
	}
	if len(classField.Decls) != 2 {
		t.Fatalf("declarations = %v, want both @field lines", classField.Decls)
	}
}

func TestAnnotationInheritedFieldOccurrences(t *testing.T) {
	text := strings.Join([]string{
		"---@class Base",
		"---@field hp number",
		"---@class Player: Base",
		"---@type Player",
		"local p = nil",
		"p.hp = 1",
		"print(p.hp)",
		"",
	}, "\n")
	result, _ := analyzeText(t, text)
	baseField := result.ClassFields["base"]["hp"]
	if baseField == nil {
		t.Fatal("missing Base.hp field")
	}
	p := findSymbolByCanonical(result, "p")
	if p == nil {
		t.Fatal("missing p")
	}
	pField := p.Fields["hp"]
	if pField == nil || pField.ClassField != baseField {
		t.Fatalf("p.hp = %+v, want linked to Base.hp", pField)
	}
	occs := result.OccurrencesForSymbol(pField)
	if len(occs) != 3 {
		t.Fatalf("inherited field occurrences = %v, want annotation, write, read", occurrenceTexts(text, occs))
	}
	if !result.Renameable(pField) {
		t.Fatal("inherited field should be renameable")
	}
}

func TestAnnotationParamFieldOccurrences(t *testing.T) {
	text := strings.Join([]string{
		"---@class Player",
		"---@field hp number",
		"local Player = {}",
		"---@param p Player",
		"local function damage(p)",
		"  p.hp = p.hp - 1",
		"end",
		"",
	}, "\n")
	result, _ := analyzeText(t, text)
	classField := result.ClassFields["player"]["hp"]
	if classField == nil {
		t.Fatal("missing canonical hp field")
	}
	damage := findSymbolByCanonical(result, "damage")
	if damage == nil {
		t.Fatal("missing damage function")
	}
	p := result.paramSymbolsFor(damage)["p"]
	if p == nil {
		t.Fatal("missing p parameter")
	}
	pField := p.Fields["hp"]
	if pField == nil || pField.ClassField != classField {
		t.Fatalf("parameter p.hp = %+v, want linked to Player.hp", pField)
	}
	occs := result.OccurrencesForSymbol(pField)
	if len(occs) != 3 {
		t.Fatalf("parameter field occurrences = %v, want annotation, write, read", occurrenceTexts(text, occs))
	}
}

func TestAnnotationOptionalClassReceiver(t *testing.T) {
	text := strings.Join([]string{
		"---@class Player",
		"---@field hp number",
		"---@class Enemy",
		"---@field hp number",
		"local Player = {}",
		"---@type Player|nil",
		"local p = Player",
		"p.hp = 1",
		"---@type Player|Enemy",
		"local mixed = nil",
		"mixed.hp = 1",
		"",
	}, "\n")
	result, _ := analyzeText(t, text)
	playerField := result.ClassFields["player"]["hp"]
	if playerField == nil {
		t.Fatal("missing Player.hp")
	}
	p := findSymbolByCanonical(result, "p")
	if p == nil || p.Fields["hp"] == nil || p.Fields["hp"].ClassField != playerField {
		t.Fatalf("optional receiver p.hp = %+v, want Player.hp", p)
	}
	mixed := findSymbolByCanonical(result, "mixed")
	if mixed == nil || mixed.Fields["hp"] == nil {
		t.Fatalf("missing mixed.hp: %+v", mixed)
	}
	if mixed.Fields["hp"].ClassField != nil {
		t.Fatalf("ambiguous union receiver linked to %+v", mixed.Fields["hp"].ClassField)
	}
}

func TestAnnotationSelfFieldOccurrences(t *testing.T) {
	text := strings.Join([]string{
		"---@class Player",
		"---@field hp number",
		"local Player = {}",
		"function Player:hit()",
		"  self.hp = 1",
		"end",
		"",
	}, "\n")
	result, _ := analyzeText(t, text)
	classField := result.ClassFields["player"]["hp"]
	if classField == nil {
		t.Fatal("missing canonical hp field")
	}
	player := findSymbolByCanonical(result, "player")
	if player == nil {
		t.Fatal("missing Player")
	}
	method := player.Fields["hit"]
	if method == nil {
		t.Fatalf("missing hit method: %+v", method)
	}
	self := result.paramSymbolsFor(method)["self"]
	if self == nil {
		t.Fatal("missing implicit self")
	}
	selfField := self.Fields["hp"]
	if selfField == nil || selfField.ClassField != classField {
		t.Fatalf("self.hp = %+v, want linked to Player.hp", selfField)
	}
	occs := result.OccurrencesForSymbol(selfField)
	if len(occs) != 2 {
		t.Fatalf("self field occurrences = %v, want annotation and write", occurrenceTexts(text, occs))
	}
}

func TestAnnotationFieldDynamicAccessBlocksRename(t *testing.T) {
	text := strings.Join([]string{
		"---@class Player",
		"---@field hp number",
		"local Player = {}",
		"---@type Player",
		"local p = Player",
		"p.hp = 1",
		"p[\"other\"] = 2",
		"",
	}, "\n")
	result, _ := analyzeText(t, text)
	classField := result.ClassFields["player"]["hp"]
	if classField == nil {
		t.Fatal("missing canonical hp field")
	}
	receiver := findSymbolByCanonical(result, "p")
	if receiver == nil || !receiver.DynamicFields {
		t.Fatalf("p dynamic fields = %+v", receiver)
	}
	if result.Renameable(classField) {
		t.Fatal("field with dynamic access should not be renameable")
	}
	if conflict := result.RenameConflict(classField, "armor"); conflict == nil {
		t.Fatal("dynamic access should produce a rename conflict")
	}
}

func TestAnnotationTypeReferences(t *testing.T) {
	text := strings.Join([]string{
		"---@class Player",
		"local Player = {}",
		"",
		"---@param p Player",
		"---@return Player",
		"local function identity(p)",
		"  return p",
		"end",
		"",
		"---@type Player",
		"local player = Player",
		"",
	}, "\n")
	result, _ := analyzeText(t, text)
	class := result.TypeSymbols["player"]
	if class == nil {
		t.Fatal("missing class symbol")
	}
	occs := result.OccurrencesForSymbol(class)
	if len(occs) != 4 {
		t.Fatalf("class occurrences = %v, want declaration plus three type references", occurrenceTexts(text, occs))
	}
	if !occs[0].Annotation || !occs[0].Decl {
		t.Fatalf("class declaration occurrence = %+v", occs[0])
	}
	paramOffset := strings.Index(text, "---@param p Player") + len("---@param p ")
	occ := result.OccurrenceAt(0, paramOffset)
	if occ == nil || occ.Symbol != class || !occ.Annotation {
		t.Fatalf("OccurrenceAt(type reference) = %+v", occ)
	}
	if refs := result.References(class, true); len(refs) != 4 {
		t.Fatalf("references = %d, want 4", len(refs))
	}
	if refs := result.References(class, false); len(refs) != 3 {
		t.Fatalf("references without declaration = %d, want 3", len(refs))
	}
	if !result.Renameable(class) {
		t.Fatal("class should be renameable")
	}
}

func TestAnnotationAsTypeReference(t *testing.T) {
	text := strings.Join([]string{
		"---@class Player",
		"local Player = {}",
		"local f = nil --[[@as Player]]",
		"",
	}, "\n")
	result, _ := analyzeText(t, text)
	class := result.TypeSymbols["player"]
	if class == nil {
		t.Fatal("missing class symbol")
	}
	occs := result.OccurrencesForSymbol(class)
	if len(occs) != 2 {
		t.Fatalf("class occurrences = %v, want declaration and @as reference", occurrenceTexts(text, occs))
	}
	asOffset := strings.Index(text, "--[[@as ") + len("--[[@as ")
	if occ := result.OccurrenceAt(0, asOffset); occ == nil || occ.Symbol != class {
		t.Fatalf("OccurrenceAt(@as) = %+v", occ)
	}
}

func TestAnnotationTypeRenameConflictAndAliases(t *testing.T) {
	text := strings.Join([]string{
		"---@class Player",
		"local Player = {}",
		"---@class Enemy",
		"local Enemy = {}",
		"---@alias Mode string",
		"---@param m Mode",
		"local function set_mode(m) end",
		"",
	}, "\n")
	result, _ := analyzeText(t, text)
	player := result.TypeSymbols["player"]
	enemy := result.TypeSymbols["enemy"]
	mode := result.TypeSymbols["mode"]
	if player == nil || enemy == nil || mode == nil {
		t.Fatalf("type symbols = %+v", result.TypeSymbols)
	}
	if conflict := result.RenameConflict(player, "enemy"); conflict != enemy {
		t.Fatalf("conflict = %+v, want Enemy", conflict)
	}
	if conflict := result.RenameConflict(player, "mode"); conflict != mode {
		t.Fatalf("conflict = %+v, want Mode", conflict)
	}
	if occs := result.OccurrencesForSymbol(mode); len(occs) != 2 {
		t.Fatalf("alias occurrences = %v, want declaration and parameter type", occurrenceTexts(text, occs))
	}
	if !result.Renameable(mode) {
		t.Fatal("alias should be renameable")
	}
}

func TestAnnotationTypeNamespaceDoesNotCollideWithCode(t *testing.T) {
	text := strings.Join([]string{
		"---@class Player",
		"local Player = {}",
		"local p = 1",
		"",
	}, "\n")
	result, _ := analyzeText(t, text)
	class := result.TypeSymbols["player"]
	if class == nil {
		t.Fatal("missing class symbol")
	}
	if conflict := result.RenameConflict(class, "p"); conflict != nil {
		t.Fatalf("type rename collided with a code local: %+v", conflict)
	}
}

func TestAnnotationPrimitiveNamesDoNotBindToTypes(t *testing.T) {
	text := strings.Join([]string{
		"---@class string",
		"local StringClass = {}",
		"---@type string",
		"local value = nil",
		"",
	}, "\n")
	result, _ := analyzeText(t, text)
	class := result.TypeSymbols["string"]
	if class == nil {
		t.Fatal("missing string class")
	}
	occs := result.OccurrencesForSymbol(class)
	if len(occs) != 1 {
		t.Fatalf("string occurrences = %v, want only the class declaration", occurrenceTexts(text, occs))
	}
}

func TestAnnotationClassGenericsDoNotBindInMethodDocs(t *testing.T) {
	text := strings.Join([]string{
		"---@class T",
		"local Outside = {}",
		"---@class Box<T>",
		"---@field value T",
		"local Box = {}",
		"---@return T",
		"function Box:get()",
		"  return self.value",
		"end",
		"---@type T",
		"local outside = nil",
		"",
	}, "\n")
	result, _ := analyzeText(t, text)
	classT := result.TypeSymbols["t"]
	if classT == nil {
		t.Fatal("missing T class")
	}
	classOffset := strings.Index(text, "---@class T") + len("---@class ")
	typeOffset := strings.Index(text, "---@type T") + len("---@type ")
	occs := result.OccurrencesForSymbol(classT)
	if len(occs) != 2 {
		t.Fatalf("T occurrences = %v, want class declaration and the @type T reference", occurrenceTexts(text, occs))
	}
	for _, occ := range occs {
		if occ.Span.Start != classOffset && occ.Span.Start != typeOffset {
			t.Fatalf("class T bound to a generic reference at %d: %+v", occ.Span.Start, occ)
		}
	}
}

func TestAnnotationGenericNamesDoNotBindToTypes(t *testing.T) {
	text := strings.Join([]string{
		"---@class T",
		"local T = {}",
		"---@generic T",
		"---@param value T",
		"local function identity(value)",
		"  return value",
		"end",
		"",
	}, "\n")
	result, _ := analyzeText(t, text)
	class := result.TypeSymbols["t"]
	if class == nil {
		t.Fatal("missing T class")
	}
	occs := result.OccurrencesForSymbol(class)
	if len(occs) != 1 {
		t.Fatalf("T occurrences = %v, want only the class declaration", occurrenceTexts(text, occs))
	}
}

func TestAnnotationFunctionParamNamesDoNotBindToTypes(t *testing.T) {
	text := strings.Join([]string{
		"---@class x",
		"local X = {}",
		"---@type fun(x: number)",
		"local f = nil",
		"",
	}, "\n")
	result, _ := analyzeText(t, text)
	class := result.TypeSymbols["x"]
	if class == nil {
		t.Fatal("missing x class")
	}
	occs := result.OccurrencesForSymbol(class)
	if len(occs) != 1 {
		t.Fatalf("x occurrences = %v, want only the class declaration", occurrenceTexts(text, occs))
	}
}

func TestAnnotationCastTarget(t *testing.T) {
	text := strings.Join([]string{
		"---@class Player",
		"local Player = {}",
		"---@type Player|nil",
		"local p = nil",
		"---@cast p: Player",
		"print(p)",
		"",
	}, "\n")
	result, _ := analyzeText(t, text)
	var p *Symbol
	for _, sym := range result.Table.Symbols {
		if sym != nil && sym.Kind == SymbolLocal && sym.Canonical == "p" {
			p = sym
			break
		}
	}
	if p == nil {
		t.Fatal("missing p")
	}
	occs := result.OccurrencesForSymbol(p)
	cast := false
	for _, occ := range occs {
		if occ.Annotation && text[occ.Span.Start:occ.Span.End] == "p" {
			cast = true
		}
	}
	if !cast {
		t.Fatalf("cast occurrence missing: %v", occurrenceTexts(text, occs))
	}
	castOffset := strings.Index(text, "---@cast p: Player") + len("---@cast ")
	if occ := result.OccurrenceAt(0, castOffset); occ == nil || occ.Symbol != p {
		t.Fatalf("OccurrenceAt(cast) = %+v", occ)
	}
}

func findSymbolByCanonical(result *Result, canonical string) *Symbol {
	for _, sym := range result.Table.Symbols {
		if sym != nil && sym.Canonical == canonical {
			return sym
		}
	}
	return nil
}

package analysis

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/asimmons91/pico-lsp/internal/analysis/types"
)

// TypeResolver resolves a named type from the cart's type namespace. The name
// is passed as written; implementations should compare case-insensitively.
type TypeResolver func(name string) (types.Type, bool)

type unknownTypeError struct {
	Name string
}

func (e *unknownTypeError) Error() string {
	return fmt.Sprintf("unknown type %q", e.Name)
}

// parseType parses a complete type expression. Trailing non-whitespace text is
// an error; use parseTypePrefix when the type is followed by documentation.
func parseType(input string, resolve TypeResolver) (types.Type, error) {
	typ, end, err := parseTypePrefix(input, resolve)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input[end:]) != "" {
		return nil, fmt.Errorf("unexpected %q after type", strings.TrimSpace(input[end:]))
	}
	return typ, nil
}

// parseTypePrefix parses the longest type expression at the start of input and
// returns the number of bytes consumed.
func parseTypePrefix(input string, resolve TypeResolver) (types.Type, int, error) {
	p := &typeParser{input: input, resolve: resolve}
	typ, err := p.parseUnion()
	if err != nil {
		return nil, 0, err
	}
	return typ, p.pos, nil
}

type typeParser struct {
	input   string
	pos     int
	resolve TypeResolver
}

func (p *typeParser) parseUnion() (types.Type, error) {
	first, err := p.parsePostfix()
	if err != nil {
		return nil, err
	}
	members := []types.Type{first}
	for {
		p.skipSpace()
		if !p.consume("|") {
			break
		}
		next, err := p.parsePostfix()
		if err != nil {
			return nil, err
		}
		members = append(members, next)
	}
	return types.UnionOf(members...), nil
}

func (p *typeParser) parsePostfix() (types.Type, error) {
	typ, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		p.skipSpace()
		switch {
		case p.consume("?"):
			typ = types.Optional(typ)
		case p.consume("[]"):
			typ = types.Array{Elem: typ}
		default:
			return typ, nil
		}
	}
}

func (p *typeParser) parsePrimary() (types.Type, error) {
	p.skipSpace()
	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("expected a type")
	}
	switch p.input[p.pos] {
	case '(':
		p.pos++
		typ, err := p.parseUnion()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if !p.consume(")") {
			return nil, fmt.Errorf("expected ')'")
		}
		return typ, nil
	case '{':
		return p.parseRecord()
	case '[':
		return p.parseTuple()
	case '"', '\'':
		value, err := p.parseStringLiteral()
		if err != nil {
			return nil, err
		}
		return types.Literal{Value: value}, nil
	}

	if isNumberStart(p.input[p.pos]) {
		value, err := p.parseNumberLiteral()
		if err != nil {
			return nil, err
		}
		return types.Literal{Value: value}, nil
	}

	name := p.parseName()
	if name == "" {
		return nil, fmt.Errorf("expected a type name")
	}
	switch strings.ToLower(name) {
	case "nil":
		return types.NilType, nil
	case "any":
		return types.AnyType, nil
	case "boolean":
		return types.BooleanType, nil
	case "number":
		return types.NumberType, nil
	case "integer":
		return types.IntegerType, nil
	case "string":
		return types.StringType, nil
	case "table":
		p.skipSpace()
		if p.consume("<") {
			key, err := p.parseUnion()
			if err != nil {
				return nil, err
			}
			p.skipSpace()
			if !p.consume(",") {
				return nil, fmt.Errorf("expected ',' in table type")
			}
			elem, err := p.parseUnion()
			if err != nil {
				return nil, err
			}
			p.skipSpace()
			if !p.consume(">") {
				return nil, fmt.Errorf("expected '>' in table type")
			}
			return types.Map{Key: key, Elem: elem}, nil
		}
		return types.TableType, nil
	case "function":
		return types.FunctionType, nil
	case "thread":
		return types.ThreadType, nil
	case "userdata":
		return types.UserdataType, nil
	case "fun":
		return p.parseFunction()
	case "true":
		return types.Literal{Value: true}, nil
	case "false":
		return types.Literal{Value: false}, nil
	}

	// Named class, alias, or enum.
	var genericArgs []types.Type
	p.skipSpace()
	if p.consume("<") {
		args, err := p.parseGenericArguments()
		if err != nil {
			return nil, err
		}
		genericArgs = args
	}
	if p.resolve != nil {
		if typ, ok := p.resolve(name); ok && typ != nil {
			if len(genericArgs) > 0 {
				if types.IsAny(typ) {
					return typ, nil
				}
				applied, err := types.Apply(typ, genericArgs)
				if err != nil {
					return nil, err
				}
				return applied, nil
			}
			if record, ok := types.Resolve(typ).(*types.Record); ok && record != nil && len(record.TypeParams) > 0 {
				allDefaults := true
				for _, param := range record.TypeParams {
					if param == nil || param.Default == nil {
						allDefaults = false
						break
					}
				}
				if allDefaults {
					applied, err := types.Apply(typ, nil)
					if err == nil {
						return applied, nil
					}
				}
			}
			return typ, nil
		}
	}
	return nil, &unknownTypeError{Name: name}
}

func (p *typeParser) parseGenericArguments() ([]types.Type, error) {
	var args []types.Type
	for {
		p.skipSpace()
		if p.consume(">") {
			if len(args) == 0 {
				return nil, fmt.Errorf("expected a type argument")
			}
			return args, nil
		}
		arg, err := p.parseUnion()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		p.skipSpace()
		if p.consume(",") {
			continue
		}
		if p.consume(">") {
			return args, nil
		}
		return nil, fmt.Errorf("expected ',' or '>' in type arguments")
	}
}

func (p *typeParser) parseRecord() (types.Type, error) {
	if !p.consume("{") {
		return nil, fmt.Errorf("expected '{'")
	}
	record := types.NewRecord("")
	for {
		p.skipSpace()
		if p.consume("}") {
			return record, nil
		}
		if p.pos >= len(p.input) {
			return nil, fmt.Errorf("unterminated record type")
		}
		name := p.parseName()
		if name == "" {
			return nil, fmt.Errorf("expected a field name")
		}
		optional := p.consume("?")
		p.skipSpace()
		if !p.consume(":") {
			return nil, fmt.Errorf("expected ':' after field %q", name)
		}
		fieldType, err := p.parseUnion()
		if err != nil {
			return nil, err
		}
		record.AddField(&types.Field{Name: name, Type: fieldType, Optional: optional})
		p.skipSpace()
		if p.consume(",") || p.consume(";") {
			continue
		}
		if !p.consume("}") {
			return nil, fmt.Errorf("expected ',' or '}' in record type")
		}
		return record, nil
	}
}

func (p *typeParser) parseTuple() (types.Type, error) {
	if !p.consume("[") {
		return nil, fmt.Errorf("expected '['")
	}
	var elems []types.Type
	for {
		p.skipSpace()
		if p.consume("]") {
			return types.TupleOf(elems...), nil
		}
		typ, err := p.parseUnion()
		if err != nil {
			return nil, err
		}
		elems = append(elems, typ)
		p.skipSpace()
		if p.consume(",") {
			continue
		}
		if !p.consume("]") {
			return nil, fmt.Errorf("expected ',' or ']' in tuple type")
		}
		return types.TupleOf(elems...), nil
	}
}

func (p *typeParser) parseFunction() (types.Type, error) {
	fn := &types.Function{}
	if !p.consume("(") {
		return nil, fmt.Errorf("expected '(' after fun")
	}
	for {
		p.skipSpace()
		if p.consume(")") {
			break
		}
		if p.consume("...") {
			if p.consume(":") {
				vararg, err := p.parseUnion()
				if err != nil {
					return nil, err
				}
				fn.Vararg = vararg
			} else {
				fn.Vararg = types.AnyType
			}
		} else {
			name := p.parseName()
			optional := p.consume("?")
			var paramType types.Type = types.AnyType
			p.skipSpace()
			if p.consume(":") {
				parsed, err := p.parseUnion()
				if err != nil {
					return nil, err
				}
				paramType = parsed
			} else if name == "" {
				// A bare type in a function type is a positional parameter.
				p.pos -= 0
				parsed, err := p.parseUnion()
				if err != nil {
					return nil, err
				}
				paramType = parsed
			}
			fn.Params = append(fn.Params, types.Param{Name: name, Type: paramType, Optional: optional})
		}
		p.skipSpace()
		if p.consume(",") {
			continue
		}
		if !p.consume(")") {
			return nil, fmt.Errorf("expected ',' or ')' in function type")
		}
		break
	}
	p.skipSpace()
	if p.consume(":") {
		returns, err := p.parseTypeList()
		if err != nil {
			return nil, err
		}
		fn.Returns = returns
	}
	return fn, nil
}

func (p *typeParser) parseTypeList() (types.Tuple, error) {
	var elems []types.Type
	for {
		typ, err := p.parseUnion()
		if err != nil {
			return types.Tuple{}, err
		}
		elems = append(elems, typ)
		p.skipSpace()
		if !p.consume(",") {
			return types.TupleOf(elems...), nil
		}
	}
}

func (p *typeParser) parseName() string {
	p.skipSpace()
	start := p.pos
	for p.pos < len(p.input) {
		r, size := utf8.DecodeRuneInString(p.input[p.pos:])
		if r == '_' || r == '.' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			p.pos += size
			continue
		}
		break
	}
	return p.input[start:p.pos]
}

func (p *typeParser) parseStringLiteral() (string, error) {
	quote := p.input[p.pos]
	start := p.pos
	p.pos++
	for p.pos < len(p.input) {
		c := p.input[p.pos]
		if c == '\\' {
			p.pos += 2
			continue
		}
		p.pos++
		if c == quote {
			raw := p.input[start:p.pos]
			value, err := strconv.Unquote(raw)
			if err != nil {
				return "", err
			}
			return value, nil
		}
	}
	return "", fmt.Errorf("unterminated string literal")
}

func isNumberStart(c byte) bool {
	return c == '-' || c == '+' || (c >= '0' && c <= '9')
}

func (p *typeParser) parseNumberLiteral() (any, error) {
	p.skipSpace()
	start := p.pos
	if p.pos < len(p.input) && (p.input[p.pos] == '-' || p.input[p.pos] == '+') {
		p.pos++
	}
	if p.pos+1 < len(p.input) && p.input[p.pos] == '0' && (p.input[p.pos+1] == 'x' || p.input[p.pos+1] == 'X') {
		p.pos += 2
		for p.pos < len(p.input) {
			c := p.input[p.pos]
			if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
				p.pos++
				continue
			}
			break
		}
		text := p.input[start:p.pos]
		value, err := strconv.ParseInt(text, 0, 64)
		if err != nil {
			return nil, err
		}
		return value, nil
	}
	dot := false
	for p.pos < len(p.input) {
		c := p.input[p.pos]
		if c >= '0' && c <= '9' {
			p.pos++
			continue
		}
		if c == '.' && !dot {
			dot = true
			p.pos++
			continue
		}
		break
	}
	text := p.input[start:p.pos]
	if dot {
		value, err := strconv.ParseFloat(text, 64)
		if err != nil {
			return nil, err
		}
		return value, nil
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (p *typeParser) skipSpace() {
	for p.pos < len(p.input) && (p.input[p.pos] == ' ' || p.input[p.pos] == '\t') {
		p.pos++
	}
}

func (p *typeParser) consume(text string) bool {
	if strings.HasPrefix(p.input[p.pos:], text) {
		p.pos += len(text)
		return true
	}
	return false
}

// parseTypeListPrefix parses comma-separated types and stops at trailing prose.
func parseTypeListPrefix(input string, resolve TypeResolver) ([]types.Type, int, error) {
	var out []types.Type
	offset := 0
	for {
		rest := input[offset:]
		typ, end, err := parseTypePrefix(rest, resolve)
		if err != nil {
			return nil, offset, err
		}
		out = append(out, typ)
		offset += end
		rest = strings.TrimSpace(input[offset:])
		if !strings.HasPrefix(rest, ",") {
			break
		}
		offset = len(input) - len(rest) + 1
	}
	return out, offset, nil
}

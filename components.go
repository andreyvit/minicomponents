package minicomponents

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	startRe         = regexp.MustCompile(`(?i)<(c-[a-z0-9-]+)`)
	whitespace      = " \t\n"
	endOpenRe       = regexp.MustCompile(`^/?>`)
	endBrokenOpenRe = regexp.MustCompile(`/?>`)

	attrStartRe             = regexp.MustCompile(`(?i)^([a-z0-9-]+)([=\s/>])`)
	attrQuotedValueRe       = regexp.MustCompile(`(?i)^"([^"]*)"`)
	attrSingleQuotedValueRe = regexp.MustCompile(`(?i)^'([^']*)'`)
	attrNakedValueRe        = regexp.MustCompile(`(?i)^[^\s/<>"']+`)
	attrGoValueRe           = regexp.MustCompile(`(?i)^\{\{(.+?)\}\}`)
	brokenAttrEndRe         = regexp.MustCompile(`(?i)(\s|/?>)`)
)

type RenderMethod int

const (
	RenderMethodNone = RenderMethod(iota)
	RenderMethodTemplate
	RenderMethodFunc
	RenderMethodFuncThenTemplate
	renderMethodSlot
)

type ComponentDef struct {
	RenderMethod RenderMethod
	FuncName     string
	TemplateName string
	SlotName     string
	HasSlots     bool
}

func (c *ComponentDef) funcName(compName string) string {
	if c.FuncName != "" {
		return c.FuncName
	}
	if c.RenderMethod == RenderMethodFunc {
		return strings.ReplaceAll(compName, "-", "_")
	} else {
		return compName
	}
}

func (c *ComponentDef) templName(compName string) string {
	if c.TemplateName != "" {
		return c.TemplateName
	}
	return compName
}

type Component struct {
	Name string
	Body string
	Args []Arg
}

type Arg struct {
	Name  string
	Value string
}

type ParseErr struct {
	Code string
	Pos  int
	Line int
	Msg  string
}

func (e *ParseErr) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

func errf(orig, templ string, format string, args ...any) *ParseErr {
	pos := len(orig) - len(templ)
	line := 1 + strings.Count(orig[:pos], "\n")
	return &ParseErr{
		Code: orig,
		Pos:  pos,
		Line: line,
		Msg:  fmt.Sprintf(format, args...),
	}
}

type rewriter struct {
	comps    map[string]*ComponentDef
	trailers strings.Builder
	err      error
}

func Rewrite(templ string, baseName string, comps map[string]*ComponentDef) (string, error) {
	r := rewriter{
		comps: comps,
	}
	var output strings.Builder
	r.rewrite(&output, templ, baseName)
	output.WriteString(r.trailers.String())
	return output.String(), r.err
}

func (r *rewriter) fail(err error) {
	if r.err == nil {
		r.err = err
	}
}

func (r *rewriter) rewrite(output *strings.Builder, templ string, baseName string) {
	templ = strings.ReplaceAll(templ, "$@", "$.Args.")
	var retErr error
	orig := templ
	nextSlotTemplateIndex := 1
	for {
		// log.Printf("parsing %q", templ)
		m := startRe.FindStringSubmatchIndex(templ)
		if m == nil {
			output.WriteString(templ)
			break
		}
		output.WriteString(templ[:m[0]])
		name := templ[m[2]:m[3]]
		// log.Printf("open %q", name)

		var precededBySpace bool
		templ, precededBySpace = skipSpace(templ[m[1]:])

		c := &Component{
			Name: name,
		}
		var tagErr *ParseErr

		var comp *ComponentDef
		if slot, ok := strings.CutPrefix(c.Name, "c-slot-"); ok {
			comp = &ComponentDef{
				RenderMethod: renderMethodSlot,
				SlotName:     slot,
			}
		} else {
			comp = r.comps[c.Name]
		}
		if tagErr == nil && comp == nil {
			tagErr = errf(orig, templ, fmt.Sprintf("unknown component <%s>", c.Name))
		}

		isClosed := false
		endRe := endOpenRe
		for {
			// log.Printf("parsing attrs at %q, precededBySpace=%v", templ, precededBySpace)

			if m := endRe.FindStringIndex(templ); m != nil {
				isClosed = (templ[m[0]:m[1]] == "/>")
				templ = templ[m[1]:]
				break
			}

			if m = attrStartRe.FindStringSubmatchIndex(templ); precededBySpace && m != nil {
				attrName := templ[m[2]:m[3]]
				attrSep := templ[m[4]:m[5]]
				var value, rawValue string
				var valueOK bool = true
				// log.Printf("attrName=%q attrSep=%q", attrName, attrSep)
				if attrSep == "=" {
					templ = trimSpace(templ[m[1]:])
					if m := attrQuotedValueRe.FindStringSubmatchIndex(templ); m != nil {
						rawValue = templ[m[2]:m[3]]
						value, valueOK = rewriteInterpolatedStringAsExpr(rawValue)
						templ = templ[m[1]:]
					} else if m := attrSingleQuotedValueRe.FindStringSubmatchIndex(templ); m != nil {
						rawValue = templ[m[2]:m[3]]
						value, valueOK = rewriteInterpolatedStringAsExpr(rawValue)
						templ = templ[m[1]:]
					} else if m := attrGoValueRe.FindStringSubmatchIndex(templ); m != nil {
						rawValue = templ[m[2]:m[3]]
						value = "(" + rawValue + ")"
						templ = templ[m[1]:]
					} else if m := attrNakedValueRe.FindStringSubmatchIndex(templ); m != nil {
						rawValue = templ[m[0]:m[1]]
						value = strconv.Quote(rawValue)
						templ = templ[m[1]:]
					} else if m := brokenAttrEndRe.FindStringIndex(templ); m != nil {
						if tagErr == nil {
							tagErr = errf(orig, templ, "missing value for attr %s", attrName)
						}
						templ = templ[m[0]:]
						value = "nil"
					} else {
						if tagErr == nil {
							tagErr = errf(orig, templ, "invalid syntax of attr %s", attrName)
						}
						endRe = endBrokenOpenRe
						break
					}
					if !valueOK {
						// TODO: we could build a template and then eval it
						if tagErr == nil {
							tagErr = errf(orig, templ, "cannot represent attr %q value %s as a single call", attrName, rawValue)
						}
					}
				} else {
					templ = templ[m[4]:]
					value = "true"
				}
				// log.Printf("attr %q = %v", attrName, value)
				templ, precededBySpace = skipSpace(templ)
				c.Args = append(c.Args, Arg{attrName, value})
			} else {
				if endRe == endBrokenOpenRe {
					if tagErr == nil {
						tagErr = errf(orig, templ, "missing end of tag")
					}
					break
				} else {
					if tagErr == nil {
						tagErr = errf(orig, templ, "invalid syntax or missing end of tag")
					}
					endRe = endBrokenOpenRe
				}
			}
		}

		if !isClosed {
			closing := "</" + name + ">"

			if before, after, found := strings.Cut(templ, closing); found {
				c.Body = before
				templ = after
			} else {
				if tagErr == nil {
					tagErr = errf(orig, templ, "missing %s", closing)
				}
			}
		}

		hasSlots := (comp != nil && comp.HasSlots)
		var usesSlotTemplate bool
		var bodyExpr string
		if hasSlots {
			usesSlotTemplate = true
		} else if c.Body != "" {
			var ok bool
			bodyExpr, ok = rewriteInterpolatedStringAsExpr(strings.TrimSpace(c.Body))
			// log.Printf("<%s> ok=%v body: %q bodyExpr: %q", c.Name, ok, c.Body, bodyExpr)
			ok = false // quick fix for escaping problems
			if !ok {
				usesSlotTemplate = true
			}
		}

		var slotTemplateName string
		if usesSlotTemplate {
			slotTemplateName = baseName + "___" + c.Name + "__body__" + strconv.Itoa(nextSlotTemplateIndex)
			nextSlotTemplateIndex++

			var subout strings.Builder
			fmt.Fprintf(&subout, "{{define %q}}{{with .Data}}", slotTemplateName)
			r.rewrite(&subout, c.Body, slotTemplateName)
			subout.WriteString("{{end}}{{end}}")
			r.trailers.WriteString(subout.String())

			if hasSlots {
				c.Args = append(c.Args, Arg{"bodyTemplate", strconv.Quote(slotTemplateName)})
			} else {
				c.Args = append(c.Args, Arg{"body", fmt.Sprintf("(eval %q ($.Bind .))", slotTemplateName)})
			}
		} else if c.Body != "" {
			c.Args = append(c.Args, Arg{"body", bodyExpr})
		}

		if tagErr != nil {
			if retErr == nil {
				retErr = fmt.Errorf("%s: %w", name, tagErr)
			}
			output.WriteString("{{error ")
			output.WriteString(strconv.Quote(tagErr.Msg))
			output.WriteString("}}")
		} else {
			if comp.RenderMethod == renderMethodSlot {
				fmt.Fprintf(output, "{{eval $.Args.%sTemplate", comp.SlotName)
				writeBindArgs(output, c.Args, "$.Data")
				output.WriteString("}}")
			} else {
				switch comp.RenderMethod {
				case RenderMethodTemplate:
					output.WriteString("{{template ")
					output.WriteString(strconv.Quote(comp.templName(c.Name)))
				case RenderMethodFunc:
					output.WriteString("{{")
					output.WriteString(comp.funcName(c.Name))
				case RenderMethodFuncThenTemplate:
					output.WriteString("{{template ")
					output.WriteString(strconv.Quote(comp.templName(c.Name)))
					output.WriteString(" ($.Bind (")
					output.WriteString(comp.funcName(c.Name))
				default:
					panic(fmt.Errorf("unsupported render method %v", comp.RenderMethod))
				}
				var dataExpr string
				if usesSlotTemplate {
					dataExpr = "."
				} else {
					dataExpr = "nil"
				}
				writeBindArgs(output, c.Args, dataExpr)
				switch comp.RenderMethod {
				case RenderMethodTemplate, RenderMethodFunc:
					output.WriteString("}}")
				case RenderMethodFuncThenTemplate:
					output.WriteString("))}}")
				default:
					panic(fmt.Errorf("unsupported render method %v", comp.RenderMethod))
				}
			}
		}
	}
}

func writeBindArgs(wr *strings.Builder, args []Arg, dataExpr string) {
	dataArgIdx := findArg(args, "data")
	if dataArgIdx >= 0 {
		dataExpr = args[dataArgIdx].Value
	}

	wr.WriteString(" ($.Bind ")
	wr.WriteString(dataExpr)
	for i, arg := range args {
		if i == dataArgIdx {
			continue
		}
		wr.WriteString(" ")
		wr.WriteString(strconv.Quote(arg.Name))
		wr.WriteString(" ")
		wr.WriteString(arg.Value)
	}
	wr.WriteString(")")
}

func WrapTemplate(code string, prefix, suffix string) string {
	var defines string
	if i := strings.Index(code, "{{define"); i >= 0 {
		code, defines = code[:i], code[i:]
	}
	return prefix + code + suffix + defines
}

func ScanTemplate(code string) *ComponentDef {
	return &ComponentDef{
		RenderMethod: RenderMethodTemplate,
		HasSlots:     strings.Contains(code, "<c-slot-"),
	}
}

func rewriteInterpolatedStringAsExpr(str string) (string, bool) {
	if strings.Contains(str, "<c-") {
		return "", false
	}
	if !strings.Contains(str, "{{") {
		return strconv.Quote(str), true
	}

	var buf strings.Builder
	buf.WriteString("(print")

	for {
		prefix, remainder, found := strings.Cut(str, "{{")
		if !found {
			break
		}

		remainder, trimmingPrefix := strings.CutPrefix(remainder, "- ")

		if prefix != "" {
			if trimmingPrefix {
				prefix = strings.TrimRightFunc(prefix, unicode.IsSpace)
			}
			buf.WriteByte(' ')
			buf.WriteString(strconv.Quote(prefix))
		}

		expr, suffix, found := strings.Cut(remainder, "}}")
		if !found {
			str = remainder
			break
		}
		expr, trimmingSuffix := strings.CutSuffix(expr, "-")
		expr = strings.TrimSpace(expr)

		if !isComment(expr) {
			if isUnconcatenatableExpr(expr) {
				return "", false
			}
			buf.WriteByte(' ')
			buf.WriteString(parenthesizeIfNecessary(expr))
		}

		if trimmingSuffix {
			suffix = strings.TrimLeftFunc(suffix, unicode.IsSpace)
		}
		str = suffix
	}
	if str != "" {
		buf.WriteByte(' ')
		buf.WriteString(strconv.Quote(str))
	}

	buf.WriteString(")")
	return buf.String(), true
}

func findArg(args []Arg, name string) int {
	for i, arg := range args {
		if arg.Name == name {
			return i
		}
	}
	return -1
}

func isComment(expr string) bool {
	return strings.HasPrefix(expr, "/*")
}

func isUnconcatenatableExpr(expr string) bool {
	if strings.Contains(expr, ":=") {
		return true
	}
	firstWord, _, _ := strings.Cut(strings.TrimSpace(expr), " ")
	return actions[firstWord]
}

var actions = map[string]bool{
	"if":       true,
	"else":     true,
	"range":    true,
	"break":    true,
	"continue": true,
	"with":     true,
	"end":      true,
	"template": true,
	"block":    true,
}

var safeSimpleExprRe = regexp.MustCompile(`^[.\w]+$`)

func parenthesizeIfNecessary(expr string) string {
	if safeSimpleExprRe.MatchString(expr) {
		return expr
	} else {
		return "(" + expr + ")"
	}
}

func skipSpace(s string) (string, bool) {
	r := trimSpace(s)
	return r, len(r) != len(s)
}

func trimSpace(s string) string {
	return strings.TrimLeft(s, whitespace)
}

func Args(args ...any) map[string]any {
	n := len(args)
	if n%2 != 0 {
		panic(fmt.Errorf("odd number of arguments %d: %v", n, args))
	}
	m := make(map[string]any, n/2)
	for i := 0; i < n; i += 2 {
		key, value := args[i], args[i+1]
		if keyStr, ok := key.(string); ok {
			m[keyStr] = value
		} else {
			panic(fmt.Errorf("argument %d must be a string, got %T: %v", i, key, key))
		}
	}
	if len(m) == 0 {
		m["__dummy"] = true
	}
	return m
}

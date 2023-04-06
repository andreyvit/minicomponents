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
	attrQuotedValueRe       = regexp.MustCompile(`(?i)^"([^"]+)"`)
	attrSingleQuotedValueRe = regexp.MustCompile(`(?i)^'([^']+)'`)
	attrNakedValueRe        = regexp.MustCompile(`(?i)^[^\s/<>"']+`)
	attrGoValueRe           = regexp.MustCompile(`(?i)^\{\{(.+?)\}\}`)
	brokenAttrEndRe         = regexp.MustCompile(`(?i)(\s|/?>)`)
)

type RenderMethod int

const (
	RenderMethodNone = RenderMethod(iota)
	RenderMethodTemplate
	RenderMethodFunc
)

type ComponentDef struct {
	RenderMethod RenderMethod
	FuncName     string
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

func Rewrite(templ string, baseName string, comps map[string]*ComponentDef) (string, error) {
	var wr strings.Builder
	var retErr error
	orig := templ
	for {
		// log.Printf("parsing %q", templ)
		m := startRe.FindStringSubmatchIndex(templ)
		if m == nil {
			wr.WriteString(templ)
			break
		}
		wr.WriteString(templ[:m[0]])
		name := templ[m[2]:m[3]]
		// log.Printf("open %q", name)

		var precededBySpace bool
		templ, precededBySpace = skipSpace(templ[m[1]:])

		c := &Component{
			Name: name,
		}
		var tagErr *ParseErr

		comp := comps[c.Name]
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
				var value string
				// log.Printf("attrName=%q attrSep=%q", attrName, attrSep)
				if attrSep == "=" {
					templ = trimSpace(templ[m[1]:])
					if m := attrQuotedValueRe.FindStringSubmatchIndex(templ); m != nil {
						value = rewriteInterpolatedStringAsExpr(templ[m[2]:m[3]])
						templ = templ[m[1]:]
					} else if m := attrSingleQuotedValueRe.FindStringSubmatchIndex(templ); m != nil {
						value = rewriteInterpolatedStringAsExpr(templ[m[2]:m[3]])
						templ = templ[m[1]:]
					} else if m := attrGoValueRe.FindStringSubmatchIndex(templ); m != nil {
						value = "(" + templ[m[2]:m[3]] + ")"
						templ = templ[m[1]:]
					} else if m := attrNakedValueRe.FindStringSubmatchIndex(templ); m != nil {
						value = strconv.Quote(templ[m[0]:m[1]])
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

		if strings.TrimSpace(c.Body) != "" {
			c.Args = append(c.Args, Arg{"body", rewriteInterpolatedStringAsExpr(strings.TrimSpace(c.Body))})
			// wr.WriteString("{{")
		}

		if tagErr != nil {
			if retErr == nil {
				retErr = fmt.Errorf("%s: %w", name, tagErr)
			}
			wr.WriteString("{{error ")
			wr.WriteString(strconv.Quote(tagErr.Msg))
			wr.WriteString("}}")
		} else {
			if comp.RenderMethod == RenderMethodTemplate {
				wr.WriteString("{{template ")
				wr.WriteString(strconv.Quote(c.Name))
			} else {
				wr.WriteString("{{")
				wr.WriteString(comp.FuncName)
			}
			wr.WriteString(" ($.Bind nil")
			for _, arg := range c.Args {
				wr.WriteString(" ")
				wr.WriteString(strconv.Quote(arg.Name))
				wr.WriteString(" ")
				wr.WriteString(arg.Value)
			}
			wr.WriteString(")}}")
		}
	}
	return wr.String(), retErr
}

func rewriteInterpolatedStringAsExpr(str string) string {
	if !strings.Contains(str, "{{") {
		return strconv.Quote(str)
	}

	var buf strings.Builder
	buf.WriteString("(concat")

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

		buf.WriteByte(' ')
		buf.WriteString(parenthesizeIfNecessary(strings.TrimSpace(expr)))

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
	return buf.String()
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

package minicomponents

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"testing"
)

func TestRewrite(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expCode   string
		expOutput string
	}{
		{"", ``, ``, ``},
		{"", `foo`, `foo`, `foo`},

		{"", `foo <c-test/> bar`, `foo {{template "c-test" ($.Bind nil)}} bar`, `foo TEST bar`},
		{"", `foo <c-test /> bar`, `foo {{template "c-test" ($.Bind nil)}} bar`, `foo TEST bar`},
		{"", `foo <c-test></c-test> bar`, `foo {{template "c-test" ($.Bind nil)}} bar`, `foo TEST bar`},

		{"", `foo <c-test abc/> bar`, `foo {{template "c-test" ($.Bind nil "abc" true)}} bar`, `foo TEST bar`},
		{"", `foo <c-test   abc  /> bar`, `foo {{template "c-test" ($.Bind nil "abc" true)}} bar`, `foo TEST bar`},
		{"", `foo <c-test abc=xyz /> bar`, `foo {{template "c-test" ($.Bind nil "abc" "xyz")}} bar`, `foo TEST bar`},
		{"", `foo <c-test abc="xyz" /> bar`, `foo {{template "c-test" ($.Bind nil "abc" "xyz")}} bar`, `foo TEST bar`},
		{"", `foo <c-test abc='xyz' /> bar`, `foo {{template "c-test" ($.Bind nil "abc" "xyz")}} bar`, `foo TEST bar`},
		{"", `foo <c-test abc="xyz uvw" /> bar`, `foo {{template "c-test" ($.Bind nil "abc" "xyz uvw")}} bar`, `foo TEST bar`},
		{"", `foo <c-test abc='xyz uvw' /> bar`, `foo {{template "c-test" ($.Bind nil "abc" "xyz uvw")}} bar`, `foo TEST bar`},
		{"", `foo <c-test abc={{xyz}} /> bar`, `foo {{template "c-test" ($.Bind nil "abc" (xyz))}} bar`, `foo TEST bar`},
		{"", `foo <c-test abc={{xyz}} def g=4 jk="uvw" /> bar`, `foo {{template "c-test" ($.Bind nil "abc" (xyz) "def" true "g" "4" "jk" "uvw")}} bar`, `foo TEST bar`},
		{"", `foo <c-test data={{xyz}} /> bar`, `foo {{template "c-test" ($.Bind (xyz))}} bar`, `foo TEST bar`},

		{"", `foo <c-test> bar`, `foo {{error "missing </c-test>"}} bar`, `foo ERROR bar`},
		{"", `foo <c-test abc= > bar`, `foo {{error "missing value for attr abc"}} bar`, `foo ERROR bar`},
		{"", `foo <c-test abc=> bar`, `foo {{error "missing value for attr abc"}} bar`, `foo ERROR bar`},
		{"", `foo <c-test ab$$> bar`, `foo {{error "invalid syntax or missing end of tag"}} bar`, `foo ERROR bar`},
		{"", `foo <c-test ab<c-boz/> bar`, `foo {{error "invalid syntax or missing end of tag"}} bar`, `foo ERROR bar`},

		{"", `foo <c-test /> bar <c-another/> boz`, `foo {{template "c-test" ($.Bind nil)}} bar {{template "c-another" ($.Bind nil)}} boz`, `foo TEST bar ANOTHER boz`},

		{"", `foo <c-foo abc="42" test /> bar`, `foo {{render_foo ($.Bind nil "abc" "42" "test" true)}} bar`, `foo FOO bar`},

		{"", `foo <c-xxx /> bar`, `foo {{error "unknown component <c-xxx>"}} bar`, `foo ERROR bar`},

		{"", `foo <c-test>bar</c-test> boz`, `foo {{template "c-test" ($.Bind nil "body" "bar")}} boz`, `foo TEST boz`},

		{"", `foo <c-test>ba{{.test}}r</c-test> boz`, `foo {{template "c-test" ($.Bind nil "body" (print "ba" .test "r"))}} boz`, `foo TEST boz`},
		{"", `foo <c-test>ba{{.test.foo}}r</c-test> boz`, `foo {{template "c-test" ($.Bind nil "body" (print "ba" .test.foo "r"))}} boz`, `foo TEST boz`},
		{"", `foo <c-test>ba{{ .test }}r</c-test> boz`, `foo {{template "c-test" ($.Bind nil "body" (print "ba" .test "r"))}} boz`, `foo TEST boz`},
		{"", `foo <c-test>ba{{.test | brackets}}r</c-test> boz`, `foo {{template "c-test" ($.Bind nil "body" (print "ba" (.test | brackets) "r"))}} boz`, `foo TEST boz`},
		{"", `foo <c-test>ba {{.test}} r</c-test> boz`, `foo {{template "c-test" ($.Bind nil "body" (print "ba " .test " r"))}} boz`, `foo TEST boz`},
		{"", `foo <c-test>ba {{- .test}} r</c-test> boz`, `foo {{template "c-test" ($.Bind nil "body" (print "ba" .test " r"))}} boz`, `foo TEST boz`},
		{"", `foo <c-test>ba {{- .test -}} r</c-test> boz`, `foo {{template "c-test" ($.Bind nil "body" (print "ba" .test "r"))}} boz`, `foo TEST boz`},
		{"", `foo <c-test abc="xy{{.test}}z" /> bar`, `foo {{template "c-test" ($.Bind nil "abc" (print "xy" .test "z"))}} bar`, `foo TEST bar`},
		{"", `foo <c-test abc='xy{{.test}}z' /> bar`, `foo {{template "c-test" ($.Bind nil "abc" (print "xy" .test "z"))}} bar`, `foo TEST bar`},

		{"eval'ed template body for unsuspecting component", `foo <c-button>{{if .Good}}green{{else}}red{{end}}</c-button> bar`, `foo {{template "c-button" ($.Bind . "body" (eval "mypage___c-button__body__1" ($.Bind .)))}} bar{{define "mypage___c-button__body__1"}}{{with .Data}}{{if .Good}}green{{else}}red{{end}}{{end}}{{end}}`, `foo <button>green</button> bar`},

		{"slot component body", `foo <c-slot-body /> bar <c-slot-another data="hello" /> boz`, `foo {{eval $.Args.bodyTemplate ($.Bind $.Data)}} bar {{eval $.Args.anotherTemplate ($.Bind "hello")}} boz`, `foo TEST bar <button>hello</button> boz`},
		{"slot component body with extra arg", `foo <c-slot-body answer={{42}} /> bar`, `foo {{eval $.Args.bodyTemplate ($.Bind $.Data "answer" (42))}} bar`, `foo TEST bar`},
		{"slot component body with data override and arg", `foo <c-slot-body data="hello" answer={{42}} /> bar`, `foo {{eval $.Args.bodyTemplate ($.Bind "hello" "answer" (42))}} bar`, `foo TEST bar`},

		{"slot component", `foo <c-box first="hello" second="world">“{{.}}”</c-box> bar`, `foo {{template "c-box" ($.Bind . "first" "hello" "second" "world" "bodyTemplate" "mypage___c-box__body__1")}} bar{{define "mypage___c-box__body__1"}}{{with .Data}}“{{.}}”{{end}}{{end}}`, `foo <box>“hello”|“world”</box> bar`},
		{"two slot component calls", `foo <c-simple>A</c-simple> bar <c-simple>B</c-simple> boz`, `foo {{template "c-simple" ($.Bind . "bodyTemplate" "mypage___c-simple__body__1")}} bar {{template "c-simple" ($.Bind . "bodyTemplate" "mypage___c-simple__body__2")}} boz{{define "mypage___c-simple__body__1"}}{{with .Data}}A{{end}}{{end}}{{define "mypage___c-simple__body__2"}}{{with .Data}}B{{end}}{{end}}`, `foo <simple>A</simple> bar <simple>B</simple> boz`},
	}
	comps := map[string]*ComponentDef{
		"c-test":    {RenderMethod: RenderMethodTemplate},
		"c-another": {RenderMethod: RenderMethodTemplate},
		"c-foo":     {RenderMethod: RenderMethodFunc, ImplName: "render_foo"},
		"c-button":  {RenderMethod: RenderMethodTemplate},
		"c-box":     {RenderMethod: RenderMethodTemplate, HasSlots: true},
		"c-simple":  {RenderMethod: RenderMethodTemplate, HasSlots: true},
	}
	for _, tt := range tests {
		if tt.name == "" {
			tt.name = tt.input
		}
		t.Run(tt.name, func(t *testing.T) {
			actual, _ := Rewrite(tt.input, "mypage", comps)
			if actual != tt.expCode {
				t.Errorf("** Rewrite(%s) returned:\n\t%s\nexpected:\n\t%s", tt.input, actual, tt.expCode)
			} else {
				t.Logf("✓ Rewrite(%s) == %s", tt.input, actual)
			}

			root := template.New("")
			root.Funcs(template.FuncMap{
				"render_foo": func(v any) template.HTML {
					return "FOO"
				},
				"xyz": func() string {
					return "zyx"
				},
				"brackets": func(v any) string {
					return fmt.Sprintf("[%v]", v)
				},
				"eval": func(templateName string, data any) (template.HTML, error) {
					t.Logf("eval %q with %s", templateName, must(json.Marshal(data)))
					var buf strings.Builder
					err := root.ExecuteTemplate(&buf, templateName, data)
					if err != nil {
						return "", err
					}
					return template.HTML(buf.String()), nil
				},
				"error": func(message string) string {
					return "ERROR"
				},
			})
			page := must(root.New("mypage").Parse(WrapTemplate(tt.expCode, "{{with .Data}}", "{{end}}")))
			must(root.New("c-test").Parse(`TEST`))
			must(root.New("c-another").Parse(`ANOTHER`))
			must(root.New("c-button").Parse(`<button>{{.Args.body}}</button>`))
			must(root.New("c-simple").Parse(`<simple>{{eval .Args.bodyTemplate ($.Bind $.Data)}}</simple>`))
			must(root.New("c-box").Parse(`<box>{{eval .Args.bodyTemplate ($.Bind .Args.first)}}|{{eval .Args.bodyTemplate ($.Bind .Args.second)}}</box>`))
			// for testing component bodies
			must(root.New("button___body").Parse(`{{with .Data}}<button>{{.}}</button>{{end}}`))

			var out strings.Builder
			err := page.Execute(&out, &renderData{
				Data: map[string]any{
					"Foo":  true,
					"Good": true,
				},
				Args: map[string]any{
					// for testing component bodies
					"bodyTemplate":    "c-test",
					"anotherTemplate": "button___body",
				},
			})
			if err != nil {
				t.Errorf("** Execute failed: %v", err)
			} else if output := out.String(); output != tt.expOutput {
				t.Errorf("** Execute returned:\n\t%s\nexpected:\n\t%s", output, tt.expOutput)
			}
		})
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

type renderData struct {
	Data any
	Args map[string]any
}

func (d *renderData) Bind(value any, args ...any) *renderData {
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
	return &renderData{
		Data: value,
		Args: m,
	}
}

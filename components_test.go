package minicomponents

import (
	"testing"
)

func TestRewrite(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{``, ``},
		{`foo`, `foo`},

		{`foo <c-test/> bar`, `foo {{template "c-test" ($.Bind nil)}} bar`},
		{`foo <c-test /> bar`, `foo {{template "c-test" ($.Bind nil)}} bar`},
		{`foo <c-test></c-test> bar`, `foo {{template "c-test" ($.Bind nil)}} bar`},

		{`foo <c-test abc/> bar`, `foo {{template "c-test" ($.Bind nil "abc" true)}} bar`},
		{`foo <c-test   abc  /> bar`, `foo {{template "c-test" ($.Bind nil "abc" true)}} bar`},
		{`foo <c-test abc=xyz /> bar`, `foo {{template "c-test" ($.Bind nil "abc" "xyz")}} bar`},
		{`foo <c-test abc="xyz" /> bar`, `foo {{template "c-test" ($.Bind nil "abc" "xyz")}} bar`},
		{`foo <c-test abc='xyz' /> bar`, `foo {{template "c-test" ($.Bind nil "abc" "xyz")}} bar`},
		{`foo <c-test abc="xyz uvw" /> bar`, `foo {{template "c-test" ($.Bind nil "abc" "xyz uvw")}} bar`},
		{`foo <c-test abc='xyz uvw' /> bar`, `foo {{template "c-test" ($.Bind nil "abc" "xyz uvw")}} bar`},
		{`foo <c-test abc={{xyz}} /> bar`, `foo {{template "c-test" ($.Bind nil "abc" (xyz))}} bar`},
		{`foo <c-test abc={{xyz}} def g=4 jk="uvw" /> bar`, `foo {{template "c-test" ($.Bind nil "abc" (xyz) "def" true "g" "4" "jk" "uvw")}} bar`},

		{`foo <c-test> bar`, `foo {{error "missing </c-test>"}} bar`},
		{`foo <c-test abc= > bar`, `foo {{error "missing value for attr abc"}} bar`},
		{`foo <c-test abc=> bar`, `foo {{error "missing value for attr abc"}} bar`},
		{`foo <c-test ab$$> bar`, `foo {{error "invalid syntax or missing end of tag"}} bar`},
		{`foo <c-test ab<c-boz/> bar`, `foo {{error "invalid syntax or missing end of tag"}} bar`},

		{`foo <c-test /> bar <c-another/> boz`, `foo {{template "c-test" ($.Bind nil)}} bar {{template "c-another" ($.Bind nil)}} boz`},

		{`foo <c-foo abc="42" test /> bar`, `foo {{render_foo ($.Bind nil "abc" "42" "test" true)}} bar`},

		{`foo <c-xxx /> bar`, `foo {{error "unknown component <c-xxx>"}} bar`},

		{`foo <c-test>bar</c-test> boz`, `foo {{template "c-test" ($.Bind nil "body" "bar")}} boz`},

		{`foo <c-test>ba{{.test}}r</c-test> boz`, `foo {{template "c-test" ($.Bind nil "body" (print "ba" .test "r"))}} boz`},
		{`foo <c-test>ba{{.test.foo}}r</c-test> boz`, `foo {{template "c-test" ($.Bind nil "body" (print "ba" .test.foo "r"))}} boz`},
		{`foo <c-test>ba{{ .test }}r</c-test> boz`, `foo {{template "c-test" ($.Bind nil "body" (print "ba" .test "r"))}} boz`},
		{`foo <c-test>ba{{.test | foo}}r</c-test> boz`, `foo {{template "c-test" ($.Bind nil "body" (print "ba" (.test | foo) "r"))}} boz`},
		{`foo <c-test>ba {{.test}} r</c-test> boz`, `foo {{template "c-test" ($.Bind nil "body" (print "ba " .test " r"))}} boz`},
		{`foo <c-test>ba {{- .test}} r</c-test> boz`, `foo {{template "c-test" ($.Bind nil "body" (print "ba" .test " r"))}} boz`},
		{`foo <c-test>ba {{- .test -}} r</c-test> boz`, `foo {{template "c-test" ($.Bind nil "body" (print "ba" .test "r"))}} boz`},
		{`foo <c-test abc="xy{{.test}}z" /> bar`, `foo {{template "c-test" ($.Bind nil "abc" (print "xy" .test "z"))}} bar`},
		{`foo <c-test abc='xy{{.test}}z' /> bar`, `foo {{template "c-test" ($.Bind nil "abc" (print "xy" .test "z"))}} bar`},
	}
	comps := map[string]*ComponentDef{
		"c-test":    {RenderMethod: RenderMethodTemplate},
		"c-another": {RenderMethod: RenderMethodTemplate},
		"c-foo":     {RenderMethod: RenderMethodFunc, FuncName: "render_foo"},
	}
	for _, tt := range tests {
		actual, _ := Rewrite(tt.input, "mypage", comps)
		if actual != tt.expected {
			t.Errorf("** Rewrite(%s) returned:\n\t%s\nexpected:\n\t%s", tt.input, actual, tt.expected)
		} else {
			t.Logf("✓ Rewrite(%s) == %s", tt.input, actual)
		}
	}
}

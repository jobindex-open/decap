package recap

import (
	"strings"
	"testing"

	"github.com/go-shiori/dom"
)

func TestPreprocess(t *testing.T) {
	doc, _ := dom.Parse(strings.NewReader(`<aside id="a"><h2>Summary</h2></aside><aside id="b">ad</aside>`))
	preprocessDocument(doc)
	if n := dom.QuerySelector(doc, "#a"); n == nil || n.Data != "div" {
		t.Error("aside with heading should become div")
	}
	if n := dom.QuerySelector(doc, "#b"); n == nil || n.Data != "aside" {
		t.Error("aside without heading should stay aside")
	}
}

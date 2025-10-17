package readability

import (
	"regexp"
	"strings"

	"github.com/go-shiori/dom"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var reWS = regexp.MustCompile(`\s+`)

// genericBadgeLabels enumerates short, generic badge texts that should not
// be promoted (e.g. "job", "new") when trimming headers.
var genericBadgeLabels = map[string]struct{}{
	"job": {}, "jobs": {}, "stilling": {}, "stillinger": {}, "annonce": {},
	"annoncer": {}, "ad": {}, "ads": {}, "advertisement": {}, "ny": {}, "new": {},
}

// isGenericBadgeLabel reports whether a badge text is too generic to keep.
func isGenericBadgeLabel(raw string) bool {
	raw = strings.ToLower(strings.TrimSpace(raw))
	_, ok := genericBadgeLabels[raw]
	return ok
}

// addClass idempotently appends a class to an element node.
func addClass(n *html.Node, cls string) {
	if n.Type != html.ElementNode {
		return
	}
	for i := range n.Attr {
		if n.Attr[i].Key == "class" {
			parts := strings.Fields(n.Attr[i].Val)
			for _, p := range parts {
				if p == cls {
					return
				}
			}
			n.Attr[i].Val += " " + cls
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: "class", Val: cls})
}

// getAttr returns the first attribute value for key or "".
func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// setAttr sets or adds an attribute on a node.
func setAttr(n *html.Node, key, val string) {
	for i := range n.Attr {
		if n.Attr[i].Key == key {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}

// cloneNodeDeep performs a deep copy of a DOM subtree.
func cloneNodeDeep(n *html.Node) *html.Node {
	if n == nil {
		return nil
	}
	c := &html.Node{
		Type:     n.Type,
		DataAtom: n.DataAtom,
		Data:     n.Data,
		Attr:     append([]html.Attribute(nil), n.Attr...),
	}
	for ch := range n.ChildNodes() {
		cc := cloneNodeDeep(ch)
		c.AppendChild(cc)
	}
	return c
}

// removeNode safely detaches a node from its parent.
func removeNode(n *html.Node) {
	if n == nil || n.Parent == nil {
		return
	}
	n.Parent.RemoveChild(n)
}

// nodeContains checks whether ancestor contains node (inclusive).
func nodeContains(ancestor, node *html.Node) bool {
	if ancestor == nil || node == nil {
		return false
	}
	if ancestor == node {
		return true
	}
	for _, d := range dom.QuerySelectorAll(ancestor, "*") {
		if d == node {
			return true
		}
	}
	return false
}

// hasBlockDescendant detects if an inline wrapper wrongly wraps block content.
func hasBlockDescendant(n *html.Node) bool {
	if n == nil {
		return false
	}
	blockSel := "address,article,aside,blockquote,div,dl,fieldset,figcaption,figure,footer,form,h1,h2,h3,h4,h5,h6,header,hr,li,main,nav,ol,p,pre,section,table,ul,video"
	return len(dom.QuerySelectorAll(n, blockSel)) > 0
}

// findHeading returns the first heading element within a header region.
func findHeading(h *html.Node) *html.Node {
	return dom.QuerySelector(h, "h1,h2,h3")
}

// cloneUsefulBadge extracts a non-generic badge clone (if any) from a header.
func cloneUsefulBadge(h *html.Node) *html.Node {
	badge := dom.QuerySelector(h, ".badge, [class*='badge']")
	if badge == nil {
		return nil
	}
	raw := strings.TrimSpace(dom.TextContent(badge))
	if len(raw) < 3 || len(raw) > 40 {
		return nil
	}
	if isGenericBadgeLabel(raw) {
		return nil
	}
	return cloneNodeDeep(badge)
}

// findSummaryParagraph heuristically finds a descriptive summary paragraph
// near a heading suitable for inclusion in a trimmed header.
func findSummaryParagraph(h *html.Node) *html.Node {
	for _, p := range dom.QuerySelectorAll(h, "p") {
		txt := strings.TrimSpace(reWS.ReplaceAllString(dom.TextContent(p), " "))
		if l := len(txt); l < 40 || l > 600 {
			continue
		}
		bolds := dom.QuerySelectorAll(p, "b,strong")
		if len(bolds) > 6 {
			continue
		}
		if len(bolds) > 0 || strings.Count(txt, " ")+1 > 6 {
			return cloneNodeDeep(p)
		}
	}
	return nil
}

// collectHeavyMeta gathers nodes considered heavy / noisy to decide pruning.
func collectHeavyMeta(h, heading, summary *html.Node) (heavy []*html.Node) {
	metaSelectors := ".row,.col,[class*=grid],table,form,button,ul,ol,figure,picture,video,time"
	for _, el := range dom.QuerySelectorAll(h, metaSelectors) {
		if nodeContains(heading, el) {
			continue
		}
		if summary != nil && nodeContains(el, summary) {
			continue
		}
		heavy = append(heavy, el)
	}
	return
}

// shouldReplaceHeader decides whether to replace an original header with a
// trimmed variant based on shrink ratio and noise metrics.
func shouldReplaceHeader(originalText, newText string, heavyCount, mediaOrExtra int) bool {
	shrinkRatio := 1.0
	if len(originalText) > 0 && len(newText) > 0 {
		shrinkRatio = float64(len(newText)) / float64(len(originalText))
	}
	return heavyCount > 0 || mediaOrExtra > 8 || shrinkRatio < 0.85
}

// headerNormalization trims noisy article headers keeping salient elements.
func headerNormalization(doc *html.Node) {
	for _, h := range dom.QuerySelectorAll(doc, "article > header, body > header, main > header") {
		heading := findHeading(h)
		if heading == nil {
			continue
		}
		badge := cloneUsefulBadge(h)
		summary := findSummaryParagraph(h)
		heavy := collectHeavyMeta(h, heading, summary)

		wrapper := dom.CreateElement("div")
		setAttr(wrapper, "data-trimmed-header", "1")
		if badge != nil {
			wrapper.AppendChild(badge)
		}
		wrapper.AppendChild(cloneNodeDeep(heading))
		if summary != nil {
			wrapper.AppendChild(summary)
		}

		originalText := strings.TrimSpace(dom.TextContent(h))
		newText := strings.TrimSpace(dom.TextContent(wrapper))
		mediaOrExtra := len(dom.QuerySelectorAll(h, "img,a"))

		if shouldReplaceHeader(originalText, newText, len(heavy), mediaOrExtra) {
			h.Parent.InsertBefore(wrapper, h)
			removeNode(h)
			continue
		}
		for _, m := range heavy {
			removeNode(m)
		}
	}
}

// normalizeInlineWrappers converts inline elements that improperly wrap
// block-level content into divs to stabilize readability scoring.
func normalizeInlineWrappers(doc *html.Node) {
	inlineSel := "span,b,i,strong,em,u,s,code,kbd,mark,q,small,sub,sup,label,time,var,abbr,cite,dfn,tt"
	for _, el := range dom.QuerySelectorAll(doc, inlineSel) {
		if hasBlockDescendant(el) {
			el.Data = "div"
			el.DataAtom = atom.Div
		}
	}
}

// JobTeam has multiple sites with this annoying filterbar
// Purge it
func fixJobTeam(doc *html.Node) {
	for _, el := range dom.QuerySelectorAll(doc, "#filterbar-container") {
		removeNode(el)
	}
}

// preprocessDocument applies structural cleanup before readability parsing:
// - removes dialogs/scripts
// - neutralizes embedded media containers
// - normalizes headers / inline wrappers
// - site specific noise removal
func preprocessDocument(doc *html.Node) {
	for _, n := range dom.QuerySelectorAll(doc, "dialog") {
		removeNode(n)
	}
	for _, n := range dom.QuerySelectorAll(doc, "object,embed,iframe") {
		n.Data = "div"
		n.DataAtom = atom.Div
	}
	for _, n := range dom.QuerySelectorAll(doc, "nav") {
		addClass(n, "nav")
	}
	headerNormalization(doc)
	normalizeInlineWrappers(doc)
	fixJobTeam(doc)
}

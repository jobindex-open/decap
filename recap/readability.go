package recap

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/go-shiori/dom"
	readability "github.com/jobindex-open/go-readability"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var (
	// Tags that can self-close like <img/>
	htmlVoidElements = map[string]struct{}{
		"base": {}, "basefont": {}, "bgsound": {}, "link": {},
		"meta": {}, "area": {}, "br": {}, "embed": {}, "img": {},
		"input": {}, "keygen": {}, "wbr": {}, "param": {},
		"source": {}, "track": {}, "hr": {}, "col": {},
		"frame": {},
	}
	reSelfClosingTag = regexp.MustCompile(`(?i)<([a-z][a-z0-9:_-]*)(\s[^<>]*?)?/\s*>`)
)

func init() {
	// These classnames are candidates for removal
	readability.RxUnlikelyCandidates = appendTokensRegex(readability.RxUnlikelyCandidates, extraUnlikelyTokens)

	// If classname matches these, -25 points
	// contact and widget removed.
	var RxNegative = regexp.MustCompile(`(?i)-ad-|^hid$| hid$| hid |^hid |banner|combx|comment|com-|foot|footer|footnote|gdpr|masthead|media|meta|outbrain|promo|related|share|shoutbox|sidebar|skyscraper|sponsor|shopping|tags|tool`)
	readability.RxNegative = appendTokensRegex(RxNegative, extraNegativeTokens)

	// If classname matches these, +25 points
	readability.RxPositive = appendTokensRegex(readability.RxPositive, extraPositiveTokens)

	// All links punish the score, but these only do it by 30% (0.3) of the original punishment.
	readability.RxHashURL = regexp.MustCompile(`(?i)^(?:#.+|mailto:|tel:)`)
}

// appendTokensRegex merges additional token alternatives into an existing
// compiled regexp used by the readability library.
func appendTokensRegex(existing *regexp.Regexp, extra []string) *regexp.Regexp {
	if existing == nil || len(extra) == 0 {
		return existing
	}
	base := existing.String()
	pattern := base + "|" + strings.Join(extra, "|")
	return regexp.MustCompile(pattern)
}

// net/html is strict about self-closing tags, where browsers arent (historically)
func fixSelfClosingTags(htmlStr string) string {
	return reSelfClosingTag.ReplaceAllStringFunc(htmlStr, func(token string) string {
		sub := reSelfClosingTag.FindStringSubmatch(token)
		tag := strings.ToLower(sub[1])
		if _, ok := htmlVoidElements[tag]; ok {
			return token
		}
		attrs := strings.TrimRight(sub[2], " \t\r\n")
		return fmt.Sprintf("<%s%s></%s>", sub[1], attrs, sub[1])
	})
}

type Parser struct {
	readability.Parser
	article readability.Article
}

func NewParser() Parser {
	p := readability.NewParser()
	// Bullets arent common in articles, but they are in jobads, so include them.
	p.TagsToScore = append(p.TagsToScore, "li", "dt", "dd")
	return Parser{Parser: p}
}

func (p *Parser) Execute(doc *html.Node) error {
	var err error
	p.article, err = p.ParseDocument(doc, nil)
	if err != nil {
		return fmt.Errorf("Readability failed to parse document: %v", err)
	}
	return nil
}

// removeNode safely detaches a node from its parent.
func removeNode(n *html.Node) {
	if n == nil || n.Parent == nil {
		return
	}
	n.Parent.RemoveChild(n)
}

func removeAllMany(doc *html.Node, selectors ...string) {
	for _, n := range dom.QuerySelectorAll(doc, strings.Join(selectors, ",")) {
		removeNode(n)
	}
}

var reHeadline = regexp.MustCompile(`\bh[1-6]\b`)

var headingAtoms = map[string]atom.Atom{
	"h1": atom.H1,
	"h2": atom.H2,
	"h3": atom.H3,
	"h4": atom.H4,
	"h5": atom.H5,
	"h6": atom.H6,
}

func preprocessDocument(doc *html.Node) {
	// Almost certainly trash nodes
	for _, n := range dom.QuerySelectorAll(doc, "dialog,style,turbo-frame,nav") {
		removeNode(n)
	}
	// May contain data we want, make it visible to readability
	for _, n := range dom.QuerySelectorAll(doc, "object,embed,iframe") {
		n.Data = "div"
		n.DataAtom = atom.Div
	}
	// Some sites use h tags as classes instead of tags,
	// meaning we dont score them properly.
	for _, n := range dom.QuerySelectorAll(doc, "*") {
		match := reHeadline.FindString(dom.GetAttribute(n, "class"))
		if match == "" {
			continue
		}
		n.Data = match
		n.DataAtom = headingAtoms[match]
	}
	// Generic bad entries
	removeAllMany(
		doc,
		"[id='cmplz-cookiebanner-container']", // Complianz cookie box killer
		"[id='didomi-host']",                  // Didomi cookie box killer
		"[class='d-none']",                    // Elvium killer
		"[class*='print-hidden']",             // Smartrecruiters killer
		"[id='paywall-modal']",                // nordiskemediehus killer
		"[class*='unprintable']",              // GP killer
		"[aria-hidden='true']",                // GP killer
		"[data-section-name='related-jobs']",  // Teamtailor related-jobs killer
	)
}

var (
	// Elements we want to discard early.
	extraUnlikelyTokens = []string{

		// UI / structural chrome
		"breadcrumb", "filter-bar" /* removed overly-broad "modal" */, "dialog", "paywall",
		"paywall-modal",

		// Navigation
		".*navigation.*", ".+[-_]nav", "nav",

		// Cookie / consent / privacy widgets
		"cookie-alert", "cookie[-_]?banner", "cookie[-_]?consent", "cookie[-_]?notice",
		"cookie[-_]?preferences?", "cookie[-_]?settings?", "consent[-_]?banner",
		"consent[-_]?manager", "privacy[-_]?center", "tracking[-_]?consent",
		"consent[-_]?info", "cmp2",

		// Recommendation / similar content blocks (EN + DA)
		"similar[-_]?jobs?", "similar[-_]?positions?", "similar[-_]?roles?",
		"other[-_]?jobs?", "other[-_]?positions?", "other[-_]?roles?",
		"other[-_]?jobs[-_]?in[-_]?the[-_]?organis(?:ation|ation)",
		"lignende[-_]?jobs?", "lignende[-_]?stillinger",
		"andre[-_]?job", "andre[-_]?jobs?", "andre[-_]?stillinger",
		"related[-_]?jobs?", "related[-_]?positions?", "related[-_]?roles?",
		"lignende[-_]?annoncer", "andre[-_]?annoncer", "relaterede[-_]?annoncer",
		"flere[-_]?annoncer", "mest[-_]?sete[-_]?annoncer", "seneste[-_]?annoncer",
		"nyeste[-_]?annoncer", "similar[-_]?ads?", "other[-_]?ads?", "related[-_]?ads?",
		"more[-_]?ads?",

		// Testimonials / social proof
		"testimonials?",
	}

	// Penalize blocks so they are unlikely top candidates.
	extraNegativeTokens = []string{

		// Testimonials & feedback
		"testimonials?", "testimonial", "hvad[-_]?siger", "siger[-_]?folk",
		"vores[-_]?vikars?[-_]?feedback", "vikars?[-_]?feedback", "brugers?[-_]?feedback",
		"kunde[-_]?udtalelser", "kunders?[-_]?udtalelser", "tilfredse[-_]?kunder",
		"anbefalinger", "reference[r]?s?", "feedback[-_]?section",

		// Recommendation / similar listings
		"similar[-_]?jobs?", "other[-_]?jobs?", "other[-_]?positions?", "other[-_]?roles?",
		"other[-_]?jobs[-_]?in[-_]?the[-_]?organis(?:ation|ation)",
		"similar[-_]?positions?", "similar[-_]?roles?",
		"lignende[-_]?jobs?", "lignende[-_]?stillinger",
		"related[-_]?jobs?", "job[-_]?listings?",
		"andre[-_]?job", "andre[-_]?jobs?", "andre[-_]?stillinger",
		"se[-_]?lokale[-_]?stillinger", "lokale[-_]?stillinger",
		"lignende[-_]?annoncer", "andre[-_]?annoncer", "relaterede[-_]?annoncer",
		"flere[-_]?annoncer", "mest[-_]?sete[-_]?annoncer", "seneste[-_]?annoncer",
		"nyeste[-_]?annoncer", "similar[-_]?ads?", "other[-_]?ads?", "related[-_]?ads?",
		"more[-_]?ads?",

		// Cookie / consent
		"accept[-_]?cookies?", "allow[-_]?all[-_]?cookies?", "only[-_]?necessary[-_]?cookies?",
		"necessary[-_]?cookies?", "reject[-_]?all[-_]?cookies?", "deny[-_]?all[-_]?cookies?",
		"manage[-_]?cookies?", "cookie[-_]?settings?", "cookie[-_]?preferences?",
		"cookie[-_]?consent", "cookie[-_]?banner", "cookie[-_]?notice",
		"consent[-_]?banner", "consent[-_]?manager", "gdpr[-_]?consent",
		"privacy[-_]?preferences?", "privacy[-_]?center", "tracking[-_]?preferences?",
		"tracking[-_]?consent", "consent[-_]?info", "cmp2",

		//
		"branding", "disclaimer",
	}

	// Boost likelihood for authentic job ad / vacancy content.
	extraPositiveTokens = []string{

		// Core job / recruitment
		"job", "jobs?", "jobpost", "job-post", "jobposting", "job-posting",
		"joblisting", "job-listing", "jobboard", "job-board",
		"jobannouncement", "job-announcement", "job-summary",
		"vacancy", "vacancies?", "position", "positions?",
		"role", "roles?", "opening", "openings?", "opportunity", "opportunities?",
		"career", "careers?", "employment", "recruitment", "recruiting",
		"hiring", "apply", "application", "intern", "internship", "trainee",
		"apprentice", "graduate", "student-assistant", "job-details?",

		// Job ad semantic sections (EN)
		"responsibilit(?:y|ies)", "requirements?", "qualifications?", "skills?",
		"benefits?", "perks?", "compensation", "about[-_]?the[-_]?role",
		"about[-_]?you", "about[-_]?us", "who[-_]?you[-_]?are",

		// Danish core terms
		"stilling", "stillinger", "stillingsopslag", "jobopslag", "jobannonce", "karriere",
		"ledig", "ledige", "praktik", "praktikplads?", "praktikant", "studerende",
		"elev", "lærling", "rekruttering", "ansøg", "ansøgning", "ansættelse",

		// Danish semantic sections
		"ansvarsområder", "arbejdsopgaver", "kvalifikationer", "kompetencer",
		"vi[-_]?tilbyder", "om[-_]?stillingen", "om[-_]?dig", "om[-_]?os",

		// Generic section markers useful in structured job ads
		"profile", "jobprofile", "job-profile", "jobdescription", "job-description",
		"jobdetails?", "job[-_]?detail[s]?",

		// General structured text
		"break[-_]?words?", "span[-_]?content",
	}
)

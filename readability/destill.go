package readability

import (
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-shiori/dom"
	readability "github.com/jobindex-open/go-readability"
)

// Request represents the JSON body accepted by the distillation endpoint.
type Request struct {
	HTML    string `json:"html"`    // Raw HTML string to distill.
	BaseURL string `json:"baseUrl"` // Optional base URL used for resolving relative links.
}

type Response struct {
	HTML     string                 `json:"html"`
	Metadata map[string]interface{} `json:"metadata"`
}

var (
	// extraUnlikelyTokens: elements we want to discard early (UI chrome, modals,
	// recommendation rails, ads, cookie / consent surfaces). Grouped & extended.
	extraUnlikelyTokens = []string{
		// UI / structural chrome
		"breadcrumb-info", "filter-bar", "modal", "dialog", "paywall", "paywall-modal",
		// Cookie / consent / privacy widgets
		"cookie-alert", "cookie[-_]?banner", "cookie[-_]?consent", "cookie[-_]?notice",
		"cookie[-_]?preferences?", "cookie[-_]?settings?", "consent[-_]?banner",
		"consent[-_]?manager", "privacy[-_]?center", "tracking[-_]?consent",
		// Recommendation / similar content blocks (EN + DA)
		"similar[-_]?jobs?", "similar[-_]?positions?", "similar[-_]?roles?",
		"other[-_]?jobs?", "other[-_]?positions?", "other[-_]?roles?",
		"other[-_]?jobs[-_]?in[-_]?the[-_]?organis(?:ation|ation)",
		"lignende[-_]?jobs?", "lignende[-_]?stillinger",
		"andre[-_]?job", "andre[-_]?jobs?", "andre[-_]?stillinger",
		"lignende[-_]?annoncer", "andre[-_]?annoncer", "relaterede[-_]?annoncer",
		"flere[-_]?annoncer", "mest[-_]?sete[-_]?annoncer", "seneste[-_]?annoncer",
		"nyeste[-_]?annoncer", "similar[-_]?ads?", "other[-_]?ads?", "related[-_]?ads?", "more[-_]?ads?",
		// Testimonials / social proof
		"testimonials?",
	}

	// extraNegativeTokens: penalize blocks so they are unlikely top candidates.
	// Includes testimonial, recommendation, feedback, ad & consent patterns.
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
		"andre[-_]?job", "andre[-_]?jobs?", "andre[-_]?stillinger",
		"se[-_]?lokale[-_]?stillinger", "lokale[-_]?stillinger",
		"lignende[-_]?annoncer", "andre[-_]?annoncer", "relaterede[-_]?annoncer",
		"flere[-_]?annoncer", "mest[-_]?sete[-_]?annoncer", "seneste[-_]?annoncer",
		"nyeste[-_]?annoncer", "similar[-_]?ads?", "other[-_]?ads?", "related[-_]?ads?", "more[-_]?ads?",
		// Cookie / consent
		"accept[-_]?cookies?", "allow[-_]?all[-_]?cookies?", "only[-_]?necessary[-_]?cookies?",
		"necessary[-_]?cookies?", "reject[-_]?all[-_]?cookies?", "deny[-_]?all[-_]?cookies?",
		"manage[-_]?cookies?", "cookie[-_]?settings?", "cookie[-_]?preferences?",
		"cookie[-_]?consent", "cookie[-_]?banner", "cookie[-_]?notice",
		"consent[-_]?banner", "consent[-_]?manager", "gdpr[-_]?consent",
		"privacy[-_]?preferences?", "privacy[-_]?center", "tracking[-_]?preferences?",
		"tracking[-_]?consent",
	}

	// extraPositiveTokens: boost likelihood for authentic job ad / vacancy content.
	// Extended with semantic sections (responsibilities, qualifications, benefits) EN + DA.
	extraPositiveTokens = []string{
		// Core job / recruitment
		"job", "jobs", "jobpost", "job-post", "jobposting", "job-posting",
		"joblisting", "job-listing", "jobboard", "job-board",
		"jobannouncement", "job-announcement", "job-summary",
		"vacancy", "vacancies", "position", "positions",
		"role", "roles", "opening", "openings", "opportunity", "opportunities",
		"career", "careers", "employment", "recruitment", "recruiting",
		"hiring", "apply", "application", "intern", "internship", "trainee",
		"apprentice", "graduate", "student-assistant",
		// Job ad semantic sections (EN)
		"responsibilit(?:y|ies)", "requirements?", "qualifications?", "skills?",
		"benefits?", "perks?", "compensation", "about[-_]?the[-_]?role",
		"about[-_]?you", "about[-_]?us", "who[-_]?you[-_]?are",
		// Danish core terms
		"stilling", "stillinger", "stillingsopslag", "jobopslag", "jobannonce", "karriere",
		"ledig", "ledige", "praktik", "praktikplads", "praktikant", "studerende",
		"elev", "lærling", "rekruttering", "ansøg", "ansøgning", "ansættelse",
		// Danish semantic sections
		"ansvarsområder", "arbejdsopgaver", "kvalifikationer", "kompetencer",
		"vi[-_]?tilbyder", "om[-_]?stillingen", "om[-_]?dig", "om[-_]?os",
		// Generic section markers useful in structured job ads
		"profile", "jobprofile", "job-profile", "jobdescription", "job-description",
		"jobdetails?", "job-details?",
	}
)

// appendTokensRegex merges additional token alternatives into an existing
// compiled regexp used by the readability library.
func appendTokensRegex(existing *regexp.Regexp, extra []string) *regexp.Regexp {
	if existing == nil || len(extra) == 0 {
		return existing
	}
	return regexp.MustCompile(existing.String() + "|" + strings.Join(extra, "|"))
}

func init() {
	// Extend readability heuristics with domain‑specific token adjustments.
	readability.RxUnlikelyCandidates = appendTokensRegex(readability.RxUnlikelyCandidates, extraUnlikelyTokens)
	readability.RxNegative = appendTokensRegex(readability.RxNegative, extraNegativeTokens)
	readability.RxPositive = appendTokensRegex(readability.RxPositive, extraPositiveTokens)
}

// distillHTML parses, normalizes, and extracts the primary article / job
// content returning simplified HTML plus metadata.
func DistillHTML(htmlStr string, base *url.URL) (*Response, error) {
	parser := readability.NewParser()

	parser.CharThresholds = 300
	parser.TagsToScore = append(parser.TagsToScore, "li", "dt", "dd")

	parser.ClassesToPreserve = append(parser.ClassesToPreserve,
		"caption", "emoji", "hidden", "invisible", "sr-only", "visually-hidden",
		"visuallyhidden", "wp-caption", "wp-caption-text", "wp-smiley",
	)

	doc, err := dom.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil, fmt.Errorf("failed to parse input: %v", err)
	}

	preprocessDocument(doc)

	result, err := parser.ParseDocument(doc, base)
	if err != nil {
		return nil, fmt.Errorf("failed to parse document: %v", err)
	}

	// Export metadata
	metadata := make(map[string]interface{})
	rv := reflect.ValueOf(result)
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() || f.Name == "Node" {
			continue
		}
		metadata[strings.ToLower(f.Name)] = rv.Field(i).Interface()
	}

	htmlOut := dom.InnerHTML(result.Node)
	return &Response{
		HTML:     htmlOut,
		Metadata: metadata,
	}, nil
}

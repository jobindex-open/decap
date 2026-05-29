package recap

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/go-shiori/dom"
	"golang.org/x/net/html"
)

type Request struct {
	HTML     string `json:"html"`
	document *html.Node
}

type Result struct {
	HTML     string         `json:"html"`
	Metadata map[string]any `json:"metadata"`
}

func (r *Request) ParseRequest(body io.Reader) error {
	err := json.NewDecoder(body).Decode(&r)
	if err != nil {
		return fmt.Errorf("JSON parsing error: %s", err)
	}

	if r.HTML == "" {
		return fmt.Errorf("empty HTML string")
	}

	html := fixSelfClosingTags(r.HTML)
	r.document, err = dom.Parse(strings.NewReader(html))
	if err != nil {
		return fmt.Errorf("bad input in HTML field: %v", err)
	}

	return nil
}

func (r *Request) Execute(debug bool) (*Result, error) {
	preprocessDocument(r.document)

	parser := NewParser()
	parser.Debug = debug
	err := parser.Execute(r.document)
	if err != nil {
		return nil, fmt.Errorf("failed to parse document: %v", err)
	}

	res := &Result{
		HTML: dom.InnerHTML(parser.article.Node),
		Metadata: map[string]any{
			"byline":        parser.article.Byline,
			"excerpt":       parser.article.Excerpt,
			"favicon":       parser.article.Favicon,
			"image":         parser.article.Image,
			"language":      parser.article.Language,
			"length":        parser.article.Length,
			"modifiedtime":  parser.article.ModifiedTime,
			"publishedtime": parser.article.PublishedTime,
			"sitename":      parser.article.SiteName,
			"title":         parser.article.Title,
		},
	}
	return res, nil
}

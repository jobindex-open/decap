package recap

import (
	"encoding/json"
	"fmt"
	"io"
)

type Request struct {
	HTML string `json:"html"`
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

	// TODO: Do DOM parsing here and return error

	return nil
}

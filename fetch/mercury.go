package fetch

import (
	"bytes"
	"encoding/json"
	"flag"
	"html/template"
	"os/exec"
)

const (
	contentTemplate = `
		<div class="parsed-content">
			{{.HtmlContent}}
		</div>
	`
)

var (
	mercuryCli = flag.String("mercuryCli", "", "Path to CLI for invoking the Mercury Parser API.")
)

type mercuryResponse struct {
	Content     string `json:"content"`
	HtmlContent template.HTML
}

func parseArticleContent(link string) (content string, err error) {
	if *mercuryCli == "" {
		return
	}

	output, err := exec.Command(*mercuryCli, link).Output()
	if err != nil {
		return
	}

	parsedResp := mercuryResponse{}
	err = json.Unmarshal(output, &parsedResp)
	if err != nil {
		return
	}

	// If the content is empty, return empty string to indicate that this field should not be preferred when displaying.
	if parsedResp.Content == "" {
		return content, nil
	}

	var buf bytes.Buffer
	t, err := template.New("parsedContent").Parse(contentTemplate)
	if err != nil {
		return
	}

	// This third-party HTML is sanitized at a large processing stage, so mark it safe here.
	parsedResp.HtmlContent = template.HTML(parsedResp.Content)
	if err = t.Execute(&buf, parsedResp); err != nil {
		return
	}

	content = buf.String()
	return
}

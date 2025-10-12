package fetch

import (
	"bytes"
	"encoding/json"
	"flag"
	"html/template"

	log "github.com/golang/glog"
	"github.com/jrupac/goliath/utils"
)

const (
	contentTemplate = `
		<div class="parsed-content">
			{{.HtmlContent}}
		</div>
	`
)

var (
	mercuryCli    = flag.String("mercuryCli", "", "Path to CLI for invoking the Mercury Parser API.")
	parseArticles = flag.Bool("parseArticles", false, "If true, parse article content via Mercury API.")
)

var cmdRunner utils.CommandRunner = utils.ExecCommandRunner{}

type mercuryResponse struct {
	Content     string `json:"content"`
	HtmlContent template.HTML
}

func maybeParseArticleContent(link string) (content string) {
	if !*parseArticles {
		return
	}

	if *mercuryCli == "" {
		log.Errorf("--mercuryCli must be set if --parseArticles is true.")
		return
	}

	output, err := cmdRunner.Output(*mercuryCli, link)
	if err != nil {
		log.Warningf("Failed to execute mercury CLI: %s", err)
		return
	}

	parsedResp := mercuryResponse{}
	err = json.Unmarshal(output, &parsedResp)
	if err != nil {
		log.Warningf("Failed to unmarshal mercury CLI response: %s", err)
		return
	}

	// If the content is empty, return empty string to indicate that this field should not be preferred when displaying.
	if parsedResp.Content == "" {
		return
	}

	var buf bytes.Buffer
	t, err := template.New("parsedContent").Parse(contentTemplate)
	if err != nil {
		log.Warningf("Failed to parse HTML template: %s", err)
		return
	}

	// This third-party HTML is sanitized at a later processing stage, so mark it safe here.
	parsedResp.HtmlContent = template.HTML(parsedResp.Content)
	if err = t.Execute(&buf, parsedResp); err != nil {
		log.Warningf("Failed to execute HTML template: %s", err)
		return
	}

	content = buf.String()
	return
}

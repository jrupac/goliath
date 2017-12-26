package fetch

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"html/template"
	"io/ioutil"
	"net/http"
	"time"
)

const (
	mercuryEndpoint = "https://mercury.postlight.com/parser"
	contentTemplate = `
		<div class="parsed-content">
			{{.}}
		</div>
	`
)

var (
	mercuryAPIKey = flag.String("mercuryApiKey", "", "API key for parsing content via Mercury.")

	mercuryHTTPClient = http.Client{Timeout: time.Duration(20 * time.Second)}
)

type mercuryResponse struct {
	Content string `json:"content"`
}

func parseArticleContent(link string) (content string, err error) {
	if *mercuryAPIKey == "" {
		return content, errors.New("no Mercury API provided")
	}

	req, err := http.NewRequest(http.MethodGet, mercuryEndpoint, nil)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", *mercuryAPIKey)

	q := req.URL.Query()
	q.Add("url", link)
	req.URL.RawQuery = q.Encode()

	resp, err := mercuryHTTPClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	parsedResp := mercuryResponse{}
	err = json.Unmarshal(body, &parsedResp)
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

	if err = t.Execute(&buf, template.HTML(parsedResp.Content)); err != nil {
		return
	}

	content = buf.String()
	return
}

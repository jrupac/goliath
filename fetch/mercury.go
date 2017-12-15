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
	MERCURY_ENDPOINT = "https://mercury.postlight.com/parser"
	TEMPLATE         = `
		<div class="parsed-content">
			{{.}}
		</div>
	`
)

var (
	mercuryApiKey = flag.String("mercuryApiKey", "", "API key for parsing content via Mercury.")

	mercuryHttpClient = http.Client{Timeout: time.Duration(20 * time.Second)}
)

type mercuryResponse struct {
	Content string `json:"content"`
}

func parseArticleContent(link string) (content string, err error) {
	if *mercuryApiKey == "" {
		return content, errors.New("no Mercury API provided")
	}

	req, err := http.NewRequest(http.MethodGet, MERCURY_ENDPOINT, nil)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", *mercuryApiKey)

	q := req.URL.Query()
	q.Add("url", link)
	req.URL.RawQuery = q.Encode()

	resp, err := mercuryHttpClient.Do(req)
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
	t, err := template.New("parsedContent").Parse(TEMPLATE)
	if err != nil {
		return
	}

	if err = t.Execute(&buf, template.HTML(parsedResp.Content)); err != nil {
		return
	}

	content = buf.String()
	return
}

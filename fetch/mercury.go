package fetch

import (
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"
	"net/http"
	"time"
)

const (
	MERCURY_ENDPOINT = "https://mercury.postlight.com/parser"
)

var (
	mercuryApiKey = flag.String("mercuryApiKey", "", "API key for parsing content via Mercury.")

	mercuryHttpClient = http.Client{Timeout: time.Duration(10 * time.Second)}
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

	content = parsedResp.Content
	return
}

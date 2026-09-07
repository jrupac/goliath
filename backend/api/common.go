package api

import (
	"flag"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jrupac/goliath/storage"
)

var (
	serveParsedArticles = flag.Bool("serveParsedArticles", false, "If true, serve parsed article content.")
)

// postTokenParam is the form field carrying a GReader post token.
const postTokenParam = "T"

// redactedValue replaces a credential wherever a request is logged.
const redactedValue = "[REDACTED]"

type apiResponse map[string]interface{}

type apiError struct {
	wrapped  error
	internal bool
}

func (e *apiError) Error() string {
	return e.wrapped.Error()
}

// Api is an interface that REST APIs should implement.
type Api interface {
	Handler(d *storage.Database) func(w http.ResponseWriter, r *http.Request)

	recordLatency(time.Time, string)
	returnError(http.ResponseWriter, string, error)
	returnSuccess(http.ResponseWriter, apiResponse)
}

// redactedFormKeys are form values masked before a request is written to the
// log. Requests are logged in full to make client behavior legible, and these
// are the fields where that would otherwise mean writing a credential into the
// log in the clear:
//
//   - `Passwd` is the account password, sent on every GReader login.
//   - `api_key` is the Fever credential, which is derived from the password,
//     never expires, and cannot be revoked.
//   - `T` is a GReader post token, which is short-lived and useless on its own,
//     but is still something that authorizes a request.
var redactedFormKeys = map[string]bool{
	"Passwd":       true,
	"api_key":      true,
	postTokenParam: true,
}

// redactFormValues renders a form for logging with credential-bearing values
// masked.
func redactFormValues(form url.Values) string {
	safe := make(url.Values, len(form))
	for key, values := range form {
		if !redactedFormKeys[key] {
			safe[key] = values
			continue
		}
		for _, value := range values {
			if key == postTokenParam {
				safe.Add(key, redactPostToken(value))
			} else {
				safe.Add(key, redactedValue)
			}
		}
	}
	return safe.Encode()
}

// redactPostToken masks the signature that makes a post token usable while
// keeping its issue time, which is not a secret. That is what makes a rejected
// token legible in a log: how old it was, and whether the client went and
// fetched another.
func redactPostToken(token string) string {
	issued, _, found := strings.Cut(token, ".")
	if !found {
		return redactedValue
	}
	return issued + "." + redactedValue
}

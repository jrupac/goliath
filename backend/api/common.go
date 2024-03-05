package api

import (
	"flag"
	"github.com/jrupac/goliath/storage"
	"net/http"
	"time"
)

var (
	serveParsedArticles = flag.Bool("serveParsedArticles", false, "If true, serve parsed article content.")
)

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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/auth"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/goliath/utils"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const apiVersion = 3

var (
	serveParsedArticles = flag.Bool("serveParsedArticles", false, "If true, serve parsed article content.")
)

var (
	latencyMetric = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "fever_server_latency",
			Help:       "Server-side latency of Fever API operations.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"method"},
	)
)

type itemType struct {
	ID          int64  `json:"id"`
	FeedID      int64  `json:"feed_id"`
	Title       string `json:"title"`
	Author      string `json:"author"`
	HTML        string `json:"html"`
	URL         string `json:"url"`
	IsSaved     int64  `json:"is_saved"`
	IsRead      int64  `json:"is_read"`
	CreatedTime int64  `json:"created_on_time"`
}

type feedType struct {
	ID          int64  `json:"id"`
	FaviconID   int64  `json:"favicon_id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	SiteURL     string `json:"site_url"`
	IsSpark     int64  `json:"is_spark"`
	LastUpdated int64  `json:"last_updated_on_time"`
}

type groupType struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

type faviconType struct {
	ID   int64  `json:"id"`
	Data string `json:"data"`
}

type feedsGroupType struct {
	GroupID int64  `json:"group_id"`
	FeedIDs string `json:"feed_ids"`
}

type responseType map[string]interface{}

type feverError struct {
	wrapped  error
	internal bool
}

func (e *feverError) Error() string {
	return e.wrapped.Error()
}

func init() {
	prometheus.MustRegister(latencyMetric)
}

func recordLatency(label string) func(time.Duration) {
	return func(d time.Duration) {
		// Record latency measurements in microseconds.
		latencyMetric.WithLabelValues(label).Observe(float64(d) / float64(time.Microsecond))
	}
}

// HandleFever returns a handler function that implements the Fever API.
func HandleFever(d *storage.Database) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		handleFever(d, w, r)
	}
}

func handleFever(d *storage.Database, w http.ResponseWriter, r *http.Request) {
	// Record the total server latency of each Fever call.
	defer utils.Elapsed(time.Now(), recordLatency("server"))

	// These two fields must always be set on responses.
	resp := responseType{
		"api_version":            apiVersion,
		"last_refreshed_on_time": time.Now().Unix(),
	}

	err := r.ParseForm()
	if err != nil {
		returnError(w, "Failed to parse request: %s", err)
		return
	}

	log.Infof("Fever request URL: %s", r.URL.String())
	log.Infof("Fever request body: %s", r.PostForm.Encode())

	switch r.Form.Get("api") {
	case "":
		w.Header().Set("Content-Type", "application/json")
	case "xml":
		// TODO: Implement XML response type.
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	user, authStatus := handleAuth(d, r)
	resp["auth"] = authStatus
	if resp["auth"] == 0 {
		returnSuccess(w, resp)
		return
	}

	if _, ok := r.Form["groups"]; ok {
		err := handleGroups(d, user, &resp)
		if err != nil {
			returnError(w, "Failed request 'groups': %s", err)
			return
		}
	}
	if _, ok := r.Form["feeds"]; ok {
		err := handleFeeds(d, user, &resp)
		if err != nil {
			returnError(w, "Failed request 'feeds': %s", err)
			return
		}
	}
	if _, ok := r.Form["favicons"]; ok {
		err := handleFavicons(d, user, &resp)
		if err != nil {
			returnError(w, "Failed request 'favicons': %s", err)
			return
		}
	}
	if _, ok := r.Form["items"]; ok {
		err := handleItems(d, user, &resp, r)
		if err != nil {
			returnError(w, "Failed request 'items': %s", err)
			return
		}
	}
	if _, ok := r.Form["links"]; ok {
		err := handleLinks(d, user, &resp)
		if err != nil {
			returnError(w, "Failed request 'links': %s", err)
			return
		}
	}
	if _, ok := r.Form["unread_item_ids"]; ok {
		err := handleUnreadItemIDs(d, user, &resp)
		if err != nil {
			returnError(w, "Failed request 'unread_item_ids': %s", err)
			return
		}
	}
	if _, ok := r.Form["saved_item_ids"]; ok {
		err := handleSavedItemIDs(d, user, &resp)
		if err != nil {
			returnError(w, "Failed request 'saved_item_ids': %s", err)
			return
		}
	}
	if _, ok := r.Form["mark"]; ok {
		err := handleMark(d, user, &resp, r)
		if err != nil {
			returnError(w, "Failed request 'mark': %s", err)
			return
		}
	}

	returnSuccess(w, resp)
}

func returnError(w http.ResponseWriter, msg string, err error) {
	log.Warningf(msg, err)
	if fe, ok := err.(*feverError); ok {
		if fe.internal {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
}

func returnSuccess(w http.ResponseWriter, resp map[string]interface{}) {
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	// HTML content is escaped already during fetch time (e.g., ' --> &#39;), so
	// do not HTML escape it further (e.g., &#39; --> \u0026#39;), which would
	// render incorrectly on a webpage.
	enc.SetEscapeHTML(false)
	if err := enc.Encode(resp); err != nil {
		returnError(w, "Failed to encode response JSON: %s", err)
	}
}

func handleAuth(d *storage.Database, r *http.Request) (models.User, int) {
	defer utils.Elapsed(time.Now(), recordLatency("auth"))

	// A request can be authenticated by cookie or api key in request.
	if user, err := auth.VerifyCookie(d, r); err == nil {
		log.V(2).Infof("Verified cookie: %+v", r)
		return user, 1
	} else if user, err := d.GetUserByKey(r.FormValue("api_key")); err != nil {
		utils.HttpRequestPrint("Received unauthenticated request", r)
		log.Warningf("Failed because: %s", err)
		return user, 0
	} else {
		log.V(2).Infof("Successfully authenticated by key: %+v", r)
		return user, 1
	}
}

func handleGroups(d *storage.Database, u models.User, resp *responseType) error {
	defer utils.Elapsed(time.Now(), recordLatency("groups"))

	folders, err := d.GetAllFoldersForUser(u)
	if err != nil {
		return &feverError{err, true}
	}
	var groups []groupType
	for _, f := range folders {
		g := groupType{
			ID:    f.ID,
			Title: f.Name,
		}
		groups = append(groups, g)
	}
	(*resp)["groups"] = groups
	(*resp)["feeds_groups"], err = constructFeedsGroups(d, u)
	if err != nil {
		return &feverError{err, true}
	}
	return nil
}

func handleFeeds(d *storage.Database, u models.User, resp *responseType) error {
	defer utils.Elapsed(time.Now(), recordLatency("feeds"))

	fetchedFeeds, err := d.GetAllFeedsForUser(u)
	if err != nil {
		return &feverError{err, true}
	}
	var feeds []feedType
	for _, ff := range fetchedFeeds {
		f := feedType{
			ID:          ff.ID,
			FaviconID:   ff.ID,
			Title:       ff.Title,
			URL:         ff.URL,
			SiteURL:     ff.Link,
			IsSpark:     0,
			LastUpdated: time.Now().Unix(),
		}
		feeds = append(feeds, f)
	}
	(*resp)["feeds"] = feeds
	(*resp)["feeds_groups"], err = constructFeedsGroups(d, u)
	if err != nil {
		return &feverError{err, true}
	}
	return nil
}

func handleFavicons(d *storage.Database, u models.User, resp *responseType) error {
	faviconMap, err := d.GetAllFaviconsForUser(u)
	if err != nil {
		return &feverError{err, true}
	}
	var favicons []faviconType
	for k, v := range faviconMap {
		f := faviconType{
			ID:   k,
			Data: v,
		}
		favicons = append(favicons, f)
	}
	(*resp)["favicons"] = favicons
	return nil
}

func handleItems(d *storage.Database, u models.User, resp *responseType, r *http.Request) error {
	defer utils.Elapsed(time.Now(), recordLatency("items"))

	// TODO: support "max_id" and "with_ids".
	sinceID := int64(-1)
	var err error

	if _, ok := r.Form["since_id"]; ok {
		sinceID, err = strconv.ParseInt(r.FormValue("since_id"), 10, 64)
		if err != nil {
			return &feverError{err, false}
		}
	}

	articles, err := d.GetUnreadArticlesForUser(u, 50, sinceID)
	if err != nil {
		return &feverError{err, true}
	}
	// Make an empty (not nil) slice because their JSON encodings are different.
	items := make([]itemType, 0)
	var content string
	for _, a := range articles {
		if *serveParsedArticles && a.Parsed != "" {
			log.Infof("Serving parsed content for title: %s", a.Title)
			content = a.Parsed
		} else if a.Content != "" {
			// The "content" field usually has more text, but is not always set.
			content = a.Content
		} else {
			content = a.Summary
		}

		i := itemType{
			ID:          a.ID,
			FeedID:      a.FeedID,
			Title:       a.Title,
			Author:      "",
			HTML:        content,
			URL:         a.Link,
			IsSaved:     0,
			IsRead:      0,
			CreatedTime: a.Date.Unix(),
		}
		items = append(items, i)
	}
	(*resp)["items"] = items
	return nil
}

func handleLinks(_ *storage.Database, _ models.User, resp *responseType) error {
	defer utils.Elapsed(time.Now(), recordLatency("links"))

	// Perhaps add support for links in the future.
	(*resp)["links"] = ""
	return nil
}

func handleUnreadItemIDs(d *storage.Database, u models.User, resp *responseType) error {
	defer utils.Elapsed(time.Now(), recordLatency("unread_item_ids"))

	articles, err := d.GetUnreadArticlesForUser(u, -1, -1)
	if err != nil {
		return &feverError{err, true}
	}
	var unreadItemIds []string
	for _, a := range articles {
		unreadItemIds = append(unreadItemIds, strconv.FormatInt(a.ID, 10))
	}
	(*resp)["unread_item_ids"] = strings.Join(unreadItemIds, ",")
	return nil
}

func handleSavedItemIDs(_ *storage.Database, _ models.User, resp *responseType) error {
	defer utils.Elapsed(time.Now(), recordLatency("saved_item_ids"))

	// Perhaps add support for saving items in the future.
	(*resp)["saved_item_ids"] = ""
	return nil
}

func handleMark(d *storage.Database, u models.User, _ *responseType, r *http.Request) error {
	defer utils.Elapsed(time.Now(), recordLatency("mark"))

	// TODO: Support "before" argument.
	var as string
	switch r.FormValue("as") {
	case "read", "unread", "saved", "unsaved":
		as = r.FormValue("as")
	default:
		return &feverError{fmt.Errorf("unknown 'as' value: %s", r.FormValue("as")), false}
	}

	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		return err
	}

	switch r.FormValue("mark") {
	case "item":
		if err = d.MarkArticleForUser(u, id, as); err != nil {
			return &feverError{err, true}
		}
	case "feed":
		if err = d.MarkFeedForUser(u, id, as); err != nil {
			return &feverError{err, true}
		}
	case "group":
		if err = d.MarkFolderForUser(u, id, as); err != nil {
			return &feverError{err, true}
		}
	default:
		return &feverError{fmt.Errorf("malformed 'mark' value: %s", r.FormValue("mark")), false}
	}
	return nil
}

func constructFeedsGroups(d *storage.Database, u models.User) ([]feedsGroupType, error) {
	var feedGroups []feedsGroupType
	feedsPerFolder, err := d.GetFeedsPerFolderForUser(u)
	if err != nil {
		log.Warningf("Failed to fetch feeds per folder: %s", err)
		return feedGroups, err
	}
	for k, v := range feedsPerFolder {
		feedGroup := feedsGroupType{
			GroupID: k,
			FeedIDs: v,
		}
		feedGroups = append(feedGroups, feedGroup)
	}
	return feedGroups, nil
}

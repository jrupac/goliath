package api

import (
	"encoding/json"
	"errors"
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
	feverLatencyMetric = prometheus.NewSummaryVec(
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

func init() {
	prometheus.MustRegister(feverLatencyMetric)
}

// Fever is an implementation of the Fever API.
type Fever struct {
}

// FeverHandler returns a new Fever handler.
func FeverHandler(d storage.Database) http.HandlerFunc {
	return Fever{}.Handler(d)
}

// Handler returns a handler function that implements the Fever API.
func (a Fever) Handler(d storage.Database) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.handle(d, w, r)
	}
}

func (a Fever) recordLatency(t time.Time, label string) {
	utils.Elapsed(t, func(d time.Duration) {
		// Record latency measurements in microseconds.
		feverLatencyMetric.WithLabelValues(label).Observe(float64(d) / float64(time.Microsecond))
	})
}

func (a Fever) handle(d storage.Database, w http.ResponseWriter, r *http.Request) {
	// Record the total server latency of each call.
	defer a.recordLatency(time.Now(), "server")

	// These two fields must always be set on responses.
	resp := responseType{
		"api_version":            apiVersion,
		"last_refreshed_on_time": time.Now().Unix(),
	}

	err := r.ParseForm()
	if err != nil {
		a.returnError(w, "Failed to parse request: %s", err)
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

	user, authStatus := a.handleAuth(d, r)
	resp["auth"] = authStatus
	if resp["auth"] == 0 {
		a.returnSuccess(w, resp)
		return
	}

	if _, ok := r.Form["groups"]; ok {
		err := a.handleGroups(d, user, &resp)
		if err != nil {
			a.returnError(w, "Failed request 'groups': %s", err)
			return
		}
	}
	if _, ok := r.Form["feeds"]; ok {
		err := a.handleFeeds(d, user, &resp)
		if err != nil {
			a.returnError(w, "Failed request 'feeds': %s", err)
			return
		}
	}
	if _, ok := r.Form["favicons"]; ok {
		err := a.handleFavicons(d, user, &resp)
		if err != nil {
			a.returnError(w, "Failed request 'favicons': %s", err)
			return
		}
	}
	if _, ok := r.Form["items"]; ok {
		err := a.handleItems(d, user, &resp, r)
		if err != nil {
			a.returnError(w, "Failed request 'items': %s", err)
			return
		}
	}
	if _, ok := r.Form["links"]; ok {
		err := a.handleLinks(d, user, &resp)
		if err != nil {
			a.returnError(w, "Failed request 'links': %s", err)
			return
		}
	}
	if _, ok := r.Form["unread_item_ids"]; ok {
		err := a.handleUnreadItemIDs(d, user, &resp)
		if err != nil {
			a.returnError(w, "Failed request 'unread_item_ids': %s", err)
			return
		}
	}
	if _, ok := r.Form["saved_item_ids"]; ok {
		err := a.handleSavedItemIDs(d, user, &resp)
		if err != nil {
			a.returnError(w, "Failed request 'saved_item_ids': %s", err)
			return
		}
	}
	if _, ok := r.Form["mark"]; ok {
		err := a.handleMark(d, user, &resp, r)
		if err != nil {
			a.returnError(w, "Failed request 'mark': %s", err)
			return
		}
	}

	a.returnSuccess(w, resp)
}

func (a Fever) returnError(w http.ResponseWriter, msg string, err error) {
	log.Warningf(msg, err)
	var fe *apiError
	if errors.As(err, &fe) {
		if fe.internal {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
}

func (a Fever) returnSuccess(w http.ResponseWriter, resp map[string]interface{}) {
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	// HTML content is escaped already during fetch time (e.g., ' --> &#39;), so
	// do not HTML escape it further (e.g., &#39; --> \u0026#39;), which would
	// render incorrectly on a webpage.
	enc.SetEscapeHTML(false)
	if err := enc.Encode(resp); err != nil {
		a.returnError(w, "Failed to encode response JSON: %s", err)
	}
}

func (a Fever) handleAuth(d storage.Database, r *http.Request) (models.User, int) {
	defer a.recordLatency(time.Now(), "auth")

	// A request can be authenticated by a cookie or an api key in the request.
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

func (a Fever) handleGroups(d storage.Database, u models.User, resp *responseType) error {
	defer a.recordLatency(time.Now(), "groups")

	folders, err := d.GetAllFoldersForUser(u)
	if err != nil {
		return &apiError{err, true}
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
	(*resp)["feeds_groups"], err = a.constructFeedsGroups(d, u)
	if err != nil {
		return &apiError{err, true}
	}
	return nil
}

func (a Fever) handleFeeds(d storage.Database, u models.User, resp *responseType) error {
	defer a.recordLatency(time.Now(), "feeds")

	fetchedFeeds, err := d.GetAllFeedsForUser(u)
	if err != nil {
		return &apiError{err, true}
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
	(*resp)["feeds_groups"], err = a.constructFeedsGroups(d, u)
	if err != nil {
		return &apiError{err, true}
	}
	return nil
}

func (a Fever) handleFavicons(d storage.Database, u models.User, resp *responseType) error {
	faviconMap, err := d.GetAllFaviconsForUser(u)
	if err != nil {
		return &apiError{err, true}
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

func (a Fever) handleItems(d storage.Database, u models.User, resp *responseType, r *http.Request) error {
	defer a.recordLatency(time.Now(), "items")

	// TODO: support "max_id" and "with_ids".
	sinceID := int64(-1)
	var err error

	if _, ok := r.Form["since_id"]; ok {
		sinceID, err = strconv.ParseInt(r.FormValue("since_id"), 10, 64)
		if err != nil {
			return &apiError{err, false}
		}
	}

	articles, err := d.GetArticlesWithFilterForUser(u, models.StreamFilterUnread, 50, sinceID)
	if err != nil {
		return &apiError{err, true}
	}
	// Make an empty (not nil) slice because their JSON encodings are different.
	items := make([]itemType, 0)
	for _, a := range articles {
		i := itemType{
			ID:          a.ID,
			FeedID:      a.FeedID,
			Title:       a.Title,
			Author:      "",
			HTML:        a.GetContents(*serveParsedArticles),
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

func (a Fever) handleLinks(_ storage.Database, _ models.User, resp *responseType) error {
	defer a.recordLatency(time.Now(), "links")

	// Perhaps add support for links in the future.
	(*resp)["links"] = ""
	return nil
}

func (a Fever) handleUnreadItemIDs(d storage.Database, u models.User, resp *responseType) error {
	defer a.recordLatency(time.Now(), "unread_item_ids")

	articles, err := d.GetArticleMetaWithFilterForUser(u, models.StreamFilterUnread, -1, -1)
	if err != nil {
		return &apiError{err, true}
	}
	var unreadItemIds []string
	for _, a := range articles {
		unreadItemIds = append(unreadItemIds, strconv.FormatInt(a.ID, 10))
	}
	(*resp)["unread_item_ids"] = strings.Join(unreadItemIds, ",")
	return nil
}

func (a Fever) handleSavedItemIDs(_ storage.Database, _ models.User, resp *responseType) error {
	defer a.recordLatency(time.Now(), "saved_item_ids")

	// Perhaps add support for saving items in the future.
	(*resp)["saved_item_ids"] = ""
	return nil
}

func (a Fever) handleMark(d storage.Database, u models.User, _ *responseType, r *http.Request) error {
	defer a.recordLatency(time.Now(), "mark")

	// TODO: Support "before" argument.
	var as models.MarkAction
	switch r.FormValue("as") {
	case "read":
		as = models.MarkActionRead
	case "unread":
		as = models.MarkActionUnread
	case "saved":
		as = models.MarkActionSaved
	case "unsaved":
		as = models.MarkActionUnsaved
	default:
		return &apiError{fmt.Errorf("unknown 'as' value: %s", r.FormValue("as")), false}
	}

	id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
	if err != nil {
		return err
	}

	switch r.FormValue("mark") {
	case "item":
		if err = d.MarkArticleForUser(u, id, as); err != nil {
			return &apiError{err, true}
		}
	case "feed":
		if err = d.MarkFeedForUser(u, id, as); err != nil {
			return &apiError{err, true}
		}
	case "group":
		if err = d.MarkFolderForUser(u, id, as); err != nil {
			return &apiError{err, true}
		}
	default:
		return &apiError{fmt.Errorf("malformed 'mark' value: %s", r.FormValue("mark")), false}
	}
	return nil
}

func (a Fever) constructFeedsGroups(d storage.Database, u models.User) ([]feedsGroupType, error) {
	var feedGroups []feedsGroupType
	feedsPerFolder, err := d.GetFeedsPerFolderForUser(u)
	if err != nil {
		log.Warningf("Failed to fetch feeds per folder: %s", err)
		return feedGroups, err
	}
	for k, v := range feedsPerFolder {
		var feedIds []string
		for _, i := range v {
			feedIds = append(feedIds, strconv.FormatInt(i, 10))
		}

		feedGroup := feedsGroupType{
			GroupID: k,
			FeedIDs: strings.Join(feedIds, ","),
		}
		feedGroups = append(feedGroups, feedGroup)
	}
	return feedGroups, nil
}

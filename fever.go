package main

import (
	"encoding/json"
	"flag"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/auth"
	"github.com/jrupac/goliath/storage"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const apiVersion = 3

var (
	serveParsedArticles = flag.Bool("serveParsedArticles", false, "If true, serve parsed article content.")
)

type itemType struct {
	ID          int64  `json:"id"`
	FeedID      int64  `json:"feed_id"`
	Title       string `json:"title"`
	Author      string `json:"author"`
	HTML        string `json:"html"`
	URL         string `json:"url"`
	IsSaved     bool   `json:"is_saved"`
	IsRead      bool   `json:"is_read"`
	CreatedTime int64  `json:"created_on_time"`
}

type feedType struct {
	ID          int64  `json:"id"`
	FaviconID   int64  `json:"favicon_id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	SiteURL     string `json:"site_url"`
	IsSpark     bool   `json:"is_spark"`
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

// HandleFever returns a handler function that implements the Fever API.
func HandleFever(d *storage.Database) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		handleFever(d, w, r)
	}
}

func handleFever(d *storage.Database, w http.ResponseWriter, r *http.Request) {
	// These two fields must always be set on responses.
	resp := map[string]interface{}{
		"api_version":            apiVersion,
		"last_refreshed_on_time": time.Now().Unix(),
	}

	r.ParseForm()
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

	resp["auth"] = handleAuth(d, r)
	if resp["auth"] == 0 {
		returnSuccess(w, resp)
		return
	}

	if _, ok := r.Form["groups"]; ok {
		folders, err := d.GetAllFolders()
		if err != nil {
			returnError(w, "Failed to fetch folders: %s", err)
			return
		}
		var groups []groupType
		for _, f := range folders {
			g := groupType{
				ID:    f.ID,
				Title: f.Name,
			}
			groups = append(groups, g)
		}
		resp["groups"] = groups

		resp["feeds_groups"], err = constructFeedsGroups(d)
		if err != nil {
			returnError(w, "Failed to fetch feeds per folder: %s", err)
			return
		}
	}
	if _, ok := r.Form["feeds"]; ok {
		fetchedFeeds, err := d.GetAllFeeds()
		if err != nil {
			returnError(w, "Failed to fetch feeds: %s", err)
			return
		}
		var feeds []feedType
		for _, ff := range fetchedFeeds {
			f := feedType{
				ID:          ff.ID,
				FaviconID:   ff.ID,
				Title:       ff.Title,
				URL:         ff.URL,
				SiteURL:     ff.URL,
				IsSpark:     false,
				LastUpdated: time.Now().Unix(),
			}
			feeds = append(feeds, f)
		}
		resp["feeds"] = feeds
		resp["feeds_groups"], err = constructFeedsGroups(d)
		if err != nil {
			returnError(w, "Failed to fetch feeds per folder: %s", err)
			return
		}
	}
	if _, ok := r.Form["favicons"]; ok {
		faviconMap, err := d.GetAllFavicons()
		if err != nil {
			returnError(w, "Failed to fetch favicons: %s", err)
			return
		}
		var favicons []faviconType
		for k, v := range faviconMap {
			f := faviconType{
				ID:   k,
				Data: v,
			}
			favicons = append(favicons, f)
		}
		resp["favicons"] = favicons
	}
	if _, ok := r.Form["items"]; ok {
		// TODO: support "max_id" and "with_ids".
		sinceID := int64(-1)
		var err error

		if _, ok2 := r.Form["since_id"]; ok2 {
			sinceID, err = strconv.ParseInt(r.FormValue("since_id"), 10, 64)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}

		articles, err := d.GetUnreadArticles(50, sinceID)
		if err != nil {
			returnError(w, "Failed to fetch unread articles: %s", err)
			return
		}
		var items []itemType
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
				IsSaved:     false,
				IsRead:      false,
				CreatedTime: a.Date.Unix(),
			}
			items = append(items, i)
		}
		resp["items"] = items
	}
	if _, ok := r.Form["links"]; ok {
		// Perhaps add support for Hot Links in the future.
		resp["links"] = ""
	}
	if _, ok := r.Form["unread_item_ids"]; ok {
		articles, err := d.GetUnreadArticles(-1, -1)
		if err != nil {
			returnError(w, "Failed to fetch unread articles: %s", err)
			return
		}
		var unreadItemIds []string
		for _, a := range articles {
			unreadItemIds = append(unreadItemIds, strconv.FormatInt(a.ID, 10))
		}
		resp["unread_item_ids"] = strings.Join(unreadItemIds, ",")
	}
	if _, ok := r.Form["saved_item_ids"]; ok {
		// Perhaps add support for saving items in the future.
		resp["saved_item_ids"] = ""
	}
	if _, ok := r.Form["mark"]; ok {
		// TODO: Support "before" argument.
		var as string
		switch r.FormValue("as") {
		case "read", "unread", "saved", "unsaved":
			as = r.FormValue("as")
		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		switch r.FormValue("mark") {
		case "item":
			if err2 := d.MarkArticle(id, as); err2 != nil {
				returnError(w, "Unable to mark article: %s", err2)
			}
		case "feed":
			if err2 := d.MarkFeed(id, as); err2 != nil {
				returnError(w, "Unable to mark feed: %s", err2)
			}
		case "group":
			if err2 := d.MarkFolder(id, as); err2 != nil {
				returnError(w, "Unable to mark folder: %s", err2)
			}
		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	returnSuccess(w, resp)
}

func returnError(w http.ResponseWriter, msg string, err error) {
	log.Warningf(msg, err)
	w.WriteHeader(http.StatusInternalServerError)
}

func returnSuccess(w http.ResponseWriter, resp map[string]interface{}) {
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		returnError(w, "Failed to encode response JSON: %s", err)
	}
}

func handleAuth(d *storage.Database, r *http.Request) int {
	// A request can be authenticated by cookie or api key in request.
	if auth.VerifyCookie(d, r) {
		log.V(2).Infof("Verified cookie: %+v", r)
		return 1
	} else if _, err := d.GetUserByKey(r.FormValue("api_key")); err != nil {
		log.Warningf("Rejected request: %+v", r)
		log.Warningf("Failed because: %s", err)
		return 0
	} else {
		log.V(2).Infof("Successfully authenticated by key: %+v", r)
		return 1
	}
}

func constructFeedsGroups(d *storage.Database) ([]feedsGroupType, error) {
	var feedGroups []feedsGroupType
	feedsPerFolder, err := d.GetFeedsPerFolder()
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
	return feedGroups, err
}

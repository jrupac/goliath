package main

import (
	"encoding/json"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/storage"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const API_VERSION = "1.0"

type item struct {
	Id          int64  `json:"id"`
	FeedId      int64  `json:"feed_id"`
	Title       string `json:"title"`
	Author      string `json:"author"`
	Html        string `json:"html"`
	Url         string `json:"url"`
	IsSaved     bool   `json:"is_saved"`
	IsRead      bool   `json:"is_read"`
	CreatedTime int64  `json:"created_on_time"`
}

type feed struct {
	Id          int64  `json:"id"`
	FaviconId   int64  `json:"favicon_id"`
	Title       string `json:"title"`
	Url         string `json:"url"`
	SiteUrl     string `json:"site_url"`
	IsSpark     bool   `json:"is_spark"`
	LastUpdated int64  `json:"last_updated_on_time"`
}

type group struct {
	Id    int64  `json:"id"`
	Title string `json:"title"`
}

type favicon struct {
	Id   int64  `json:"id"`
	Data string `json:"data"`
}

type feedsGroup struct {
	GroupId int64  `json:"group_id"`
	FeedIds string `json:"feed_ids"`
}

func HandleFever(d *storage.Database) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		handleFever(d, w, r)
	}
}

func handleFever(d *storage.Database, w http.ResponseWriter, r *http.Request) {
	// These two fields must always be set on responses.
	resp := map[string]interface{}{
		"api_version":            API_VERSION,
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

	resp["auth"] = auth(r.FormValue("api_key"))
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
		groups := []group{}
		for _, f := range folders {
			group := group{
				Id:    f.Id,
				Title: f.Name,
			}
			groups = append(groups, group)
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
		feeds := []feed{}
		for _, ff := range fetchedFeeds {
			f := feed{
				Id:          ff.Id,
				FaviconId:   ff.Id,
				Title:       ff.Title,
				Url:         ff.Url,
				SiteUrl:     ff.Url,
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
		favicons := []favicon{}
		for k, v := range faviconMap {
			favicon := favicon{
				Id:   k,
				Data: v,
			}
			favicons = append(favicons, favicon)
		}
		resp["favicons"] = favicons
	}
	if _, ok := r.Form["items"]; ok {
		// TODO: support "max_id" and "with_ids".
		since_id := int64(-1)
		var err error

		if _, ok := r.Form["since_id"]; ok {
			since_id, err = strconv.ParseInt(r.FormValue("since_id"), 10, 64)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}

		articles, err := d.GetUnreadArticles(50, since_id)
		if err != nil {
			returnError(w, "Failed to fetch unread articles: %s", err)
			return
		}
		items := []item{}
		var content string
		for _, a := range articles {
			// The "content" field usually has more text, but is not always set.
			if a.Content != "" {
				content = a.Content
			} else {
				content = a.Summary
			}

			i := item{
				Id:          a.Id,
				FeedId:      a.FeedId,
				Title:       a.Title,
				Author:      "",
				Html:        content,
				Url:         a.Link,
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
		unread_item_ids := []string{}
		for _, a := range articles {
			unread_item_ids = append(unread_item_ids, strconv.FormatInt(a.Id, 10))
		}
		resp["unread_item_ids"] = strings.Join(unread_item_ids, ",")
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
			if err := d.MarkArticle(id, as); err != nil {
				returnError(w, "Unable to mark article: %s", err)
			}
		case "feed":
			if err := d.MarkFeed(id, as); err != nil {
				returnError(w, "Unable to mark feed: %s", err)
			}
		case "group":
			if err := d.MarkFolder(id, as); err != nil {
				returnError(w, "Unable to mark folder: %s", err)
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

func auth(hash string) int {
	// TODO: Implement validation via DB lookup.
	if hash == "" {
		log.Warningf("No api_key provided.")
	}
	return 1
}

func constructFeedsGroups(d *storage.Database) ([]feedsGroup, error) {
	feedGroups := []feedsGroup{}
	feedsPerFolder, err := d.GetFeedsPerFolder()
	if err != nil {
		log.Warningf("Failed to fetch feeds per folder: %s", err)
		return feedGroups, err
	}
	for k, v := range feedsPerFolder {
		feedGroup := feedsGroup{
			GroupId: k,
			FeedIds: v,
		}
		feedGroups = append(feedGroups, feedGroup)
	}
	return feedGroups, err
}

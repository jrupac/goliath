package main

import (
	"encoding/json"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/storage"
	"net/http"
	"time"
)

const API_VERSION = "1.0"

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
		log.Infof("Handling request.")
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
	// Endpoint must be "/fever?api" so check specifically that "api" is in the URL.
	if _, ok := r.Form["api"]; !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

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
		encodeResponse(w, resp)
		return
	}

	if _, ok := r.Form["groups"]; ok {
		folders, err := d.GetAllFolders()
		if err != nil {
			log.Warningf("Failed to fetch folders: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
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
			log.Warningf("Failed to fetch feeds per folder: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else if _, ok := r.Form["feeds"]; ok {
		fetchedFeeds, err := d.GetAllFeeds()
		if err != nil {
			log.Warningf("Failed to fetch feeds: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
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
			log.Warningf("Failed to fetch feeds per folder: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else if _, ok := r.Form["favicons"]; ok {
		faviconMap, err := d.GetAllFavicons()
		if err != nil {
			log.Warningf("Failed to fetch favicons: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
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
	} else if _, ok := r.Form["items"]; ok {
		w.WriteHeader(http.StatusNotImplemented)
		return
	} else if _, ok := r.Form["links"]; ok {
		w.WriteHeader(http.StatusNotImplemented)
		return
	} else if _, ok := r.Form["unread_item_ids"]; ok {
		w.WriteHeader(http.StatusNotImplemented)
		return
	} else if _, ok := r.Form["saved_item_ids"]; ok {
		w.WriteHeader(http.StatusNotImplemented)
		return
	}

	encodeResponse(w, resp)
}

func encodeResponse(w http.ResponseWriter, resp map[string]interface{}) {
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Warningf("Failed to encode response JSON: %s", err)
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

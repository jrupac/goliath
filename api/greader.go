package api

import (
	"encoding/json"
	"fmt"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/goliath/utils"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"
)

const (
	readingListStreamId string = "user/-/state/com.google/reading-list"
	readStreamId        string = "user/-/state/com.google/read"
	unreadStreamId      string = "user/-/state/com.google/kept-unread"
	starredStreamId     string = "user/-/state/com.google/starred"
	broadcastStreamId   string = "user/-/state/com.google/broadcast"
)

var (
	greaderLatencyMetric = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "greader_server_latency",
			Help:       "Server-side latency of GReader API operations.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"method"},
	)
)

func init() {
	prometheus.MustRegister(greaderLatencyMetric)
}

// GReader is an implementation of the GReader API.
type GReader struct {
	d *storage.Database
}

// GReaderHandler returns a new GReader handler.
func GReaderHandler(d *storage.Database) http.HandlerFunc {
	return GReader{d}.Handler()
}

// Handler returns a handler function that implements the GReader API.
func (a GReader) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.route(w, r)
	}
}

func (a GReader) recordLatency(t time.Time, label string) {
	utils.Elapsed(t, func(d time.Duration) {
		// Record latency measurements in microseconds.
		greaderLatencyMetric.WithLabelValues(label).Observe(float64(d) / float64(time.Microsecond))
	})
}

func (a GReader) route(w http.ResponseWriter, r *http.Request) {
	// Record the total server latency of each call.
	defer a.recordLatency(time.Now(), "server")

	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/greader/accounts/ClientLogin":
		a.handleLogin(w, r)
	case "/greader/reader/api/0/token":
		a.withAuth(w, r, a.handlePostToken)
	case "/greader/reader/api/0/user-info":
		a.withAuth(w, r, a.handleUserInfo)
	case "/greader/reader/api/0/subscription/list":
		a.withAuth(w, r, a.handleSubscriptionList)
	case "/greader/reader/api/0/stream/items/ids":
		a.withAuth(w, r, a.handleStreamItemIds)
	case "/greader/reader/api/0/stream/items/contents":
		a.withAuth(w, r, a.handleStreamItemsContents)
	case "/greader/reader/api/0/edit-tag":
		a.withAuth(w, r, a.handleEditTag)
	default:
		log.Warningf("Got unexpected route: %s", r.URL.String())
		dump, err := httputil.DumpRequest(r, true)
		if err != nil {
			log.Warningf("Failed to dump request: %s", err)
		}
		log.Warningf("%q", dump)
		a.returnError(w, http.StatusBadRequest)
	}
}

func (a GReader) handleLogin(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		a.returnError(w, http.StatusBadRequest)
		return
	}

	formUser := r.Form.Get("Email")
	formPass := r.Form.Get("Passwd")

	user, err := a.d.GetUserByUsername(formUser)
	if err != nil {
		log.Warningf("Failed to find user: %s", formUser)
		a.returnError(w, http.StatusUnauthorized)
		return
	}

	ok := bcrypt.CompareHashAndPassword([]byte(user.HashPass), []byte(formPass))
	if ok == nil {
		token, err := createAuthToken(user.HashPass, formUser)
		if err != nil {
			a.returnError(w, http.StatusInternalServerError)
			return
		}
		a.returnSuccess(w, greaderHandlelogin{Auth: token})
	} else {
		a.returnError(w, http.StatusUnauthorized)
	}
}

func (a GReader) handleUserInfo(w http.ResponseWriter, _ *http.Request, user models.User) {
	a.returnSuccess(w, greaderUserInfo{
		UserId:   string(user.UserId),
		Username: user.Username,
	})
}

func (a GReader) handleSubscriptionList(w http.ResponseWriter, _ *http.Request, user models.User) {
	folders, err := a.d.GetAllFoldersForUser(user)
	if err != nil {
		a.returnError(w, http.StatusInternalServerError)
		return
	}

	feeds, err := a.d.GetAllFeedsForUser(user)
	if err != nil {
		a.returnError(w, http.StatusInternalServerError)
		return
	}

	faviconMap, err := a.d.GetAllFaviconsForUser(user)
	if err != nil {
		a.returnError(w, http.StatusInternalServerError)
		return
	}

	folderMap := map[int64]string{}
	for _, folder := range folders {
		folderMap[folder.ID] = folder.Name
	}

	subList := greaderSubscriptionList{}

	for _, feed := range feeds {
		subList.Subscriptions = append(subList.Subscriptions, greaderSubscription{
			Title: feed.Title,
			// No client seems to use this field, so let it as zero
			FirstItemMsec: "0",
			HtmlUrl:       feed.Link,
			IconUrl:       fmt.Sprintf("data:%s", faviconMap[feed.ID]),
			SortId:        feed.Title,
			Id:            greaderFeedId(feed.ID),
			Categories: []greaderCategory{{
				Id:    greaderFolderId(feed.FolderID),
				Label: folderMap[feed.FolderID],
			}},
		})
	}

	a.returnSuccess(w, subList)
}

func (a GReader) handleStreamItemIds(w http.ResponseWriter, r *http.Request, user models.User) {
	err := r.ParseForm()
	if err != nil {
		a.returnError(w, http.StatusBadRequest)
		return
	}

	limit, err := strconv.Atoi(r.Form.Get("n"))
	if err != nil {
		log.Warningf(
			"Saw unexpected 'n' parameter, defaulting to 10,000: %s", r.PostForm.Get("n"))
		limit = 10000
	}

	switch s := r.Form.Get("s"); s {
	case starredStreamId:
		// TODO: Support starred items
		a.returnSuccess(w, greaderStreamItemIds{})
		return
	case readStreamId:
		// Never return read items to the client, it's just simpler
		a.returnSuccess(w, greaderStreamItemIds{})
		return
	case readingListStreamId:
		// Handled below
		break
	default:
		log.Warningf("Saw unexpected 's' parameter: %s", s)
		a.returnError(w, http.StatusNotImplemented)
		return
	}

	xt := r.Form.Get("xt")
	if xt != readStreamId {
		// Only support excluding read items
		log.Warningf("Saw unexpected 'xt' parameter: %s", xt)
		a.returnError(w, http.StatusNotImplemented)
		return
	}

	// TODO: Support continuation tokens
	articles, err := a.d.GetUnreadArticleMetaForUser(user, limit, -1)
	if err != nil {
		a.returnError(w, http.StatusInternalServerError)
		return
	}

	streamItemIds := greaderStreamItemIds{}

	for _, article := range articles {
		streamItemIds.ItemRefs = append(streamItemIds.ItemRefs, greaderItemRef{
			Id: strconv.FormatInt(article.ID, 10),
			DirectStreamIds: []string{
				greaderFeedId(article.FeedID),
				greaderFolderId(article.FolderID),
			},
			TimestampUsec: strconv.FormatInt(article.Date.UnixMicro(), 10),
		})
	}

	a.returnSuccess(w, streamItemIds)
}

func (a GReader) handlePostToken(w http.ResponseWriter, _ *http.Request, _ models.User) {
	_, _ = fmt.Fprint(w, createPostToken())
	a.returnSuccess(w, nil)
}

func (a GReader) handleStreamItemsContents(w http.ResponseWriter, r *http.Request, user models.User) {
	err := r.ParseForm()
	if err != nil {
		a.returnError(w, http.StatusBadRequest)
		return
	}

	if !validatePostToken(r.Form.Get("T")) {
		a.returnError(w, http.StatusUnauthorized)
		return
	}

	articleIdsValue := r.Form["i"]
	var articleIds []int64
	for _, articleIdStr := range articleIdsValue {
		id, err := strconv.ParseInt(articleIdStr, 16, 64)
		if err != nil {
			a.returnError(w, http.StatusInternalServerError)
			return
		}
		articleIds = append(articleIds, id)
	}

	articles, err := a.d.GetArticlesForUser(user, articleIds)
	if err != nil {
		a.returnError(w, http.StatusInternalServerError)
		return
	}

	streamItemContents := greaderStreamItemsContents{
		Id:      readingListStreamId,
		Updated: time.Now().Unix(),
	}

	for _, article := range articles {
		streamItemContents.Items = append(streamItemContents.Items, greaderItemContent{
			CrawlTimeMsec: strconv.FormatInt(article.Date.UnixMilli(), 10),
			TimestampUsec: strconv.FormatInt(article.Date.UnixMicro(), 10),
			Id:            greaderArticleId(article.ID),
			Categories: []string{
				readingListStreamId,
				greaderFeedId(article.FeedID),
				greaderFolderId(article.FolderID),
			},
			Title:     article.Title,
			Published: article.Date.Unix(),
			Canonical: []greaderCanonical{
				{Href: article.Link},
			},
			Alternate: []greaderCanonical{
				{Href: article.Link},
			},
			Summary: greaderContent{
				Content: article.GetContents(*serveParsedArticles),
			},
			Origin: greaderOrigin{
				StreamId: greaderFeedId(article.FeedID),
			},
		})
	}

	a.returnSuccess(w, streamItemContents)
}

func (a GReader) handleEditTag(w http.ResponseWriter, r *http.Request, user models.User) {
	err := r.ParseForm()
	if err != nil {
		a.returnError(w, http.StatusBadRequest)
		return
	}

	if !validatePostToken(r.Form.Get("T")) {
		a.returnError(w, http.StatusUnauthorized)
		return
	}

	articleIdsValue := r.Form["i"]
	var articleIds []int64
	for _, articleIdStr := range articleIdsValue {
		id, err := strconv.ParseInt(articleIdStr, 16, 64)
		if err != nil {
			a.returnError(w, http.StatusInternalServerError)
			return
		}
		articleIds = append(articleIds, id)
	}

	var status string
	// Only support updating one tag
	switch r.Form.Get("a") {
	case readStreamId:
		status = "read"
	case unreadStreamId:
		status = "unread"
	case starredStreamId, broadcastStreamId:
		// TODO: Support starring items
		a.returnError(w, http.StatusNotImplemented)
		return
	}

	for _, articleId := range articleIds {
		err = a.d.MarkArticleForUser(user, articleId, status)
		if err != nil {
			a.returnError(w, http.StatusInternalServerError)
			return
		}
	}

	_, _ = w.Write([]byte("OK"))
	a.returnSuccess(w, nil)
}

func (a GReader) withAuth(w http.ResponseWriter, r *http.Request, handler func(http.ResponseWriter, *http.Request, models.User)) {
	// Header should be in format:
	//   Authorization: GoogleLogin auth=<token>
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		a.returnError(w, http.StatusUnauthorized)
		return
	}

	authFields := strings.Fields(authHeader)
	if len(authFields) != 2 || !strings.EqualFold(authFields[0], "GoogleLogin") {
		a.returnError(w, http.StatusBadRequest)
		return
	}

	authStr, tokenStr, found := strings.Cut(authFields[1], "=")
	if !found {
		a.returnError(w, http.StatusBadRequest)
		return
	}

	if !strings.EqualFold(authStr, "auth") {
		a.returnError(w, http.StatusBadRequest)
		return
	}

	username, token, err := extractAuthToken(tokenStr)
	if err != nil {
		a.returnError(w, http.StatusBadRequest)
		return
	}

	user, err := a.d.GetUserByUsername(username)
	if err != nil {
		a.returnError(w, http.StatusUnauthorized)
		return
	}

	if validateAuthToken(token, username, user.HashPass) {
		handler(w, r, user)
	} else {
		a.returnError(w, http.StatusUnauthorized)
	}
}

func greaderArticleId(articleId int64) string {
	return fmt.Sprintf("tag:google.com,2005:reader/item/%x", articleId)
}

func greaderFeedId(feedId int64) string {
	return fmt.Sprintf("feed/%d", feedId)
}

func greaderFolderId(folderId int64) string {
	return fmt.Sprintf("user/-/label/%d", folderId)
}

func (a GReader) returnError(w http.ResponseWriter, status int) {
	w.WriteHeader(status)
}

func (a GReader) returnSuccess(w http.ResponseWriter, resp any) {
	w.WriteHeader(http.StatusOK)

	if resp != nil {
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(resp); err != nil {
			a.returnError(w, http.StatusInternalServerError)
		}
	}
}

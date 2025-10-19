package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/goliath/utils"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/crypto/bcrypt"
)

const (
	readingListStreamId    string = "user/-/state/com.google/reading-list"
	readStreamId           string = "user/-/state/com.google/read"
	unreadStreamId         string = "user/-/state/com.google/kept-unread"
	starredStreamId        string = "user/-/state/com.google/starred"
	broadcastStreamId      string = "user/-/state/com.google/broadcast"
	invalidPostTokenHeader string = "X-Reader-Google-Bad-Token"
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
	d storage.Database
}

// GReaderHandler returns a new GReader handler.
func GReaderHandler(d storage.Database) http.HandlerFunc {
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

func (a GReader) preprocessRequest(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Warningf("Failed to parse request form: %s", err)
		a.returnError(w, http.StatusBadRequest)
		return
	}

	log.Infof("GReader request URL: %s", r.URL.String())
	log.Infof("Greader request method: %s", r.Method)

	contentType := r.Header.Get("Content-Type")

	if contentType == "application/x-www-form-urlencoded" {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else if strings.HasPrefix(contentType, "multipart/form-data") {
		err := r.ParseMultipartForm(10 << 20) // 10 MB limit
		if err != nil {
			log.Warningf("Failed to parse multipart form: %s", err)
			a.returnError(w, http.StatusBadRequest)
			return
		}
	}

	log.Infof("GReader request form: %+v", r.PostForm.Encode())
}

func (a GReader) route(w http.ResponseWriter, r *http.Request) {
	// Record the total server latency of each call.
	defer a.recordLatency(time.Now(), "server")

	a.preprocessRequest(w, r)

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
	case "/greader/reader/api/0/mark-all-as-read":
		a.withAuth(w, r, a.markAllAsRead)
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
	token, status := a.validateLoginForm(r)
	if status != http.StatusOK {
		a.returnError(w, status)
		return
	}
	a.returnSuccess(w, greaderHandlelogin{Auth: token})
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
		// Return an empty string if the favicon is not found
		var iconUrl string
		if favicon, ok := faviconMap[feed.ID]; ok {
			iconUrl = fmt.Sprintf("data:%s", favicon)
		}

		subList.Subscriptions = append(subList.Subscriptions, greaderSubscription{
			Title: feed.Title,
			// No client seems to use this field, so let it as zero
			FirstItemMsec: "0",
			HtmlUrl:       feed.Link,
			IconUrl:       iconUrl,
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

	sinceId := int64(-1)
	if c := r.Form.Get("c"); c != "" {
		// Note: This is parsing the continuation token as hex.
		sinceId, err = strconv.ParseInt(c, 16, 64)
		if err != nil {
			log.Warningf("Invalid continuation token: %s", c)
			a.returnError(w, http.StatusBadRequest)
			return
		}
	}

	if xt := r.Form.Get("xt"); xt != "" {
		if xt != readStreamId {
			log.Warningf("Saw unexpected 'xt' parameter: %s", xt)
			a.returnError(w, http.StatusNotImplemented)
			return
		}
	}

	if ot := r.Form.Get("ot"); ot != "" {
		if sinceId != -1 {
			log.Warningf("Saw unexpected 'ot' parameter with 'c' parameter already set: %s", ot)
		}

		// Note: This is parsing the "ot" token as decimal.
		sinceId, err = strconv.ParseInt(ot, 10, 64)
		if err != nil {
			log.Warningf("Invalid continuation token: %s", ot)
			a.returnError(w, http.StatusBadRequest)
			return
		}
	}
	if nt := r.Form.Get("nt"); nt != "" {
		log.Warningf("Saw unexpected 'nt' parameter: %s", nt)
		a.returnError(w, http.StatusNotImplemented)
		return
	}

	var articles []models.ArticleMeta

	s := r.Form.Get("s")
	switch s {
	case starredStreamId:
		articles, err = a.d.GetArticleMetaWithFilterForUser(user, models.StreamFilterSaved, limit, sinceId)
		if err != nil {
			a.returnError(w, http.StatusInternalServerError)
			return
		}
	case readingListStreamId:
		articles, err = a.d.GetArticleMetaWithFilterForUser(user, models.StreamFilterUnread, limit, sinceId)
		if err != nil {
			a.returnError(w, http.StatusInternalServerError)
			return
		}
	case readStreamId:
		// Never return read items to the client, it's just simpler
		// Only support excluding read items
		a.returnSuccess(w, greaderStreamItemIds{})
		return
	default:
		log.Warningf("Saw unexpected 's' parameter: %s", s)
		a.returnError(w, http.StatusNotImplemented)
		return
	}

	streamItemIds := greaderStreamItemIds{}
	contToken := int64(0)

	for _, article := range articles {
		streamItemIds.ItemRefs = append(streamItemIds.ItemRefs, greaderItemRef{
			// Note: This is writing the article ID as decimal in this one case.
			Id: strconv.FormatInt(article.ID, 10),
			DirectStreamIds: []string{
				greaderFeedId(article.FeedID),
				greaderFolderId(article.FolderID),
			},
			TimestampUsec: strconv.FormatInt(article.Date.UnixMicro(), 10),
		})

		if article.ID > contToken {
			contToken = article.ID
		}
	}

	// If we may have more article IDs remaining, set a continuation token.
	if len(articles) == limit && contToken > 0 {
		// Note: This is writing the continuation token as hex.
		streamItemIds.Continuation = fmt.Sprintf("%x", contToken)
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

	postToken := r.Form.Get("T")
	if !validatePostToken(postToken) {
		a.returnInvalidPostToken(w, postToken)
		return
	}

	articleIdsValue := r.Form["i"]
	var articleIds []int64
	for _, articleIdStr := range articleIdsValue {
		// Note: This is parsing the article ID as hex.
		id, err := strconv.ParseInt(articleIdStr, 16, 64)
		if err != nil {
			log.Warningf("Invalid article ID: %s", err)
			a.returnError(w, http.StatusInternalServerError)
			return
		}
		articleIds = append(articleIds, id)
	}

	articles, err := a.d.GetArticlesForUser(user, articleIds)
	if err != nil {
		log.Warningf("Failed to get articles: %v", err)
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

	postToken := r.Form.Get("T")
	if !validatePostToken(postToken) {
		a.returnInvalidPostToken(w, postToken)
		return
	}

	articleIdsValue := r.Form["i"]
	var articleIds []int64
	for _, articleIdStr := range articleIdsValue {
		// Note: This is parsing the article ID as hex.
		id, err := strconv.ParseInt(articleIdStr, 16, 64)
		if err != nil {
			log.Warningf("Invalid article ID: %s", err)
			a.returnError(w, http.StatusBadRequest)
			return
		}
		articleIds = append(articleIds, id)
	}

	mark := models.MarkActionUnknown

	// The "a" key refers to tags that are added
	switch r.Form.Get("a") {
	case "":
		// Can be empty, nothing to do
		break
	case readStreamId:
		mark = models.MarkActionRead
	case unreadStreamId:
		mark = models.MarkActionUnread
	case starredStreamId:
		mark = models.MarkActionSaved
	case broadcastStreamId:
		log.Warningf("Got unexpected 'a' parameter: %s", r.Form.Get("a"))
		a.returnError(w, http.StatusNotImplemented)
		return
	}

	// The "r" key refers to tags that are removed
	// Note: This is processed after the "a" key, so it takes precedence.
	switch r.Form.Get("r") {
	case "":
		// Can be empty, nothing to do
		break
	case readStreamId:
		mark = models.MarkActionUnread
	case unreadStreamId:
		mark = models.MarkActionRead
	case starredStreamId:
		mark = models.MarkActionUnsaved
	case broadcastStreamId:
		log.Warningf("Got unexpected 'r' parameter: %s", r.Form.Get("r"))
		a.returnError(w, http.StatusNotImplemented)
		return
	}

	if mark == models.MarkActionUnknown {
		log.Warningf("Did not specify either 'a' or 'r' parameter")
		a.returnError(w, http.StatusBadRequest)
		return
	}

	for _, articleId := range articleIds {
		err = a.d.MarkArticleForUser(user, articleId, mark)
		if err != nil {
			log.Warningf("Failed to mark article %d: %s", articleId, err)
			a.returnError(w, http.StatusInternalServerError)
			return
		}
	}

	_, _ = w.Write([]byte("OK"))
	a.returnSuccess(w, nil)
}

func (a GReader) markAllAsRead(w http.ResponseWriter, r *http.Request, user models.User) {
	err := r.ParseForm()
	if err != nil {
		log.Warningf("Failed to parse request: %s", err)
		a.returnError(w, http.StatusBadRequest)
		return
	}

	postToken := r.Form.Get("T")
	if !validatePostToken(postToken) {
		a.returnInvalidPostToken(w, postToken)
		return
	}

	// This method is only for making feeds and folders as read. Only articles
	// can be marked as unread, using the "edit tag" method.
	if folderStr := r.Form.Get("t"); folderStr != "" {
		folderId, err := strconv.ParseInt(folderStr, 10, 64)
		if err != nil {
			log.Warningf("Invalid folder ID: %s", folderStr)
			a.returnError(w, http.StatusInternalServerError)
			return
		}

		err = a.d.MarkFolderForUser(user, folderId, models.MarkActionRead)
		if err != nil {
			log.Warningf("Failed to mark folder: %s", folderStr)
			a.returnError(w, http.StatusInternalServerError)
			return
		}
	} else if feedStr := r.Form.Get("s"); feedStr != "" {
		feedId, err := strconv.ParseInt(feedStr, 10, 64)
		if err != nil {
			log.Warningf("Invalid feed ID: %s", feedStr)
			a.returnError(w, http.StatusInternalServerError)
			return
		}

		err = a.d.MarkFeedForUser(user, feedId, models.MarkActionRead)
		if err != nil {
			log.Warningf("Failed to mark feed: %d", feedId)
			a.returnError(w, http.StatusInternalServerError)
			return
		}
	} else {
		log.Warningf("Missing feed or folder ID")
		a.returnError(w, http.StatusBadRequest)
		return
	}

	_, _ = w.Write([]byte("OK"))
	a.returnSuccess(w, nil)
}

func (a GReader) withAuth(w http.ResponseWriter, r *http.Request, handler func(http.ResponseWriter, *http.Request, models.User)) {
	// Header should be in format:
	//   Authorization: GoogleLogin auth=<token>
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		log.Warningf("Missing authorization header")
		a.returnError(w, http.StatusUnauthorized)
		return
	}

	authFields := strings.Fields(authHeader)
	if len(authFields) != 2 || !strings.EqualFold(authFields[0], "GoogleLogin") {
		log.Warningf("Invalid authorization header: %s", authHeader)
		a.returnError(w, http.StatusBadRequest)
		return
	}

	authStr, tokenStr, found := strings.Cut(authFields[1], "=")
	if !found {
		log.Warningf("Invalid authorization header: %s", authHeader)
		a.returnError(w, http.StatusBadRequest)
		return
	}

	if !strings.EqualFold(authStr, "auth") {
		log.Warningf("Invalid authorization header: %s", authHeader)
		a.returnError(w, http.StatusBadRequest)
		return
	}

	username, token, err := extractAuthToken(tokenStr)
	if err != nil {
		log.Warningf("Invalid authorization header: %s", authHeader)
		a.returnError(w, http.StatusBadRequest)
		return
	}

	user, err := a.d.GetUserByUsername(username)
	if err != nil {
		log.Warningf("Failed to find user: %s", username)
		a.returnError(w, http.StatusUnauthorized)
		return
	}

	if !validateAuthToken(token, username, user.HashPass) {
		log.Warningf("Invalid token for user: %s", username)
		a.returnError(w, http.StatusUnauthorized)
		return
	}

	handler(w, r, user)
}

func greaderArticleId(articleId int64) string {
	// Note: This is writing the article ID as hex.
	return fmt.Sprintf("tag:google.com,2005:reader/item/%x", articleId)
}

func greaderFeedId(feedId int64) string {
	return fmt.Sprintf("feed/%d", feedId)
}

func greaderFolderId(folderId int64) string {
	return fmt.Sprintf("user/-/label/%d", folderId)
}

func (a GReader) validateLoginForm(r *http.Request) (string, int) {
	token := ""

	formUser := r.Form.Get("Email")
	formPass := r.Form.Get("Passwd")

	user, err := a.d.GetUserByUsername(formUser)
	if err != nil {
		log.Warningf("Failed to find user: %s", formUser)
		return token, http.StatusUnauthorized
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.HashPass), []byte(formPass))
	if err != nil {
		log.Warningf("Failed to validate password: %v", err)
		return token, http.StatusUnauthorized
	}

	token, err = createAuthToken(user.HashPass, formUser)
	if err != nil {
		log.Warningf("Failed to create auth token: %v", err)
		return token, http.StatusInternalServerError
	}

	return token, http.StatusOK
}

func (a GReader) returnError(w http.ResponseWriter, status int) {
	w.WriteHeader(status)
}

func (a GReader) returnInvalidPostToken(w http.ResponseWriter, token string) {
	log.Warningf("Invalid post token: %s", token)
	w.WriteHeader(http.StatusUnauthorized)
	w.Header().Set(invalidPostTokenHeader, "true")
}

func (a GReader) returnSuccess(w http.ResponseWriter, resp any) {
	if resp != nil {
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(resp); err != nil {
			a.returnError(w, http.StatusInternalServerError)
		}
	}
}

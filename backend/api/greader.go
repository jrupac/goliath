package api

import (
	"cmp"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"slices"
	"strconv"
	"strings"
	"time"

	log "github.com/golang/glog"
	"github.com/jrupac/goliath/auth"
	"github.com/jrupac/goliath/fetch"
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

// preprocessRequest parses the request form and logs the request. It reports
// whether the request is well-formed enough to dispatch; the caller must not
// route a request for which this returns false, since the response status has
// already been written.
func (a GReader) preprocessRequest(w http.ResponseWriter, r *http.Request) bool {
	err := r.ParseForm()
	if err != nil {
		log.Warningf("Failed to parse request form: %s", err)
		a.returnError(w, http.StatusBadRequest)
		return false
	}

	log.Infof("GReader request URL: %s", r.URL.String())
	log.Infof("Greader request method: %s", r.Method)
	// The GReader API is implemented against observed client behavior rather
	// than a published specification, so which client sent a request is part
	// of interpreting it.
	log.Infof("GReader request user agent: %s", r.Header.Get("User-Agent"))

	contentType := r.Header.Get("Content-Type")

	if contentType == "application/x-www-form-urlencoded" {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return false
		}
	} else if strings.HasPrefix(contentType, "multipart/form-data") {
		err := r.ParseMultipartForm(10 << 20) // 10 MB limit
		if err != nil {
			log.Warningf("Failed to parse multipart form: %s", err)
			a.returnError(w, http.StatusBadRequest)
			return false
		}
	}

	log.Infof("GReader request form: %+v", redactFormValues(r.PostForm))
	return true
}

func (a GReader) route(w http.ResponseWriter, r *http.Request) {
	// Record the total server latency of each call.
	defer a.recordLatency(time.Now(), "server")

	if !a.preprocessRequest(w, r) {
		return
	}

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
	case "/greader/reader/api/0/subscription/quickadd":
		a.withAuth(w, r, a.handleQuickAdd)
	case "/greader/reader/api/0/subscription/edit":
		a.withAuth(w, r, a.handleSubscriptionEdit)
	case "/greader/reader/api/0/tag/list":
		a.withAuth(w, r, a.handleTagList)
	case "/greader/reader/api/0/rename-tag":
		a.withAuth(w, r, a.handleRenameTag)
	case "/greader/reader/api/0/disable-tag":
		a.withAuth(w, r, a.handleDisableTag)
	case "/greader/ext/parse-full-article":
		a.withAuth(w, r, a.handleParseFullArticle)
	default:
		log.Warningf("Got unexpected route: %s", r.URL.String())
		dump, err := dumpRequestRedacted(r)
		if err != nil {
			log.Warningf("Failed to dump request: %s", err)
		} else {
			log.Warningf("%q", dump)
		}
		// The dump above carries no body: ParseForm has already consumed it.
		// Log the parsed form separately, since for an unimplemented endpoint
		// this payload is the only record of what the client was asking for.
		log.Warningf("Unexpected route form: %+v", redactFormValues(r.PostForm))
		a.returnError(w, http.StatusBadRequest)
	}
}

// redactedHeaders are masked before a request is written to the log. The
// GReader auth token neither expires nor can be revoked short of a password
// change, so logging it in the clear grants indefinite account access to
// anyone who can read the logs.
var redactedHeaders = []string{"Authorization", "Cookie"}

// dumpRequestRedacted renders a request for logging with credential-bearing
// headers masked.
//
// The body is deliberately not included: callers reach this after ParseForm has
// already drained it, so DumpRequest would record an empty body and imply the
// request had none.
func dumpRequestRedacted(r *http.Request) ([]byte, error) {
	original := r.Header
	safe := original.Clone()
	if safe == nil {
		safe = http.Header{}
	}
	for _, h := range redactedHeaders {
		if len(safe.Values(h)) > 0 {
			safe.Set(h, redactedValue)
		}
	}

	r.Header = safe
	defer func() { r.Header = original }()

	return httputil.DumpRequest(r, false)
}

func (a GReader) handleLogin(w http.ResponseWriter, r *http.Request) {
	token, status := a.validateLoginForm(r)
	if status != http.StatusOK {
		a.returnError(w, status)
		return
	}
	// Revealed at the boundary: handing the token to the client is the point
	// of this response, and it is the only time the value is recoverable.
	a.returnSuccess(w, greaderHandlelogin{Auth: token.Reveal()})
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

	// The folder holding feeds that are in none of the user's own is stored
	// under a sentinel name, which a client shown it draws as a folder called
	// "<root>". Presented under a name meant for people instead.
	//
	// Canonically such a feed would carry no category at all and a client would
	// show it at the top level. That is the better answer and not this one: the
	// web client keys a feed selection by (feed, folder) and marks a folder read
	// by ID, so a feed with no folder is a change to its selection model rather
	// than to this response.
	if root, err := a.d.GetRootFolderForUser(user); err == nil {
		folderMap[root.ID] = models.RootFolderDisplayName
	} else if !errors.Is(err, sql.ErrNoRows) {
		a.returnError(w, http.StatusInternalServerError)
		return
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
			Url:           feed.URL,
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

	// "n" is what the client would like, not what it gets: it is clamped to the
	// ceiling a single read returns, so that asking for an arbitrarily large
	// page cannot turn into asking the database for one.
	limit := storage.MaxFetchedRows
	if n := r.Form.Get("n"); n != "" {
		requested, err := strconv.Atoi(n)
		if err != nil || requested <= 0 {
			log.Warningf("Saw unusable 'n' parameter, defaulting to %d: %s", limit, n)
		} else {
			limit = min(requested, limit)
		}
	}

	// "c" pages through a result set and "ot" narrows it to recent items. They
	// are independent bounds and clients send them together.
	cursor := models.StreamCursor{}
	if c := r.Form.Get("c"); c != "" {
		// Note: This is parsing the continuation token as hex.
		cursor.SinceID, err = strconv.ParseInt(c, 16, 64)
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
		// "ot" is a Unix timestamp in seconds, bounding how far back the
		// client wants to look, not an item ID.
		seconds, err := strconv.ParseInt(ot, 10, 64)
		if err != nil {
			log.Warningf("Invalid 'ot' timestamp: %s", ot)
			a.returnError(w, http.StatusBadRequest)
			return
		}
		cursor.Since = time.Unix(seconds, 0)
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
		articles, err = a.d.GetArticleMetaWithFilterForUser(user, models.StreamFilterSaved, limit, cursor)
		if err != nil {
			a.returnError(w, http.StatusInternalServerError)
			return
		}
	case readingListStreamId:
		articles, err = a.d.GetArticleMetaWithFilterForUser(user, models.StreamFilterUnread, limit, cursor)
		if err != nil {
			a.returnError(w, http.StatusInternalServerError)
			return
		}
	case readStreamId:
		// Clients poll this stream to reconcile read state set elsewhere,
		// bounding it with "ot" so that the answer stays proportional to how
		// long they have been away.
		articles, err = a.d.GetArticleMetaWithFilterForUser(user, models.StreamFilterRead, limit, cursor)
		if err != nil {
			a.returnError(w, http.StatusInternalServerError)
			return
		}
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

func (a GReader) handlePostToken(w http.ResponseWriter, r *http.Request, user models.User) {
	token := createPostToken(postTokenBinding(user, sessionFrom(r.Context())))
	// Revealed at the boundary: handing the token to the client is what this
	// endpoint is for. Printing it without this writes the redaction placeholder
	// into the response body instead.
	_, _ = fmt.Fprint(w, token.Reveal())
}

// checkPostToken validates the post token on a mutating request, writing the
// error response and reporting false if it does not hold up.
func (a GReader) checkPostToken(w http.ResponseWriter, r *http.Request, user models.User) bool {
	token := models.Secret(r.Form.Get(postTokenParam))
	if !validatePostToken(postTokenBinding(user, sessionFrom(r.Context())), token) {
		a.returnInvalidPostToken(w, token)
		return false
	}
	return true
}

func (a GReader) handleStreamItemsContents(w http.ResponseWriter, r *http.Request, user models.User) {
	err := r.ParseForm()
	if err != nil {
		a.returnError(w, http.StatusBadRequest)
		return
	}

	if !a.checkPostToken(w, r, user) {
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

	if !a.checkPostToken(w, r, user) {
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

	// Marked in a single statement rather than one per ID. Clients batch
	// hundreds of IDs into one request here, so a per-ID loop costs that many
	// sequential round trips and can fail halfway, leaving the batch partly
	// applied with no way for the client to learn which IDs landed.
	n, err := a.d.MarkArticlesForUser(user, articleIds, mark)
	if err != nil {
		log.Warningf("Failed to mark %d articles: %s", len(articleIds), err)
		a.returnError(w, http.StatusInternalServerError)
		return
	}

	switch mark {
	case models.MarkActionRead:
		hour, day := readActivityLabels()
		articlesMarkedReadMetric.WithLabelValues(user.Username, "individual", hour, day).Add(float64(n))
	case models.MarkActionSaved:
		articlesSavedMetric.WithLabelValues(user.Username).Add(float64(n))
	}

	_, _ = w.Write([]byte("OK"))
}

// handleQuickAdd subscribes the user to a feed named by URL.
//
// The URL is fetched and parsed before anything is stored. A client adding a
// feed sends only its address, so the title and site link have to come from
// the feed itself; and a URL that is not a feed would otherwise become a
// subscription that fails on every cycle, with nothing recording that it never
// worked in the first place.
func (a GReader) handleQuickAdd(w http.ResponseWriter, r *http.Request, user models.User) {
	if !a.checkPostToken(w, r, user) {
		return
	}

	// Clients send this in both the query string and the body; r.Form merges
	// the two, so either source answers.
	feedURL := strings.TrimSpace(r.Form.Get("quickadd"))
	if feedURL == "" {
		log.Warningf("Missing 'quickadd' parameter")
		a.returnError(w, http.StatusBadRequest)
		return
	}

	// Adding a feed already subscribed to reports the existing subscription
	// rather than creating a second one. The insert would not collide: its
	// conflict key covers the title, which the feed is free to change between
	// one add and the next.
	switch existing, err := a.d.GetFeedByUrlForUser(user, feedURL); {
	case err == nil:
		log.Infof("Feed %s is already subscribed as %d", feedURL, existing.ID)
		a.returnSuccess(w, greaderQuickAddResponse{
			Query:      feedURL,
			NumResults: 1,
			StreamId:   greaderFeedId(existing.ID),
			StreamName: existing.Title,
		})
		return
	case !errors.Is(err, sql.ErrNoRows):
		log.Warningf("Failed to look up feed %s: %s", feedURL, err)
		a.returnError(w, http.StatusInternalServerError)
		return
	}

	discovered, err := fetch.DiscoverFeed(feedURL)
	if err != nil {
		// The client's URL is what is wrong, so this is not a server error.
		log.Warningf("Refusing to subscribe to %s: %s", feedURL, err)
		a.returnError(w, http.StatusBadRequest)
		return
	}

	// Paused across the write so that the fetcher rereads the feed list and
	// picks up the new subscription, rather than ignoring it until a restart.
	fetch.Pause()
	defer fetch.Resume()

	// Folder 0 means the root folder, which the storage layer resolves. No
	// client sends a folder when adding, and the root is where an unfiled
	// subscription belongs.
	feedID, err := a.d.InsertFeedForUser(user, discovered, 0)
	if err != nil {
		log.Warningf("Failed to add feed %s: %s", feedURL, err)
		a.returnError(w, http.StatusInternalServerError)
		return
	}

	log.Infof("Added feed %d (%q) at %s for user %s", feedID, discovered.Title, feedURL, user.Username)
	a.returnSuccess(w, greaderQuickAddResponse{
		Query:      feedURL,
		NumResults: 1,
		StreamId:   greaderFeedId(feedID),
		StreamName: discovered.Title,
	})
}

// handleSubscriptionEdit renames a feed, moves it between folders, or removes
// it, selected by `ac`.
func (a GReader) handleSubscriptionEdit(w http.ResponseWriter, r *http.Request, user models.User) {
	if !a.checkPostToken(w, r, user) {
		return
	}

	feedId, err := parseFeedId(r.Form.Get("s"))
	if err != nil {
		log.Warningf("Invalid feed ID: %s", r.Form.Get("s"))
		a.returnError(w, http.StatusBadRequest)
		return
	}

	// Every operation here changes or destroys a subscription, so the feed is
	// read back under the requester's own ID. Ownership is part of that lookup,
	// so a feed belonging to somebody else is indistinguishable from one that
	// does not exist.
	feed, err := a.d.GetFeedForUser(user, feedId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Warningf("User %s asked to edit feed %d, which is not theirs", user.Username, feedId)
			a.returnError(w, http.StatusNotFound)
			return
		}
		log.Warningf("Failed to look up feed %d: %s", feedId, err)
		a.returnError(w, http.StatusInternalServerError)
		return
	}

	switch ac := r.Form.Get("ac"); ac {
	case "edit":
		a.editSubscription(w, r, user, feed)
	case "unsubscribe":
		a.unsubscribeFeed(w, r, user, feed)
	default:
		log.Warningf("Saw unexpected 'ac' parameter: %s", ac)
		a.returnError(w, http.StatusNotImplemented)
	}
}

// editSubscription applies a rename, a move between folders, or both.
//
// Clients send one `ac=edit` for either, distinguished only by which of `t`
// and `a` are present, so both are applied when both are given rather than one
// being treated as the real intent.
func (a GReader) editSubscription(w http.ResponseWriter, r *http.Request, user models.User, feed models.Feed) {
	title := r.Form.Get("t")
	addLabel := r.Form.Get("a")
	removeLabel := r.Form.Get("r")

	if title == "" && addLabel == "" && removeLabel == "" {
		// Nothing to change is not an error. Clients send a bare `ac=edit`
		// naming only the feed as a confirmation step after adding one, and
		// treat a refusal as the add itself having failed -- which they undo by
		// unsubscribing, destroying the feed they just created.
		log.Infof("Edit of feed %d changes nothing", feed.ID)
		_, _ = w.Write([]byte("OK"))
		return
	}

	// The move is applied first. It is the change that can fail on a folder the
	// user does not own, and applying it before the rename means a refusal
	// leaves the feed entirely untouched.
	if addLabel != "" || removeLabel != "" {
		folderId, ok := a.destinationFolder(w, user, feed, addLabel, removeLabel)
		if !ok {
			return
		}
		if folderId != feed.FolderID {
			if err := a.moveFeedToFolder(user, feed.ID, folderId); err != nil {
				log.Warningf("Failed to move feed %d to folder %d: %s", feed.ID, folderId, err)
				a.returnError(w, http.StatusInternalServerError)
				return
			}
			log.Infof("Moved feed %d from folder %d to %d for user %s",
				feed.ID, feed.FolderID, folderId, user.Username)
			feed.FolderID = folderId
		}
	}

	if title != "" {
		feed.Title = title
		// Marked so that fetching leaves it alone. A feed's own metadata is
		// refreshed on every first fetch, which would otherwise undo this the
		// next time the fetcher restarted.
		feed.TitleOverridden = true
		if err := a.d.UpdateFeedMetadataForUser(user, feed); err != nil {
			log.Warningf("Failed to rename feed %d: %s", feed.ID, err)
			a.returnError(w, http.StatusInternalServerError)
			return
		}
		log.Infof("Renamed feed %d to %q for user %s", feed.ID, title, user.Username)
	}

	_, _ = w.Write([]byte("OK"))
}

// destinationFolder works out which folder an edit is asking a feed to end up
// in, writing the error response and reporting false if it cannot.
//
// `a` adds a label and `r` removes one, mirroring how edit-tag names tags. A
// feed here has exactly one folder, so an `a` decides the destination outright
// and `r` is only worth checking for disagreement. An `r` on its own is a
// client saying the feed should no longer be filed anywhere, which is the root
// folder: the place a feed lives when it is in none of the user's own.
func (a GReader) destinationFolder(
	w http.ResponseWriter, user models.User, feed models.Feed, addLabel, removeLabel string) (int64, bool) {

	if addLabel == "" {
		if from, err := parseFolderId(removeLabel); err != nil {
			log.Warningf("Invalid folder ID: %s", removeLabel)
			a.returnError(w, http.StatusBadRequest)
			return 0, false
		} else if from != feed.FolderID {
			log.Warningf("Client removed feed %d from folder %d, but it is in %d",
				feed.ID, from, feed.FolderID)
		}
		root, err := a.d.GetRootFolderForUser(user)
		if err != nil {
			log.Warningf("Failed to look up the root folder for user %s: %s", user.Username, err)
			a.returnError(w, http.StatusInternalServerError)
			return 0, false
		}
		return root.ID, true
	}

	if removeLabel != "" {
		if from, err := parseFolderId(removeLabel); err == nil && from != feed.FolderID {
			log.Warningf("Client moved feed %d from folder %d, but it is in %d",
				feed.ID, from, feed.FolderID)
		}
	}
	// If the desination folder does not exist, this is the path by which it is created.
	folder, ok := a.folderForLabel(w, user, addLabel, true)
	if !ok {
		return 0, false
	}
	return folder.ID, true
}

// folderForLabel finds the user's folder a label names, writing the error
// response and reporting false if it cannot. With create set, a name that
// matches none of the user's folders makes one, filed under the root.
//
// A label is `user/-/label/<x>` or a bare <x>. An <x> that reads as a number is
// a folder ID, which is what this server hands out; anything else is a folder
// name, which is what other servers hand out and what a client sends when it
// is making a folder up. A folder whose name is itself a number is therefore
// reachable only by its ID. The root's display name resolves to the root, so
// that filing a feed under the category a client was shown puts it back where
// it came from.
func (a GReader) folderForLabel(
	w http.ResponseWriter, user models.User, label string, create bool) (models.Folder, bool) {

	tail, isLabel := strings.CutPrefix(label, folderStreamPrefix)
	if !isLabel && strings.Contains(label, "/") {
		// Another kind of stream ID, such as a state. Taken as a name it would
		// make a folder called "user/-/state/...".
		log.Warningf("Not a folder label: %q", label)
		a.returnError(w, http.StatusBadRequest)
		return models.Folder{}, false
	}

	if id, err := strconv.ParseInt(tail, 10, 64); err == nil {
		folder, err := a.d.GetFolderForUser(user, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				log.Warningf("User %s named folder %d, which is not theirs", user.Username, id)
				a.returnError(w, http.StatusNotFound)
				return models.Folder{}, false
			}
			log.Warningf("Failed to look up folder %d: %s", id, err)
			a.returnError(w, http.StatusInternalServerError)
			return models.Folder{}, false
		}
		return folder, true
	}

	name := strings.TrimSpace(tail)
	if name == "" {
		log.Warningf("Empty folder label: %q", label)
		a.returnError(w, http.StatusBadRequest)
		return models.Folder{}, false
	}

	folders, err := a.d.GetAllFoldersForUser(user)
	if err != nil {
		log.Warningf("Failed to list folders for user %s: %s", user.Username, err)
		a.returnError(w, http.StatusInternalServerError)
		return models.Folder{}, false
	}
	isRoot := strings.EqualFold(name, models.RootFolderDisplayName)
	for _, f := range folders {
		if (isRoot && f.Name == models.RootFolder) || (!isRoot && f.Name == name) {
			return f, true
		}
	}

	if !create || isRoot {
		log.Warningf("User %s named folder %q, which does not exist", user.Username, name)
		a.returnError(w, http.StatusNotFound)
		return models.Folder{}, false
	}

	id, err := a.d.InsertFolderForUser(user, models.Folder{Name: name}, 0)
	if err != nil {
		log.Warningf("Failed to create folder %q for user %s: %s", name, user.Username, err)
		a.returnError(w, http.StatusInternalServerError)
		return models.Folder{}, false
	}
	log.Infof("Created folder %d (%q) for user %s", id, name, user.Username)
	return models.Folder{ID: id, Name: name}, true
}

// moveFeedToFolder repoints a feed at another folder.
//
// Fetching is paused across the change. A feed's folder is part of the key its
// articles are stored under, so a fetch still holding the old one writes rows
// that no longer satisfy the foreign key: they are rejected, and the feed stays
// empty until something else makes the fetcher reread the list. Pausing settles
// whatever is in flight and guarantees that reread.
func (a GReader) moveFeedToFolder(user models.User, feedId, folderId int64) error {
	fetch.Pause()
	defer fetch.Resume()
	return a.d.UpdateFolderForFeedForUser(user, feedId, folderId)
}

// unsubscribeFeed deletes a feed and everything fetched into it.
//
// The articles go with it: Article's foreign key to Feed has no ON DELETE, so
// they cannot be left behind pointing at a feed that no longer exists. This is
// the only request a client can make that destroys stored content, so what was
// removed is logged in full -- afterwards the row is gone, and the log is the
// only remaining account of what it was.
func (a GReader) unsubscribeFeed(w http.ResponseWriter, r *http.Request, user models.User, feed models.Feed) {
	articles, err := a.d.GetArticlesForFeedForUser(user, feed.ID)
	if err != nil {
		log.Warningf("Failed to count articles in feed %d: %s", feed.ID, err)
		a.returnError(w, http.StatusInternalServerError)
		return
	}

	log.Warningf("Unsubscribing user %s from %s (%d articles), requested by %q",
		user.Username, feed, len(articles), r.Header.Get("User-Agent"))

	// Paused across the delete so the fetcher rereads the feed list and stops
	// fetching a feed that no longer exists.
	fetch.Pause()
	defer fetch.Resume()

	if err = a.d.DeleteFeedForUser(user, feed.ID, feed.FolderID); err != nil {
		log.Warningf("Failed to unsubscribe from feed %d: %s", feed.ID, err)
		a.returnError(w, http.StatusInternalServerError)
		return
	}

	log.Warningf("Unsubscribed user %s from feed %d", user.Username, feed.ID)
	_, _ = w.Write([]byte("OK"))
}

// handleTagList lists the user's folders, along with the state a client can
// file items under.
//
// The root is listed under its display name, as subscription/list presents
// it, so that every category a client sees on a feed is also a tag it can find
// here.
func (a GReader) handleTagList(w http.ResponseWriter, _ *http.Request, user models.User) {
	folders, err := a.d.GetAllFoldersForUser(user)
	if err != nil {
		log.Warningf("Failed to list folders for user %s: %s", user.Username, err)
		a.returnError(w, http.StatusInternalServerError)
		return
	}
	slices.SortFunc(folders, func(x, y models.Folder) int { return cmp.Compare(x.ID, y.ID) })

	tags := greaderTagList{Tags: []greaderTag{{Id: starredStreamId}}}
	for _, f := range folders {
		label := f.Name
		if f.Name == models.RootFolder {
			label = models.RootFolderDisplayName
		}
		tags.Tags = append(tags.Tags, greaderTag{Id: greaderFolderId(f.ID), Label: label, Type: "folder"})
	}
	a.returnSuccess(w, tags)
}

// handleRenameTag renames one of the user's folders.
//
// The folder is named by `s`, or by `t`, which some clients use instead; the
// new name is `dest`, which is always a name, whether as `user/-/label/<name>`
// or bare. A folder keeps its ID across a rename, so nothing filed under it
// moves and no ID a client holds stops working.
func (a GReader) handleRenameTag(w http.ResponseWriter, r *http.Request, user models.User) {
	if !a.checkPostToken(w, r, user) {
		return
	}

	source := r.Form.Get("s")
	if source == "" {
		source = r.Form.Get("t")
	}
	dest := r.Form.Get("dest")
	name, isLabel := strings.CutPrefix(dest, folderStreamPrefix)
	name = strings.TrimSpace(name)
	if source == "" || name == "" || (!isLabel && strings.Contains(dest, "/")) {
		log.Warningf("Invalid rename of folder %q to %q", source, dest)
		a.returnError(w, http.StatusBadRequest)
		return
	}

	folder, ok := a.folderForLabel(w, user, source, false)
	if !ok {
		return
	}
	if folder.Name == models.RootFolder {
		log.Warningf("Refusing to rename the root folder for user %s", user.Username)
		a.returnError(w, http.StatusBadRequest)
		return
	}
	if err := models.ValidateFolderName(name); err != nil {
		log.Warningf("Refusing to rename folder %d: %s", folder.ID, err)
		a.returnError(w, http.StatusBadRequest)
		return
	}

	switch err := a.d.RenameFolderForUser(user, folder.ID, name); {
	case err == nil:
	case errors.Is(err, storage.ErrFolderNameTaken):
		log.Warningf("Refusing to rename folder %d to %q, which another folder has", folder.ID, name)
		a.returnError(w, http.StatusConflict)
		return
	case errors.Is(err, sql.ErrNoRows):
		a.returnError(w, http.StatusNotFound)
		return
	default:
		log.Warningf("Failed to rename folder %d: %s", folder.ID, err)
		a.returnError(w, http.StatusInternalServerError)
		return
	}

	log.Infof("Renamed folder %d from %q to %q for user %s", folder.ID, folder.Name, name, user.Username)
	_, _ = w.Write([]byte("OK"))
}

// handleDisableTag removes one or more of the user's folders, moving the feeds
// in each to the root folder.
//
// Folders are named by `s`, which may repeat, or by `t`. Every one is resolved
// before any is removed, so that a request naming a folder the user does not
// own changes nothing, rather than removing whichever folders were listed
// before it.
func (a GReader) handleDisableTag(w http.ResponseWriter, r *http.Request, user models.User) {
	if !a.checkPostToken(w, r, user) {
		return
	}

	labels := slices.Concat(r.Form["s"], r.Form["t"])
	if len(labels) == 0 {
		log.Warningf("Missing folder to remove")
		a.returnError(w, http.StatusBadRequest)
		return
	}

	var folders []models.Folder
	for _, label := range labels {
		folder, ok := a.folderForLabel(w, user, label, false)
		if !ok {
			return
		}
		if folder.Name == models.RootFolder {
			log.Warningf("Refusing to remove the root folder for user %s", user.Username)
			a.returnError(w, http.StatusBadRequest)
			return
		}
		if !slices.ContainsFunc(folders, func(f models.Folder) bool { return f.ID == folder.ID }) {
			folders = append(folders, folder)
		}
	}

	// Paused across the change for the same reason as a move: the feeds leaving
	// each folder change the key their articles are stored under.
	fetch.Pause()
	defer fetch.Resume()

	for _, folder := range folders {
		moved, err := a.d.DeleteFolderForUser(user, folder.ID)
		if err != nil {
			log.Warningf("Failed to remove folder %d: %s", folder.ID, err)
			a.returnError(w, http.StatusInternalServerError)
			return
		}
		log.Warningf("Removed folder %d (%q) for user %s, moving %d feeds to the root, requested by %q",
			folder.ID, folder.Name, user.Username, moved, r.Header.Get("User-Agent"))
	}

	_, _ = w.Write([]byte("OK"))
}

func (a GReader) markAllAsRead(w http.ResponseWriter, r *http.Request, user models.User) {
	err := r.ParseForm()
	if err != nil {
		log.Warningf("Failed to parse request: %s", err)
		a.returnError(w, http.StatusBadRequest)
		return
	}

	if !a.checkPostToken(w, r, user) {
		return
	}

	// This method is only for making feeds and folders as read. Only articles
	// can be marked as unread, using the "edit tag" method.
	if folderStr := r.Form.Get("t"); folderStr != "" {
		folderId, err := parseFolderId(folderStr)
		if err != nil {
			log.Warningf("Invalid folder ID: %s", folderStr)
			a.returnError(w, http.StatusBadRequest)
			return
		}

		n, err := a.d.MarkFolderForUser(user, folderId, models.MarkActionRead)
		if err != nil {
			log.Warningf("Failed to mark folder: %s", folderStr)
			a.returnError(w, http.StatusInternalServerError)
			return
		}
		hour, day := readActivityLabels()
		articlesMarkedReadMetric.WithLabelValues(user.Username, "folder", hour, day).Add(float64(n))
	} else if feedStr := r.Form.Get("s"); feedStr != "" {
		feedId, err := parseFeedId(feedStr)
		if err != nil {
			log.Warningf("Invalid feed ID: %s", feedStr)
			a.returnError(w, http.StatusBadRequest)
			return
		}

		n, err := a.d.MarkFeedForUser(user, feedId, models.MarkActionRead)
		if err != nil {
			log.Warningf("Failed to mark feed: %d", feedId)
			a.returnError(w, http.StatusInternalServerError)
			return
		}
		hour, day := readActivityLabels()
		articlesMarkedReadMetric.WithLabelValues(user.Username, "feed", hour, day).Add(float64(n))
	} else {
		log.Warningf("Missing feed or folder ID")
		a.returnError(w, http.StatusBadRequest)
		return
	}

	_, _ = w.Write([]byte("OK"))
}

func (a GReader) withAuth(w http.ResponseWriter, r *http.Request, handler func(http.ResponseWriter, *http.Request, models.User)) {
	// Header should be in format:
	//   Authorization: GoogleLogin auth=<token>
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		// A browser presents its session as a cookie instead, which is what
		// lets the page keep the credential out of reach of its own scripts.
		// Endpoints that change data still require a post token, so accepting
		// a cookie here does not make them reachable from another site.
		token, ok := auth.SessionFromRequest(r)
		if !ok {
			log.Warningf("Missing authorization header")
			a.returnError(w, http.StatusUnauthorized)
			return
		}
		// Sent before the handler writes anything, so that a browser holds a
		// cookie that lasts as long as the session it names.
		auth.RefreshSessionCookie(w, r)
		a.dispatchAuthenticated(w, r, token, handler)
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

	a.dispatchAuthenticated(w, r, models.Secret(tokenStr), handler)
}

// dispatchAuthenticated resolves a credential and runs the handler as its
// owner, whichever way the credential was presented.
func (a GReader) dispatchAuthenticated(w http.ResponseWriter, r *http.Request, token models.Secret, handler func(http.ResponseWriter, *http.Request, models.User)) {
	user, session, ok := a.resolveCredential(w, token)
	if !ok {
		return
	}

	InitUserMetrics(user.Username)

	handler(w, r.WithContext(withSession(r.Context(), session)), user)
}

// resolveCredential identifies the user behind a bearer token, along with the
// session it belongs to where there is one, writing the error response and
// reporting false if it cannot.
func (a GReader) resolveCredential(w http.ResponseWriter, token models.Secret) (models.User, models.Session, bool) {
	if storage.IsSessionToken(token) {
		user, session, err := a.d.LookupSession(token)
		if err != nil {
			// Expired, revoked and never-issued are deliberately one case, so
			// that a caller cannot probe for which.
			log.Warningf("Rejected unknown or expired session")
			a.returnError(w, http.StatusUnauthorized)
			return models.User{}, models.Session{}, false
		}
		return user, session, true
	}

	username, digest, err := extractLegacyAuthToken(token.Reveal())
	if err != nil {
		log.Warningf("Unparseable authorization token")
		a.returnError(w, http.StatusBadRequest)
		return models.User{}, models.Session{}, false
	}

	user, err := a.d.GetUserByUsername(username)
	if err != nil {
		log.Warningf("Failed to find user: %s", username)
		a.returnError(w, http.StatusUnauthorized)
		return models.User{}, models.Session{}, false
	}

	if !validateLegacyAuthToken(digest, username, user.HashPass) {
		log.Warningf("Invalid token for user: %s", username)
		a.returnError(w, http.StatusUnauthorized)
		return models.User{}, models.Session{}, false
	}

	// Surfaced so that the deprecation window can be closed on evidence that
	// nothing is still presenting the old format.
	log.Warningf("Accepted legacy auth token for user: %s", username)
	return user, models.Session{}, true
}

// feedStreamPrefix and folderStreamPrefix are what greaderFeedId and
// greaderFolderId emit, and what a client sends back.
const (
	feedStreamPrefix   = "feed/"
	folderStreamPrefix = "user/-/label/"
)

// parseFeedId reads a feed identifier from a request, accepting either the
// `feed/<id>` stream form or a bare decimal ID.
//
// Both spellings are accepted because the endpoints disagreed about which they
// took: subscription editing is given a stream ID, marking a feed read a bare
// number. Accepting both everywhere settles it without making one existing
// client's requests invalid.
func parseFeedId(s string) (int64, error) {
	return strconv.ParseInt(strings.TrimPrefix(s, feedStreamPrefix), 10, 64)
}

// parseFolderId reads a folder identifier, accepting either the
// `user/-/label/<id>` stream form or a bare decimal ID.
func parseFolderId(s string) (int64, error) {
	return strconv.ParseInt(strings.TrimPrefix(s, folderStreamPrefix), 10, 64)
}

func greaderArticleId(articleId int64) string {
	// Note: This is writing the article ID as hex.
	return fmt.Sprintf("tag:google.com,2005:reader/item/%x", articleId)
}

func greaderFeedId(feedId int64) string {
	return feedStreamPrefix + strconv.FormatInt(feedId, 10)
}

func greaderFolderId(folderId int64) string {
	return folderStreamPrefix + strconv.FormatInt(folderId, 10)
}

func (a GReader) validateLoginForm(r *http.Request) (models.Secret, int) {
	token := models.Secret("")

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

	// Recorded so that a session listing can identify the device holding it.
	// This is the only point at which the client describes itself as part of
	// establishing a credential.
	token, err = a.d.CreateSession(user, models.AuthSchemeGReader, r.Header.Get("User-Agent"))
	if err != nil {
		log.Warningf("Failed to create session: %v", err)
		return token, http.StatusInternalServerError
	}

	return token, http.StatusOK
}

func (a GReader) returnError(w http.ResponseWriter, status int) {
	w.WriteHeader(status)
}

func (a GReader) returnInvalidPostToken(w http.ResponseWriter, token models.Secret) {
	// Redacted explicitly rather than left to the type, so that the issue time
	// survives: it is what makes a rejection legible.
	log.Warningf("Invalid post token: %s", redactPostToken(token.Reveal()))
	// Set before WriteHeader: headers written afterwards are discarded, and
	// this one is how clients know to refetch a token rather than treat the
	// 401 as an auth failure.
	w.Header().Set(invalidPostTokenHeader, "true")
	w.WriteHeader(http.StatusUnauthorized)
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

type parseFullArticleResponse struct {
	Content string `json:"content"`
}

func (a GReader) handleParseFullArticle(w http.ResponseWriter, r *http.Request, user models.User) {
	if !*serveParsedArticles {
		a.returnError(w, http.StatusForbidden)
		return
	}

	err := r.ParseForm()
	if err != nil {
		a.returnError(w, http.StatusBadRequest)
		return
	}

	if !a.checkPostToken(w, r, user) {
		return
	}

	articleIDStr := r.Form.Get("i")
	if articleIDStr == "" {
		log.Warning("Missing article ID parameter 'i'")
		a.returnError(w, http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(articleIDStr, 16, 64)
	if err != nil {
		log.Warningf("Invalid article ID: %s", articleIDStr)
		a.returnError(w, http.StatusBadRequest)
		return
	}

	articles, err := a.d.GetArticlesForUser(user, []int64{id})
	if err != nil {
		log.Warningf("Failed to retrieve article: %s", err)
		a.returnError(w, http.StatusInternalServerError)
		return
	}

	if len(articles) == 0 {
		log.Warningf("Article %d not found for user", id)
		a.returnError(w, http.StatusNotFound)
		return
	}

	article := articles[0]

	// If the article is already parsed, return it directly.
	if article.Parsed != "" {
		a.returnSuccess(w, parseFullArticleResponse{Content: article.Parsed})
		return
	}

	// Fetch and parse the URL.
	parsedContent, err := fetch.ExtractFullText(r.Context(), article.Link)
	if err != nil {
		log.Warningf("Failed to extract full text for article %d (%s): %s", id, article.Link, err)
		a.returnError(w, http.StatusBadGateway)
		return
	}

	// Rewrite relative URLs/images and proxy them using the article's URL as base.
	rewrittenContent := fetch.ProcessHTMLContent(article.Link, parsedContent)

	// Sanitize content using the fetch package policy.
	sanitizedContent := fetch.SanitizeBody(rewrittenContent)

	// Persist the sanitized parsed content to CockroachDB.
	err = a.d.UpdateArticleParsedContentForUser(user, id, sanitizedContent)
	if err != nil {
		log.Warningf("Failed to persist parsed article content to DB: %s", err)
		a.returnError(w, http.StatusInternalServerError)
		return
	}

	a.returnSuccess(w, parseFullArticleResponse{Content: sanitizedContent})
}

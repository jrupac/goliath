package api

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jrupac/goliath/fetch"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"golang.org/x/crypto/bcrypt"
)

// Post tokens are signed, so the key has to exist before any handler that
// issues or checks one runs.
func TestMain(m *testing.M) {
	InitPostTokenKey()
	os.Exit(m.Run())
}

// postTokenFor issues a post token for a user authenticating without a session,
// which is how the handler tests call in.
func postTokenFor(user models.User) models.Secret {
	return createPostToken(postTokenBinding(user, models.Session{}))
}

func TestHandleParseFullArticle(t *testing.T) {
	// Enable serveParsedArticles flag for testing
	*serveParsedArticles = true

	// Set up mock HTTP server for the extracted article
	articleHTML := `
<!DOCTYPE html>
<html>
<head><title>Test Article</title></head>
<body>
	<article>
		<h1>Article Header</h1>
		<p>Here is a relative link: <a href="subpage/link">Relative Link</a></p>
		<p>Here is a relative image: <img src="images/photo.jpg" /></p>
		<script>alert("evil script")</script>
	</article>
</body>
</html>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(articleHTML))
	}))
	defer ts.Close()

	// Override the HTTP client in the readability extractor
	fetch.SetClientForTesting(ts.Client())

	// Configure mock database
	mockDB := &storage.MockDB{
		OnGetArticlesForUser: func(u models.User, ids []int64) ([]models.Article, error) {
			if len(ids) == 1 && ids[0] == 12345 {
				return []models.Article{
					{
						ID:     12345,
						Title:  "Test Title",
						Link:   ts.URL + "/folder/article.html", // Base URL for relative resolution
						Parsed: "",                              // Empty to trigger extraction
					},
				}, nil
			}
			return nil, nil
		},
	}

	var savedParsedContent string
	mockDB.OnUpdateArticleParsedContentForUser = func(u models.User, articleID int64, parsed string) error {
		if articleID == 12345 {
			savedParsedContent = parsed
		}
		return nil
	}

	greader := GReader{d: mockDB}

	// Prepare request
	form := url.Values{}
	form.Add("T", postTokenFor(models.User{UserId: "test-user"}).Reveal())
	form.Add("i", "3039") // hex representation of 12345 is 3039 (12345 = 0x3039)
	req := httptest.NewRequest("POST", "/greader/ext/parse-full-article", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	user := models.User{UserId: "test-user"}

	// Run handler
	greader.handleParseFullArticle(w, req, user)

	// Check response status code
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", resp.StatusCode)
	}

	// Verify that the savedParsedContent (and response content) has both sanitization and rewriting transformations applied:
	// 1. Sanitization: script tags must be stripped.
	if strings.Contains(savedParsedContent, "evil script") || strings.Contains(savedParsedContent, "<script>") {
		t.Errorf("saved parsed content was not sanitized (contained script): %s", savedParsedContent)
	}

	// 2. Link rewriting: "subpage/link" must be rewritten using the base article URL (ts.URL + "/folder/article.html").
	expectedLink := ts.URL + "/folder/subpage/link"
	if !strings.Contains(savedParsedContent, expectedLink) {
		t.Errorf("saved parsed content did not have link resolved (expected %q): %s", expectedLink, savedParsedContent)
	}

	// 3. Image rewriting: "images/photo.jpg" must be rewritten to absolute URL.
	expectedImg := ts.URL + "/folder/images/photo.jpg"
	if !strings.Contains(savedParsedContent, expectedImg) {
		t.Errorf("saved parsed content did not have image resolved (expected %q): %s", expectedImg, savedParsedContent)
	}
}

// editTagRequest builds an edit-tag request carrying the given hex article IDs.
func editTagRequest(user models.User, tag string, addTag bool, hexIds ...string) *http.Request {
	form := url.Values{}
	form.Add("T", postTokenFor(user).Reveal())
	if addTag {
		form.Add("a", tag)
	} else {
		form.Add("r", tag)
	}
	for _, id := range hexIds {
		form.Add("i", id)
	}
	req := httptest.NewRequest("POST", "/greader/reader/api/0/edit-tag", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

// Clients batch hundreds of IDs into one request here, so the handler must
// issue a single bulk mark rather than one call per ID.
func TestHandleEditTagMarksAllIdsInOneCall(t *testing.T) {
	var calls int
	var gotIds []int64
	var gotMark models.MarkAction

	mockDB := &storage.MockDB{
		OnMarkArticlesForUser: func(_ models.User, ids []int64, mark models.MarkAction) (int64, error) {
			calls++
			gotIds = ids
			gotMark = mark
			return int64(len(ids)), nil
		},
	}

	greader := GReader{d: mockDB}
	w := httptest.NewRecorder()
	user := models.User{UserId: "test-user"}
	req := editTagRequest(user, readStreamId, true, "3039", "1", "10c2b4a0bbed8001")
	greader.handleEditTag(w, req, user)

	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("status = %d, want %d", got, http.StatusOK)
	}
	if calls != 1 {
		t.Errorf("MarkArticlesForUser called %d times, want exactly 1", calls)
	}
	// Article IDs are hex on the wire.
	want := []int64{12345, 1, 1207726252529385473}
	if len(gotIds) != len(want) {
		t.Fatalf("marked %v, want %v", gotIds, want)
	}
	for i := range want {
		if gotIds[i] != want[i] {
			t.Errorf("marked[%d] = %d, want %d", i, gotIds[i], want[i])
		}
	}
	if gotMark != models.MarkActionRead {
		t.Errorf("mark = %v, want MarkActionRead", gotMark)
	}
}

// Removing the read tag is how clients mark unread, rather than adding the
// kept-unread tag.
func TestHandleEditTagRemoveReadMarksUnread(t *testing.T) {
	var gotMark models.MarkAction
	mockDB := &storage.MockDB{
		OnMarkArticlesForUser: func(_ models.User, ids []int64, mark models.MarkAction) (int64, error) {
			gotMark = mark
			return int64(len(ids)), nil
		},
	}

	greader := GReader{d: mockDB}
	w := httptest.NewRecorder()
	user := models.User{UserId: "u"}
	greader.handleEditTag(w, editTagRequest(user, readStreamId, false, "3039"), user)

	if gotMark != models.MarkActionUnread {
		t.Errorf("mark = %v, want MarkActionUnread", gotMark)
	}
}

// A malformed article ID is the client's fault, not the server's.
func TestHandleEditTagRejectsUnparseableIdAsBadRequest(t *testing.T) {
	greader := GReader{d: &storage.MockDB{}}
	w := httptest.NewRecorder()
	user := models.User{UserId: "u"}
	greader.handleEditTag(w, editTagRequest(user, readStreamId, true, "nothex"), user)

	if got := w.Result().StatusCode; got != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", got, http.StatusBadRequest)
	}
}

func TestMarkAllAsReadRejectsUnparseableIdAsBadRequest(t *testing.T) {
	for _, tc := range []struct{ name, key string }{
		{"folder", "t"},
		{"feed", "s"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			form := url.Values{}
			form.Add("T", postTokenFor(models.User{UserId: "u"}).Reveal())
			form.Add(tc.key, "not-a-number")
			req := httptest.NewRequest("POST", "/greader/reader/api/0/mark-all-as-read", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			w := httptest.NewRecorder()
			GReader{d: &storage.MockDB{}}.markAllAsRead(w, req, models.User{UserId: "u"})

			if got := w.Result().StatusCode; got != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", got, http.StatusBadRequest)
			}
		})
	}
}

// Clients use this header to distinguish "refetch your post token" from a real
// auth failure, so it has to survive to the wire.
func TestReturnInvalidPostTokenSendsHeader(t *testing.T) {
	w := httptest.NewRecorder()
	GReader{}.returnInvalidPostToken(w, "stale")

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if got := resp.Header.Get(invalidPostTokenHeader); got != "true" {
		t.Errorf("%s = %q, want %q", invalidPostTokenHeader, got, "true")
	}
}

// The auth token neither expires nor can be revoked short of a password
// change, so it must never reach the log.
func TestDumpRequestRedactedMasksCredentials(t *testing.T) {
	const secret = "GoogleLogin auth=super-secret-token"

	req := httptest.NewRequest("POST", "/greader/reader/api/0/subscription/edit", nil)
	req.Header.Set("Authorization", secret)
	req.Header.Set("Cookie", "goliath_token=also-secret")
	req.Header.Set("User-Agent", "TestClient/1.0")

	dump, err := dumpRequestRedacted(req)
	if err != nil {
		t.Fatalf("dumpRequestRedacted: %v", err)
	}

	got := string(dump)
	for _, leaked := range []string{"super-secret-token", "also-secret"} {
		if strings.Contains(got, leaked) {
			t.Errorf("dump leaked %q:\n%s", leaked, got)
		}
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Errorf("dump missing redaction marker:\n%s", got)
	}
	// Diagnostics that are not credentials must survive.
	if !strings.Contains(got, "TestClient/1.0") {
		t.Errorf("dump dropped User-Agent:\n%s", got)
	}
	// The caller's request must be left exactly as it was found.
	if got := req.Header.Get("Authorization"); got != secret {
		t.Errorf("Authorization header not restored: %q", got)
	}
}

// A request whose form cannot be parsed must not reach a handler: the error
// status has already been written, and dispatching anyway writes a second one.
func TestRouteDoesNotDispatchWhenFormParsingFails(t *testing.T) {
	req := httptest.NewRequest("POST", "/greader/reader/api/0/token", nil)
	req.URL.RawQuery = "%zz"

	w := httptest.NewRecorder()
	GReader{d: &storage.MockDB{}}.route(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	// route sets this only after preprocessing succeeds, so its absence shows
	// dispatch was skipped rather than merely erroring later.
	if got := resp.Header.Get("Content-Type"); got == "application/json" {
		t.Error("route continued past a failed form parse")
	}
}

func authorizedRequest(token string) *http.Request {
	req := httptest.NewRequest("GET", "/greader/reader/api/0/user-info", nil)
	req.Header.Set("Authorization", "GoogleLogin auth="+token)
	return req
}

// A session token identifies its holder on its own: nothing in the credential
// names a user, so the only way to resolve one is to look the session up.
func TestWithAuthResolvesSessionToken(t *testing.T) {
	const token = "gol1_" + "abcdefghijklmnopqrstuvwxyz012345"
	want := models.User{UserId: "user-1", Username: "someone", Key: "k"}

	var gotToken models.Secret
	mockDB := &storage.MockDB{
		OnLookupSession: func(tok models.Secret) (models.User, models.Session, error) {
			gotToken = tok
			return want, models.Session{SessionId: "s1", UserId: want.UserId, Scheme: models.AuthSchemeGReader}, nil
		},
	}

	greader := GReader{d: mockDB}
	w := httptest.NewRecorder()

	var got models.User
	greader.withAuth(w, authorizedRequest(token), func(_ http.ResponseWriter, _ *http.Request, u models.User) {
		got = u
	})

	if gotToken.Reveal() != token {
		t.Errorf("looked up %q, want %q", gotToken.Reveal(), token)
	}
	if got.UserId != want.UserId {
		t.Errorf("handler ran as %v, want %v", got.UserId, want.UserId)
	}
}

// An unknown session must not fall through to the legacy path, which would
// otherwise try to parse an opaque token as a username-bearing one and report a
// different status.
func TestWithAuthRejectsUnknownSessionToken(t *testing.T) {
	mockDB := &storage.MockDB{
		OnLookupSession: func(models.Secret) (models.User, models.Session, error) {
			return models.User{}, models.Session{}, errors.New("no such session")
		},
	}

	greader := GReader{d: mockDB}
	w := httptest.NewRecorder()

	called := false
	greader.withAuth(w, authorizedRequest("gol1_deadbeef"), func(http.ResponseWriter, *http.Request, models.User) {
		called = true
	})

	if called {
		t.Error("handler ran for an unknown session")
	}
	if got := w.Result().StatusCode; got != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", got, http.StatusUnauthorized)
	}
}

// Clients store their token indefinitely and cannot refresh it, so credentials
// predating sessions have to keep working.
func TestWithAuthAcceptsLegacyToken(t *testing.T) {
	const username = "someone"
	const hashPass = "$2a$10$notarealbcrypthashbutfine"

	sum := sha256.New()
	sum.Write([]byte(legacyTokenSalt))
	sum.Write([]byte(username))
	sum.Write([]byte(hashPass))
	inner, err := json.Marshal(greaderTokenType{
		Username: username,
		Token:    base64.URLEncoding.EncodeToString(sum.Sum(nil)),
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	token := base64.URLEncoding.EncodeToString(inner)

	want := models.User{UserId: "user-1", Username: username, Key: "k", HashPass: models.Secret(hashPass)}
	mockDB := &storage.MockDB{
		OnGetUserByUsername: func(string) (models.User, error) { return want, nil },
		OnLookupSession: func(models.Secret) (models.User, models.Session, error) {
			t.Error("legacy token was looked up as a session")
			return models.User{}, models.Session{}, errors.New("unexpected")
		},
	}

	greader := GReader{d: mockDB}
	w := httptest.NewRecorder()

	var got models.User
	greader.withAuth(w, authorizedRequest(token), func(_ http.ResponseWriter, _ *http.Request, u models.User) {
		got = u
	})

	if got.UserId != want.UserId {
		t.Errorf("handler ran as %v, want %v (status %d)", got.UserId, want.UserId, w.Result().StatusCode)
	}
}

// Logging in establishes a session rather than deriving a credential from the
// password hash, and records what the client called itself.
func TestValidateLoginFormCreatesSession(t *testing.T) {
	const password = "correct horse"
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}

	var gotScheme models.AuthScheme
	var gotUserAgent string
	mockDB := &storage.MockDB{
		OnGetUserByUsername: func(string) (models.User, error) {
			return models.User{UserId: "user-1", Username: "someone", Key: "k", HashPass: models.Secret(hashed)}, nil
		},
		OnCreateSession: func(_ models.User, scheme models.AuthScheme, userAgent string) (models.Secret, error) {
			gotScheme = scheme
			gotUserAgent = userAgent
			return "gol1_issued", nil
		},
	}

	form := url.Values{"Email": {"someone"}, "Passwd": {password}}
	req := httptest.NewRequest("POST", "/greader/accounts/ClientLogin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "TestClient/1.0")
	if err = req.ParseForm(); err != nil {
		t.Fatalf("ParseForm: %v", err)
	}

	greader := GReader{d: mockDB}
	token, status := greader.validateLoginForm(req)

	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if token != "gol1_issued" {
		t.Errorf("token = %q, want the created session's token", token)
	}
	if gotScheme != models.AuthSchemeGReader {
		t.Errorf("scheme = %q, want %q", gotScheme, models.AuthSchemeGReader)
	}
	if gotUserAgent != "TestClient/1.0" {
		t.Errorf("user agent = %q, want %q", gotUserAgent, "TestClient/1.0")
	}
}

// A wrong password must not establish a session.
func TestValidateLoginFormRejectsBadPassword(t *testing.T) {
	hashed, err := bcrypt.GenerateFromPassword([]byte("correct horse"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}

	mockDB := &storage.MockDB{
		OnGetUserByUsername: func(string) (models.User, error) {
			return models.User{UserId: "user-1", Username: "someone", Key: "k", HashPass: models.Secret(hashed)}, nil
		},
		OnCreateSession: func(models.User, models.AuthScheme, string) (models.Secret, error) {
			t.Error("session created despite a bad password")
			return "", nil
		},
	}

	form := url.Values{"Email": {"someone"}, "Passwd": {"wrong"}}
	req := httptest.NewRequest("POST", "/greader/accounts/ClientLogin", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err = req.ParseForm(); err != nil {
		t.Fatalf("ParseForm: %v", err)
	}

	greader := GReader{d: mockDB}
	if _, status := greader.validateLoginForm(req); status != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", status, http.StatusUnauthorized)
	}
}

// The session has to reach the handlers that issue and check post tokens, or
// binding to it would silently degrade to binding to the user.
func TestPostTokenIssuedUnderASessionIsBoundToIt(t *testing.T) {
	user := models.User{UserId: "user-1", Username: "someone", Key: "k"}
	session := models.Session{SessionId: "session-1", UserId: user.UserId, Scheme: models.AuthSchemeGReader}

	mockDB := &storage.MockDB{
		OnLookupSession: func(models.Secret) (models.User, models.Session, error) {
			return user, session, nil
		},
	}

	greader := GReader{d: mockDB}
	w := httptest.NewRecorder()
	greader.withAuth(w, authorizedRequest("gol1_token"), greader.handlePostToken)

	token := models.Secret(w.Body.String())
	if token.Empty() {
		t.Fatal("no post token was issued")
	}
	if !validatePostToken(postTokenBinding(user, session), token) {
		t.Error("the issued token is not bound to the session it was issued under")
	}
	if validatePostToken(postTokenBinding(user, models.Session{}), token) {
		t.Error("the issued token is bound to the user, so the session did not reach the handler")
	}
}

// A mutating request presenting a token from a different session must be
// refused, with the header that tells the client to fetch a new one.
func TestEditTagRejectsPostTokenFromAnotherSession(t *testing.T) {
	user := models.User{UserId: "user-1", Username: "someone", Key: "k"}
	other := models.Session{SessionId: "session-2", UserId: user.UserId}

	greader := GReader{d: &storage.MockDB{
		OnMarkArticlesForUser: func(models.User, []int64, models.MarkAction) (int64, error) {
			t.Error("articles were marked despite a post token from another session")
			return 0, nil
		},
	}}

	form := url.Values{}
	form.Add("T", createPostToken(postTokenBinding(user, other)).Reveal())
	form.Add("a", readStreamId)
	form.Add("i", "3039")
	req := httptest.NewRequest("POST", "/greader/reader/api/0/edit-tag", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	greader.handleEditTag(w, req, user)

	resp := w.Result()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
	if got := resp.Header.Get(invalidPostTokenHeader); got != "true" {
		t.Errorf("%s = %q, want %q", invalidPostTokenHeader, got, "true")
	}
}

// streamItemIdsRequest builds a stream/items/ids request from raw query
// parameters, so that a test can send exactly what a client sends.
func streamItemIdsRequest(params url.Values) *http.Request {
	req := httptest.NewRequest("GET", "/greader/reader/api/0/stream/items/ids?"+params.Encode(), nil)
	_ = req.ParseForm()
	return req
}

// Clients reconcile read state by polling the read stream, so it has to answer
// with the items that are actually read.
func TestStreamItemIdsReadStreamReturnsReadItems(t *testing.T) {
	var gotFilter models.StreamFilter
	mockDB := &storage.MockDB{
		OnGetArticleMetaWithFilterForUser: func(_ models.User, filter models.StreamFilter, _ int, _ models.StreamCursor) ([]models.ArticleMeta, error) {
			gotFilter = filter
			return []models.ArticleMeta{{ID: 12345, FeedID: 7, FolderID: 3}}, nil
		},
	}

	w := httptest.NewRecorder()
	user := models.User{UserId: "u"}
	GReader{d: mockDB}.handleStreamItemIds(w, streamItemIdsRequest(url.Values{
		"s": {readStreamId},
		"n": {"10000"},
	}), user)

	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("status = %d, want %d", got, http.StatusOK)
	}
	if gotFilter != models.StreamFilterRead {
		t.Errorf("filter = %v, want StreamFilterRead", gotFilter)
	}

	var res greaderStreamItemIds
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(res.ItemRefs) != 1 || res.ItemRefs[0].Id != "12345" {
		t.Errorf("itemRefs = %+v, want the one read article", res.ItemRefs)
	}
}

// "ot" is a Unix timestamp in seconds, nine orders of magnitude away from a
// real article ID. Reading it as an ID cursor turns it into a predicate that
// matches everything the user owns.
func TestStreamItemIdsOtIsATimestampNotAnId(t *testing.T) {
	var got models.StreamCursor
	mockDB := &storage.MockDB{
		OnGetArticleMetaWithFilterForUser: func(_ models.User, _ models.StreamFilter, _ int, cursor models.StreamCursor) ([]models.ArticleMeta, error) {
			got = cursor
			return nil, nil
		},
	}

	w := httptest.NewRecorder()
	user := models.User{UserId: "u"}
	GReader{d: mockDB}.handleStreamItemIds(w, streamItemIdsRequest(url.Values{
		"s":  {readStreamId},
		"ot": {"1786146087"},
	}), user)

	if want := time.Unix(1786146087, 0); !got.Since.Equal(want) {
		t.Errorf("cursor.Since = %v, want %v", got.Since, want)
	}
	if got.SinceID != 0 {
		t.Errorf("cursor.SinceID = %d, want 0; 'ot' must not be read as an ID", got.SinceID)
	}
}

// "c" pages and "ot" narrows; a client sends both and neither may clobber the
// other.
func TestStreamItemIdsContinuationAndOtCompose(t *testing.T) {
	var got models.StreamCursor
	mockDB := &storage.MockDB{
		OnGetArticleMetaWithFilterForUser: func(_ models.User, _ models.StreamFilter, _ int, cursor models.StreamCursor) ([]models.ArticleMeta, error) {
			got = cursor
			return nil, nil
		},
	}

	w := httptest.NewRecorder()
	user := models.User{UserId: "u"}
	GReader{d: mockDB}.handleStreamItemIds(w, streamItemIdsRequest(url.Values{
		"s":  {readStreamId},
		"c":  {"10c2b4a0bbed8001"},
		"ot": {"1786146087"},
	}), user)

	// The continuation token is hex.
	if got.SinceID != 1207726252529385473 {
		t.Errorf("cursor.SinceID = %d, want 1207726252529385473", got.SinceID)
	}
	if want := time.Unix(1786146087, 0); !got.Since.Equal(want) {
		t.Errorf("cursor.Since = %v, want %v", got.Since, want)
	}
}

func TestStreamItemIdsRejectsUnparseableOt(t *testing.T) {
	w := httptest.NewRecorder()
	user := models.User{UserId: "u"}
	GReader{d: &storage.MockDB{}}.handleStreamItemIds(w, streamItemIdsRequest(url.Values{
		"s":  {readStreamId},
		"ot": {"not-a-timestamp"},
	}), user)

	if got := w.Result().StatusCode; got != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", got, http.StatusBadRequest)
	}
}

// Clients send the canonical reading-list pairing on every sync, and it still
// means unread.
func TestStreamItemIdsReadingListStillMeansUnread(t *testing.T) {
	var gotFilter models.StreamFilter
	mockDB := &storage.MockDB{
		OnGetArticleMetaWithFilterForUser: func(_ models.User, filter models.StreamFilter, _ int, _ models.StreamCursor) ([]models.ArticleMeta, error) {
			gotFilter = filter
			return nil, nil
		},
	}

	w := httptest.NewRecorder()
	user := models.User{UserId: "u"}
	GReader{d: mockDB}.handleStreamItemIds(w, streamItemIdsRequest(url.Values{
		"s":  {readingListStreamId},
		"xt": {readStreamId},
	}), user)

	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("status = %d, want %d", got, http.StatusOK)
	}
	if gotFilter != models.StreamFilterUnread {
		t.Errorf("filter = %v, want StreamFilterUnread", gotFilter)
	}
}

// "n" is a client's request for a page size, and nothing on the wire stops it
// asking for more than the server should produce at once.
func TestStreamItemIdsClampsLimit(t *testing.T) {
	for _, tc := range []struct {
		name string
		n    string
		want int
	}{
		{"within the ceiling", "500", 500},
		{"above the ceiling", "1000000", storage.MaxFetchedRows},
		{"absent", "", storage.MaxFetchedRows},
		{"unparseable", "lots", storage.MaxFetchedRows},
		{"zero", "0", storage.MaxFetchedRows},
		{"negative", "-1", storage.MaxFetchedRows},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got int
			mockDB := &storage.MockDB{
				OnGetArticleMetaWithFilterForUser: func(_ models.User, _ models.StreamFilter, limit int, _ models.StreamCursor) ([]models.ArticleMeta, error) {
					got = limit
					return nil, nil
				},
			}

			params := url.Values{"s": {readingListStreamId}}
			if tc.n != "" {
				params.Set("n", tc.n)
			}
			w := httptest.NewRecorder()
			user := models.User{UserId: "u"}
			GReader{d: mockDB}.handleStreamItemIds(w, streamItemIdsRequest(params), user)

			if got != tc.want {
				t.Errorf("limit = %d, want %d", got, tc.want)
			}
		})
	}
}

// subscriptionRequest builds a signed POST to a subscription endpoint.
func subscriptionRequest(user models.User, path string, form url.Values) *http.Request {
	form.Set(postTokenParam, postTokenFor(user).Reveal())
	req := httptest.NewRequest("POST", path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = req.ParseForm()
	return req
}

func editRequest(user models.User, form url.Values) *http.Request {
	return subscriptionRequest(user, "/greader/reader/api/0/subscription/edit", form)
}

// Every subscription operation names a feed the requester must own. A feed
// belonging to someone else has to be indistinguishable from one that does not
// exist, or the endpoint answers whether an ID is in use.
func TestSubscriptionEditRefusesAnotherUsersFeed(t *testing.T) {
	mockDB := &storage.MockDB{
		OnGetFeedForUser: func(_ models.User, feedID int64) (models.Feed, error) {
			if feedID == 1 {
				return models.Feed{ID: 1, FolderID: 9}, nil
			}
			return models.Feed{}, sql.ErrNoRows
		},
		OnDeleteFeedForUser: func(models.User, int64, int64) error {
			t.Error("deleted a feed the user does not own")
			return nil
		},
		OnUpdateFeedMetadataForUser: func(models.User, models.Feed) error {
			t.Error("renamed a feed the user does not own")
			return nil
		},
	}

	user := models.User{UserId: "u", Username: "u"}
	for _, form := range []url.Values{
		{"ac": {"unsubscribe"}, "s": {"feed/2"}},
		{"ac": {"edit"}, "s": {"feed/2"}, "t": {"new"}},
	} {
		w := httptest.NewRecorder()
		GReader{d: mockDB}.handleSubscriptionEdit(w, editRequest(user, form), user)
		if got := w.Result().StatusCode; got != http.StatusNotFound {
			t.Errorf("%v: status = %d, want %d", form, got, http.StatusNotFound)
		}
	}
}

// `s` arrives as a stream ID here and as a bare number on mark-all-as-read.
// Both spellings are accepted in both places rather than adding a second
// convention.
func TestSubscriptionEditAcceptsBothFeedIdForms(t *testing.T) {
	for _, s := range []string{"feed/12345", "12345"} {
		var renamed int64
		mockDB := &storage.MockDB{
			OnGetFeedForUser: func(_ models.User, feedID int64) (models.Feed, error) {
				return models.Feed{ID: feedID, FolderID: 9}, nil
			},
			OnUpdateFeedMetadataForUser: func(_ models.User, f models.Feed) error {
				renamed = f.ID
				return nil
			},
		}
		user := models.User{UserId: "u", Username: "u"}
		w := httptest.NewRecorder()
		GReader{d: mockDB}.handleSubscriptionEdit(w,
			editRequest(user, url.Values{"ac": {"edit"}, "s": {s}, "t": {"Renamed"}}), user)

		if got := w.Result().StatusCode; got != http.StatusOK {
			t.Errorf("s=%q: status = %d, want %d", s, got, http.StatusOK)
		}
		if renamed != 12345 {
			t.Errorf("s=%q: renamed feed %d, want 12345", s, renamed)
		}
	}
}

func TestSubscriptionEditRenames(t *testing.T) {
	var got models.Feed
	mockDB := &storage.MockDB{
		OnGetFeedForUser: func(models.User, int64) (models.Feed, error) {
			return models.Feed{ID: 7, FolderID: 3, Title: "Old", URL: "https://e.invalid/f"}, nil
		},
		OnUpdateFeedMetadataForUser: func(_ models.User, f models.Feed) error {
			got = f
			return nil
		},
	}
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleSubscriptionEdit(w,
		editRequest(user, url.Values{"ac": {"edit"}, "s": {"feed/7"}, "t": {"New Name"}}), user)

	if got.Title != "New Name" {
		t.Errorf("title = %q, want %q", got.Title, "New Name")
	}
	// A rename must not discard the rest of the feed.
	if got.URL != "https://e.invalid/f" || got.FolderID != 3 {
		t.Errorf("rename altered other fields: %+v", got)
	}
}

// A move names its destination folder in `a`, mirroring edit-tag's add-label
// convention.
func TestSubscriptionEditMovesBetweenFolders(t *testing.T) {
	var toFolder int64
	mockDB := &storage.MockDB{
		OnGetFeedForUser: func(models.User, int64) (models.Feed, error) {
			return models.Feed{ID: 7, FolderID: 3}, nil
		},
		OnGetFolderForUser: func(_ models.User, folderID int64) (models.Folder, error) {
			return models.Folder{ID: folderID}, nil
		},
		OnUpdateFolderForFeedForUser: func(_ models.User, _, folderID int64) error {
			toFolder = folderID
			return nil
		},
	}
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleSubscriptionEdit(w, editRequest(user, url.Values{
		"ac": {"edit"}, "s": {"feed/7"},
		"a": {"user/-/label/4"}, "r": {"user/-/label/3"},
	}), user)

	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("status = %d, want %d", got, http.StatusOK)
	}
	if toFolder != 4 {
		t.Errorf("moved to folder %d, want 4", toFolder)
	}
}

// A destination folder is checked the same way the feed is: a move into
// somebody else's folder would file the feed where the requester cannot see it.
func TestSubscriptionEditRefusesAnotherUsersFolder(t *testing.T) {
	mockDB := &storage.MockDB{
		OnGetFeedForUser: func(models.User, int64) (models.Feed, error) {
			return models.Feed{ID: 7, FolderID: 3}, nil
		},
		OnUpdateFolderForFeedForUser: func(models.User, int64, int64) error {
			t.Error("moved a feed into a folder the user does not own")
			return nil
		},
	}
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleSubscriptionEdit(w, editRequest(user, url.Values{
		"ac": {"edit"}, "s": {"feed/7"}, "a": {"user/-/label/99"},
	}), user)

	if got := w.Result().StatusCode; got != http.StatusNotFound {
		t.Errorf("status = %d, want %d", got, http.StatusNotFound)
	}
}

// A move that fails must leave the feed entirely alone, rather than renaming it
// into a folder it did not move to.
func TestSubscriptionEditDoesNotRenameWhenTheMoveFails(t *testing.T) {
	mockDB := &storage.MockDB{
		OnGetFeedForUser: func(models.User, int64) (models.Feed, error) {
			return models.Feed{ID: 7, FolderID: 3}, nil
		},
		OnUpdateFeedMetadataForUser: func(models.User, models.Feed) error {
			t.Error("renamed the feed although the move was refused")
			return nil
		},
	}
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleSubscriptionEdit(w, editRequest(user, url.Values{
		"ac": {"edit"}, "s": {"feed/7"}, "t": {"New"}, "a": {"user/-/label/99"},
	}), user)

	if got := w.Result().StatusCode; got != http.StatusNotFound {
		t.Errorf("status = %d, want %d", got, http.StatusNotFound)
	}
}

func TestSubscriptionEditUnsubscribes(t *testing.T) {
	var deletedFeed, deletedFolder int64
	mockDB := &storage.MockDB{
		OnGetFeedForUser: func(models.User, int64) (models.Feed, error) {
			return models.Feed{ID: 7, FolderID: 3, Title: "Doomed"}, nil
		},
		OnGetArticlesForFeedForUser: func(models.User, int64) ([]models.Article, error) {
			return []models.Article{{ID: 1}, {ID: 2}}, nil
		},
		OnDeleteFeedForUser: func(_ models.User, feedID, folderID int64) error {
			deletedFeed, deletedFolder = feedID, folderID
			return nil
		},
	}
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleSubscriptionEdit(w,
		editRequest(user, url.Values{"ac": {"unsubscribe"}, "s": {"feed/7"}}), user)

	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("status = %d, want %d", got, http.StatusOK)
	}
	if deletedFeed != 7 || deletedFolder != 3 {
		t.Errorf("deleted feed %d in folder %d, want 7 in 3", deletedFeed, deletedFolder)
	}
}

// An unrecognised `ac` must not fall through to one of the operations that
// changes something.
func TestSubscriptionEditRefusesUnknownAction(t *testing.T) {
	mockDB := &storage.MockDB{
		OnGetFeedForUser: func(models.User, int64) (models.Feed, error) {
			return models.Feed{ID: 7}, nil
		},
		OnDeleteFeedForUser: func(models.User, int64, int64) error {
			t.Error("an unknown action deleted a feed")
			return nil
		},
	}
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleSubscriptionEdit(w,
		editRequest(user, url.Values{"ac": {"disable"}, "s": {"feed/7"}}), user)

	if got := w.Result().StatusCode; got != http.StatusNotImplemented {
		t.Errorf("status = %d, want %d", got, http.StatusNotImplemented)
	}
}

// Every one of these mutates or destroys a subscription, so none may run on an
// unsigned request.
func TestSubscriptionEndpointsRequireAPostToken(t *testing.T) {
	mockDB := &storage.MockDB{
		OnGetFeedForUser: func(models.User, int64) (models.Feed, error) {
			return models.Feed{ID: 7}, nil
		},
		OnDeleteFeedForUser: func(models.User, int64, int64) error {
			t.Error("deleted a feed without a post token")
			return nil
		},
		OnInsertFeedForUser: func(models.User, models.Feed, int64) (int64, error) {
			t.Error("added a feed without a post token")
			return 0, nil
		},
	}
	user := models.User{UserId: "u", Username: "u"}

	for _, tc := range []struct {
		name    string
		path    string
		form    url.Values
		handler func(http.ResponseWriter, *http.Request, models.User)
	}{
		{"quickadd", "/greader/reader/api/0/subscription/quickadd",
			url.Values{"quickadd": {"https://example.invalid/feed"}}, GReader{d: mockDB}.handleQuickAdd},
		{"edit", "/greader/reader/api/0/subscription/edit",
			url.Values{"ac": {"unsubscribe"}, "s": {"feed/7"}}, GReader{d: mockDB}.handleSubscriptionEdit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", tc.path, strings.NewReader(tc.form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			_ = req.ParseForm()

			w := httptest.NewRecorder()
			tc.handler(w, req, user)

			if got := w.Result().StatusCode; got != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", got, http.StatusUnauthorized)
			}
		})
	}
}

// Adding a feed already subscribed to reports the existing one. The insert
// would not collide: its conflict key covers the title, which a feed is free to
// change between one add and the next.
func TestQuickAddIsIdempotentForAnExistingFeed(t *testing.T) {
	mockDB := &storage.MockDB{
		OnGetFeedByUrlForUser: func(_ models.User, url string) (models.Feed, error) {
			if url == "https://example.invalid/feed" {
				return models.Feed{ID: 42, Title: "Already Here", URL: url}, nil
			}
			return models.Feed{}, sql.ErrNoRows
		},
		OnInsertFeedForUser: func(models.User, models.Feed, int64) (int64, error) {
			t.Error("inserted a feed that was already subscribed to")
			return 0, nil
		},
	}
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleQuickAdd(w, subscriptionRequest(user,
		"/greader/reader/api/0/subscription/quickadd",
		url.Values{"quickadd": {"https://example.invalid/feed"}}), user)

	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("status = %d, want %d", got, http.StatusOK)
	}
	var res greaderQuickAddResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if res.StreamId != "feed/42" || res.NumResults != 1 {
		t.Errorf("response = %+v, want the existing feed", res)
	}
}

func TestQuickAddRequiresAUrl(t *testing.T) {
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: &storage.MockDB{}}.handleQuickAdd(w, subscriptionRequest(user,
		"/greader/reader/api/0/subscription/quickadd", url.Values{}), user)

	if got := w.Result().StatusCode; got != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", got, http.StatusBadRequest)
	}
}

// A URL that is not a feed is the client's mistake, and is refused rather than
// stored as a subscription that would fail on every cycle afterwards.
func TestQuickAddRefusesSomethingThatIsNotAFeed(t *testing.T) {
	mockDB := &storage.MockDB{
		OnInsertFeedForUser: func(models.User, models.Feed, int64) (int64, error) {
			t.Error("stored a subscription for a URL that is not a feed")
			return 0, nil
		},
	}
	user := models.User{UserId: "u", Username: "u"}

	for _, target := range []string{
		"not-a-url",
		"ftp://example.invalid/feed",
		"file:///etc/passwd",
	} {
		w := httptest.NewRecorder()
		GReader{d: mockDB}.handleQuickAdd(w, subscriptionRequest(user,
			"/greader/reader/api/0/subscription/quickadd", url.Values{"quickadd": {target}}), user)

		if got := w.Result().StatusCode; got != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want %d", target, got, http.StatusBadRequest)
		}
	}
}

// The feed's own XML address, without which no client can show or edit what it
// is subscribed to.
func TestSubscriptionListIncludesTheFeedUrl(t *testing.T) {
	mockDB := &storage.MockDB{
		OnGetAllFeedsForUser: func(models.User) ([]models.Feed, error) {
			return []models.Feed{{
				ID: 7, FolderID: 3, Title: "Feed",
				URL:  "https://example.invalid/feed.xml",
				Link: "https://example.invalid/",
			}}, nil
		},
	}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleSubscriptionList(w, httptest.NewRequest("GET", "/", nil), models.User{UserId: "u"})

	var res greaderSubscriptionList
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(res.Subscriptions) != 1 {
		t.Fatalf("got %d subscriptions, want 1", len(res.Subscriptions))
	}
	if got := res.Subscriptions[0].Url; got != "https://example.invalid/feed.xml" {
		t.Errorf("url = %q, want the feed's XML address", got)
	}
	if got := res.Subscriptions[0].HtmlUrl; got != "https://example.invalid/" {
		t.Errorf("htmlUrl = %q, want the site address", got)
	}
}

// Fetching refreshes a feed's own metadata on every first fetch, and pausing
// the fetcher to add or remove a subscription makes the next fetch a first
// fetch. A rename that did not say it was a rename would survive only until
// the next change to the feed list.
func TestSubscriptionEditMarksARenamedTitleAsTheUsers(t *testing.T) {
	var got models.Feed
	mockDB := &storage.MockDB{
		OnGetFeedForUser: func(models.User, int64) (models.Feed, error) {
			return models.Feed{ID: 7, FolderID: 3, Title: "From The Feed"}, nil
		},
		OnUpdateFeedMetadataForUser: func(_ models.User, f models.Feed) error {
			got = f
			return nil
		},
	}
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleSubscriptionEdit(w,
		editRequest(user, url.Values{"ac": {"edit"}, "s": {"feed/7"}, "t": {"My Name For It"}}), user)

	if !got.TitleOverridden {
		t.Error("a renamed feed was not marked as having a user-set title")
	}
}

// A move alone leaves the title as the feed's own, so a later fetch is still
// free to update it.
func TestSubscriptionEditMoveDoesNotClaimTheTitle(t *testing.T) {
	mockDB := &storage.MockDB{
		OnGetFeedForUser: func(models.User, int64) (models.Feed, error) {
			return models.Feed{ID: 7, FolderID: 3, Title: "From The Feed"}, nil
		},
		OnGetFolderForUser: func(_ models.User, folderID int64) (models.Folder, error) {
			return models.Folder{ID: folderID}, nil
		},
		OnUpdateFeedMetadataForUser: func(_ models.User, f models.Feed) error {
			t.Errorf("a move rewrote the feed's metadata: %+v", f)
			return nil
		},
	}
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleSubscriptionEdit(w, editRequest(user, url.Values{
		"ac": {"edit"}, "s": {"feed/7"}, "a": {"user/-/label/4"},
	}), user)

	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Errorf("status = %d, want %d", got, http.StatusOK)
	}
}

// A lookup that fails for a reason other than the feed not being the user's is
// a server problem, and saying "not found" would send the client to fix
// something that is not wrong.
func TestSubscriptionEditReportsLookupFailureAsServerError(t *testing.T) {
	mockDB := &storage.MockDB{
		OnGetFeedForUser: func(models.User, int64) (models.Feed, error) {
			return models.Feed{}, errors.New("connection refused")
		},
	}
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleSubscriptionEdit(w,
		editRequest(user, url.Values{"ac": {"unsubscribe"}, "s": {"feed/7"}}), user)

	if got := w.Result().StatusCode; got != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", got, http.StatusInternalServerError)
	}
}

// Clients send a bare `ac=edit` naming only the feed after adding one, and
// treat a refusal as the add itself having failed -- which they undo by
// unsubscribing. Refusing a request that asks for no change destroys the feed.
func TestSubscriptionEditWithNothingToChangeSucceeds(t *testing.T) {
	mockDB := &storage.MockDB{
		OnGetFeedForUser: func(models.User, int64) (models.Feed, error) {
			return models.Feed{ID: 7, FolderID: 3, Title: "Just Added"}, nil
		},
		OnUpdateFeedMetadataForUser: func(models.User, models.Feed) error {
			t.Error("a no-op edit rewrote the feed")
			return nil
		},
		OnUpdateFolderForFeedForUser: func(models.User, int64, int64) error {
			t.Error("a no-op edit moved the feed")
			return nil
		},
	}
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleSubscriptionEdit(w,
		editRequest(user, url.Values{"ac": {"edit"}, "s": {"feed/7"}}), user)

	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Errorf("status = %d, want %d", got, http.StatusOK)
	}
	if got := w.Body.String(); got != "OK" {
		t.Errorf("body = %q, want %q", got, "OK")
	}
}

// A lookup that fails for a reason other than the feed not being subscribed is
// a server problem. Treating it as "not subscribed" would fetch the URL and
// create a duplicate subscription.
func TestQuickAddReportsLookupFailureRatherThanAddingAgain(t *testing.T) {
	mockDB := &storage.MockDB{
		OnGetFeedByUrlForUser: func(models.User, string) (models.Feed, error) {
			return models.Feed{}, errors.New("connection refused")
		},
		OnInsertFeedForUser: func(models.User, models.Feed, int64) (int64, error) {
			t.Error("added a feed although the existing-subscription check failed")
			return 0, nil
		},
	}
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleQuickAdd(w, subscriptionRequest(user,
		"/greader/reader/api/0/subscription/quickadd",
		url.Values{"quickadd": {"https://example.invalid/feed"}}), user)

	if got := w.Result().StatusCode; got != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", got, http.StatusInternalServerError)
	}
}

// A client shown the stored sentinel draws a folder called "<root>". The feeds
// in it are the ones filed nowhere else, so it is presented under a name meant
// for people.
func TestSubscriptionListPresentsTheRootFolderUnderAReadableName(t *testing.T) {
	const rootId, comicsId = int64(1), int64(2)
	mockDB := &storage.MockDB{
		OnGetRootFolderForUser: func(models.User) (models.Folder, error) {
			return models.Folder{ID: rootId, Name: models.RootFolder}, nil
		},
		OnGetAllFoldersForUser: func(models.User) ([]models.Folder, error) {
			return []models.Folder{
				{ID: rootId, Name: models.RootFolder},
				{ID: comicsId, Name: "Comics"},
			}, nil
		},
		OnGetAllFeedsForUser: func(models.User) ([]models.Feed, error) {
			return []models.Feed{
				{ID: 10, FolderID: rootId, Title: "Unfiled"},
				{ID: 11, FolderID: comicsId, Title: "Filed"},
			}, nil
		},
	}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleSubscriptionList(w, httptest.NewRequest("GET", "/", nil), models.User{UserId: "u"})

	var res greaderSubscriptionList
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	labels := map[string]string{}
	for _, sub := range res.Subscriptions {
		if len(sub.Categories) != 1 {
			t.Fatalf("%s has %d categories, want 1", sub.Title, len(sub.Categories))
		}
		labels[sub.Title] = sub.Categories[0].Label
	}
	if got := labels["Unfiled"]; got != models.RootFolderDisplayName {
		t.Errorf("unfiled feed's folder label = %q, want %q", got, models.RootFolderDisplayName)
	}
	if got := labels["Filed"]; got != "Comics" {
		t.Errorf("filed feed's folder label = %q, want %q", got, "Comics")
	}
}

// `a` names a destination and `r` a source. An `r` on its own is a client
// saying the feed should no longer be filed anywhere, which is the folder
// unfiled feeds live in.
func TestSubscriptionEditRemovingTheOnlyLabelMovesToTheRoot(t *testing.T) {
	const rootId, comicsId = int64(1), int64(2)
	var movedTo int64
	mockDB := &storage.MockDB{
		OnGetFeedForUser: func(models.User, int64) (models.Feed, error) {
			return models.Feed{ID: 7, FolderID: comicsId}, nil
		},
		OnGetRootFolderForUser: func(models.User) (models.Folder, error) {
			return models.Folder{ID: rootId, Name: models.RootFolder}, nil
		},
		OnUpdateFolderForFeedForUser: func(_ models.User, _, folderID int64) error {
			movedTo = folderID
			return nil
		},
	}
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleSubscriptionEdit(w, editRequest(user, url.Values{
		"ac": {"edit"}, "s": {"feed/7"}, "r": {"user/-/label/2"},
	}), user)

	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Fatalf("status = %d, want %d", got, http.StatusOK)
	}
	if movedTo != rootId {
		t.Errorf("moved to folder %d, want the root folder %d", movedTo, rootId)
	}
}

// Removing a label from a feed already unfiled asks for no change, so nothing
// is written.
func TestSubscriptionEditRemovingALabelFromAnUnfiledFeedDoesNothing(t *testing.T) {
	const rootId = int64(1)
	mockDB := &storage.MockDB{
		OnGetFeedForUser: func(models.User, int64) (models.Feed, error) {
			return models.Feed{ID: 7, FolderID: rootId}, nil
		},
		OnGetRootFolderForUser: func(models.User) (models.Folder, error) {
			return models.Folder{ID: rootId, Name: models.RootFolder}, nil
		},
		OnUpdateFolderForFeedForUser: func(models.User, int64, int64) error {
			t.Error("moved a feed that was already unfiled")
			return nil
		},
	}
	user := models.User{UserId: "u", Username: "u"}
	w := httptest.NewRecorder()
	GReader{d: mockDB}.handleSubscriptionEdit(w, editRequest(user, url.Values{
		"ac": {"edit"}, "s": {"feed/7"}, "r": {"user/-/label/1"},
	}), user)

	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Errorf("status = %d, want %d", got, http.StatusOK)
	}
}

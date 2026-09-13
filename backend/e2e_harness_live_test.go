package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jrupac/goliath/admin"
	"github.com/jrupac/goliath/api"
	"github.com/jrupac/goliath/cache"
	"github.com/jrupac/goliath/devseed"
	"github.com/jrupac/goliath/feedtest"
	"github.com/jrupac/goliath/fetch"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// The end-to-end test runs a complete server -- HTTP handlers, the admin
// service and the fetch scheduler -- and drives it only through the interfaces
// clients use, against a real CockroachDB and a local server standing in for
// every feed publisher. It creates a database of its own and drops it
// afterwards.
//
// Skipped unless GOLIATH_E2E_CRDB is set. The rest are optional:
//
//	GOLIATH_E2E_CRDB     connection string for the cluster, as a user that can
//	                     create databases
//	GOLIATH_E2E_SEED_DB  database to copy feeds and articles from, so that users
//	                     start with realistic data; it is only read
//	GOLIATH_E2E_SCALE    "large" for a sizing run rather than a quick one
//	GOLIATH_E2E_USERS, GOLIATH_E2E_FEEDS, GOLIATH_E2E_FEEDS_PER_USER,
//	GOLIATH_E2E_ITEMS    override individual sizes
//	GOLIATH_E2E_DEADLINE how long each wait for the server to catch up may
//	                     take, as a Go duration
//	GOLIATH_E2E_KEEP     keep the database afterwards
//	GOLIATH_E2E_REPORT   also write the timing report to this path, as JSON
//
// The race detector slows everything several times over, so take timings
// from a run without -race and correctness from a run with it.

const (
	greaderAPI  = "/greader/reader/api/0/"
	readingList = "user/-/state/com.google/reading-list"
	readState   = "user/-/state/com.google/read"
	starred     = "user/-/state/com.google/starred"
	labelPrefix = "user/-/label/"
)

// e2eConfig sizes a run.
type e2eConfig struct {
	Users int
	// PoolFeeds is how many feeds the local server publishes, and FeedsPerUser
	// how many of them each user subscribes to. Users take overlapping
	// windows of the pool, so most feeds have several subscribers.
	PoolFeeds    int
	FeedsPerUser int
	// Items is how many items each pool feed starts with.
	Items int
	// Rounds is how many rounds of mixed requests each user makes while
	// feeds are updating underneath them.
	Rounds    int
	SeedShare float64
	// Deadline bounds each wait for the server to catch up.
	Deadline time.Duration
}

func loadE2EConfig(t *testing.T) e2eConfig {
	c := e2eConfig{Users: 10, PoolFeeds: 30, FeedsPerUser: 12, Items: 20, Rounds: 20, SeedShare: 0.4,
		Deadline: 2 * time.Minute}
	if os.Getenv("GOLIATH_E2E_SCALE") == "large" {
		c = e2eConfig{Users: 100, PoolFeeds: 1000, FeedsPerUser: 500, Items: 5, Rounds: 5, SeedShare: 0.4,
			Deadline: 20 * time.Minute}
	}
	for env, field := range map[string]*int{
		"GOLIATH_E2E_USERS":          &c.Users,
		"GOLIATH_E2E_FEEDS":          &c.PoolFeeds,
		"GOLIATH_E2E_FEEDS_PER_USER": &c.FeedsPerUser,
		"GOLIATH_E2E_ITEMS":          &c.Items,
	} {
		if v := os.Getenv(env); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				t.Fatalf("%s=%q is not a positive integer", env, v)
			}
			*field = n
		}
	}
	if v := os.Getenv("GOLIATH_E2E_DEADLINE"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			t.Fatalf("GOLIATH_E2E_DEADLINE=%q is not a positive duration", v)
		}
		c.Deadline = d
	}
	// Each of the later phases needs a user of its own to act on.
	if c.Users < 6 {
		t.Fatalf("need at least 6 users, have %d", c.Users)
	}
	c.FeedsPerUser = min(c.FeedsPerUser, c.PoolFeeds)
	return c
}

// e2eEnv is a running server and everything needed to talk to it and to check
// its database.
type e2eEnv struct {
	cfg   e2eConfig
	d     storage.Database
	raw   *sql.DB
	web   *httptest.Server
	admin admin.AdminServiceClient
	sched *fetch.Scheduler
	feeds *feedtest.Server
	tm    *timings
}

func startE2E(t *testing.T) *e2eEnv {
	clusterDSN := os.Getenv("GOLIATH_E2E_CRDB")
	if clusterDSN == "" {
		t.Skip("GOLIATH_E2E_CRDB not set")
	}
	cfg := loadE2EConfig(t)
	ctx := context.Background()

	logDir, err := os.MkdirTemp("", "goliath-e2e-logs-")
	if err != nil {
		t.Fatal(err)
	}
	setFlag(t, "log_dir", logDir)
	t.Logf("Server logs are in %s", logDir)

	schema, err := os.ReadFile("schema/latest.sql")
	if err != nil {
		t.Fatalf("reading schema: %v", err)
	}
	name := fmt.Sprintf("goliath_e2e_%d", time.Now().UnixNano())
	dsn, err := devseed.CreateDatabase(ctx, clusterDSN, name, string(schema))
	if err != nil {
		t.Fatalf("creating database: %v", err)
	}
	// Registered first, so that it runs last, after the server has stopped.
	if os.Getenv("GOLIATH_E2E_KEEP") == "" {
		t.Cleanup(func() {
			if err := devseed.DropDatabase(context.Background(), clusterDSN, name); err != nil {
				t.Errorf("dropping %s: %v", name, err)
			}
		})
	} else {
		t.Logf("Keeping database %s", name)
	}

	feeds, err := feedtest.Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("starting feed server: %v", err)
	}
	t.Cleanup(func() { _ = feeds.Close() })

	// Fetched once when subscribed and afterwards only when the test says a
	// feed has changed, so that what the server does is what the test asked
	// for rather than whatever a timer happened to start.
	setFlag(t, "feedAllowedAddresses", feeds.Addr())
	setFlag(t, "fetchStartSpread", "0s")
	setFlag(t, "fetchReconcileInterval", "1s")
	setFlag(t, "minFetchInterval", "1h")
	setFlag(t, "maxFetchInterval", "2h")

	d, err := storage.Open(dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	raw, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	retCache, err := cache.StartRetrievalCache(runCtx, d)
	if err != nil {
		t.Fatalf("starting retrieval cache: %v", err)
	}
	allowed, err := fetch.NewFeedAllowlist()
	if err != nil {
		t.Fatalf("allowlist: %v", err)
	}
	sched := fetch.NewScheduler(fetch.New(d, retCache, allowed), d)
	fetch.SetDiscoverClientForTesting(&http.Client{Timeout: 10 * time.Second})
	api.InitPostTokenKey()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Go(func() { sched.Run(runCtx) })
	wg.Go(func() { admin.Serve(runCtx, lis, d, sched) })
	web := httptest.NewServer(newMux(d, sched))

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = conn.Close()
		web.Close()
		cancel()
		wg.Wait()
		_ = raw.Close()
		_ = d.Close()
	})

	return &e2eEnv{
		cfg:   cfg,
		d:     d,
		raw:   raw,
		web:   web,
		admin: admin.NewAdminServiceClient(conn),
		sched: sched,
		feeds: feeds,
		tm:    newTimings(),
	}
}

// setFlag sets a flag for the duration of the test.
func setFlag(t *testing.T, name, value string) {
	f := flag.Lookup(name)
	if f == nil {
		t.Fatalf("no flag named %s", name)
	}
	old := f.Value.String()
	if err := flag.Set(name, value); err != nil {
		t.Fatalf("setting %s: %v", name, err)
	}
	t.Cleanup(func() { _ = flag.Set(name, old) })
}

// parallel runs fn for each of 0..n-1 at once and waits for all of them.
func parallel(n int, fn func(i int)) {
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { fn(i) })
	}
	wg.Wait()
}

// waitFor polls check until it reports true, recording how long that took as a
// phase. check says why it is not yet satisfied, for the failure message.
func (e *e2eEnv) waitFor(t *testing.T, phase string, check func() (bool, string)) {
	t.Helper()
	end := e.tm.phase(phase)
	deadline := time.Now().Add(e.cfg.Deadline)
	for {
		ok, why := check()
		if ok {
			end()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("gave up after %s waiting for %s: %s", e.cfg.Deadline, phase, why)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// timed runs an admin call, recording its latency.
func (e *e2eEnv) timed(op string, fn func() error) error {
	start := time.Now()
	err := fn()
	e.tm.record(op, time.Since(start), err == nil)
	return err
}

/*******************************************************************************
 * Clients
 ******************************************************************************/

// tester is one user and the clients acting for them.
type tester struct {
	name     string
	password string
	user     models.User
	feverKey string
	// api presents a GReader token, as a feed reader app does; web presents
	// the session cookie, as the browser does.
	api *apiClient
	web *apiClient
	// pool holds the pool feeds they subscribed to, and seeded what was
	// copied to them from the seed database.
	pool    []poolSub
	seeded  []devseed.Copied
	deleted bool
}

type poolSub struct {
	index  int
	url    string
	feedID int64
}

func poolPath(i int) string { return fmt.Sprintf("/pool/%d", i) }

// apiClient makes GReader requests as one user.
type apiClient struct {
	env  *e2eEnv
	http *http.Client
	// auth is the GReader token; empty for a client authenticating by cookie.
	auth string

	mu        sync.Mutex
	postToken string
}

func (e *e2eEnv) newClient(auth string) *apiClient {
	return &apiClient{env: e, http: &http.Client{Timeout: time.Minute}, auth: auth}
}

// send makes one request, recording its latency under op.
func (c *apiClient) send(op, method, path string, form url.Values) (int, []byte, http.Header) {
	target := c.env.web.URL + path
	var body io.Reader
	if method == http.MethodGet {
		if len(form) > 0 {
			target += "?" + form.Encode()
		}
	} else {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, target, body)
	if err != nil {
		panic(err)
	}
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if c.auth != "" {
		req.Header.Set("Authorization", "GoogleLogin auth="+c.auth)
	}
	req.Header.Set("User-Agent", "GoliathEndToEnd/1.0")

	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		c.env.tm.record(op, time.Since(start), false)
		return 0, []byte(err.Error()), nil
	}
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	c.env.tm.record(op, time.Since(start), resp.StatusCode < 300)
	return resp.StatusCode, b, resp.Header
}

// get makes a read.
func (c *apiClient) get(op, endpoint string, form url.Values) (int, []byte) {
	status, body, _ := c.send(op, http.MethodGet, greaderAPI+endpoint, form)
	return status, body
}

// post makes a write, carrying a post token. A token the server calls stale
// is refreshed and the request sent once more, as clients do.
func (c *apiClient) post(op, endpoint string, form url.Values) (int, []byte) {
	for attempt := 0; ; attempt++ {
		withToken := url.Values{}
		for k, v := range form {
			withToken[k] = v
		}
		withToken.Set("T", c.token())
		status, body, header := c.send(op, http.MethodPost, greaderAPI+endpoint, withToken)
		if status == http.StatusUnauthorized && header.Get("X-Reader-Google-Bad-Token") != "" && attempt == 0 {
			c.refreshToken()
			continue
		}
		return status, body
	}
}

func (c *apiClient) token() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.postToken == "" {
		c.mu.Unlock()
		c.refreshToken()
		c.mu.Lock()
	}
	return c.postToken
}

func (c *apiClient) refreshToken() {
	status, body, _ := c.send("greader token", http.MethodGet, greaderAPI+"token", nil)
	c.mu.Lock()
	defer c.mu.Unlock()
	if status == http.StatusOK {
		c.postToken = string(body)
	}
}

func (c *apiClient) userInfo() (string, models.UserId, int) {
	status, body := c.get("greader user-info", "user-info", url.Values{"output": {"json"}})
	var info struct {
		UserId   string `json:"userId"`
		UserName string `json:"userName"`
	}
	if status == http.StatusOK {
		_ = json.Unmarshal(body, &info)
	}
	return info.UserName, models.UserId(info.UserId), status
}

// streamIDs lists a stream's item IDs, n to a page, following continuations
// if follow is set.
func (c *apiClient) streamIDs(op, stream string, n int, follow bool) ([]int64, int) {
	var ids []int64
	form := url.Values{"s": {stream}, "n": {strconv.Itoa(n)}, "output": {"json"}}
	for {
		status, body := c.get(op, "stream/items/ids", form)
		if status != http.StatusOK {
			return ids, status
		}
		var resp struct {
			ItemRefs []struct {
				Id string `json:"id"`
			} `json:"itemRefs"`
			Continuation string `json:"continuation"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return ids, -1
		}
		for _, ref := range resp.ItemRefs {
			id, err := strconv.ParseInt(ref.Id, 10, 64)
			if err != nil {
				return ids, -1
			}
			ids = append(ids, id)
		}
		if !follow || resp.Continuation == "" {
			return ids, status
		}
		form.Set("c", resp.Continuation)
	}
}

type itemContent struct {
	id   int64
	feed int64
}

// contents fetches items by ID, returning each item's ID and feed.
func (c *apiClient) contents(ids []int64) ([]itemContent, int) {
	form := url.Values{}
	for _, id := range ids {
		form.Add("i", strconv.FormatInt(id, 16))
	}
	status, body := c.post("greader stream/items/contents", "stream/items/contents", form)
	if status != http.StatusOK {
		return nil, status
	}
	var resp struct {
		Items []struct {
			Id     string `json:"id"`
			Origin struct {
				StreamId string `json:"streamId"`
			} `json:"origin"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, -1
	}
	var items []itemContent
	for _, it := range resp.Items {
		id, err1 := strconv.ParseInt(it.Id[strings.LastIndex(it.Id, "/")+1:], 16, 64)
		feed, err2 := strconv.ParseInt(strings.TrimPrefix(it.Origin.StreamId, "feed/"), 10, 64)
		if err1 != nil || err2 != nil {
			return nil, -1
		}
		items = append(items, itemContent{id: id, feed: feed})
	}
	return items, status
}

func (c *apiClient) editTag(ids []int64, add, remove string) int {
	form := url.Values{}
	for _, id := range ids {
		form.Add("i", strconv.FormatInt(id, 16))
	}
	if add != "" {
		form.Set("a", add)
	}
	if remove != "" {
		form.Set("r", remove)
	}
	status, _ := c.post("greader edit-tag", "edit-tag", form)
	return status
}

// quickAdd subscribes to a URL, returning the feed ID it was given.
func (c *apiClient) quickAdd(feedURL string) (int64, int) {
	status, body := c.post("greader subscription/quickadd", "subscription/quickadd", url.Values{"quickadd": {feedURL}})
	if status != http.StatusOK {
		return 0, status
	}
	var resp struct {
		StreamId string `json:"streamId"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, -1
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(resp.StreamId, "feed/"), 10, 64)
	if err != nil {
		return 0, -1
	}
	return id, status
}

func (c *apiClient) subscriptionEdit(form url.Values) int {
	status, _ := c.post("greader subscription/edit", "subscription/edit", form)
	return status
}

type subscription struct {
	feed   int64
	title  string
	folder int64
}

func (c *apiClient) subscriptions() ([]subscription, int) {
	status, body := c.get("greader subscription/list", "subscription/list", url.Values{"output": {"json"}})
	if status != http.StatusOK {
		return nil, status
	}
	var resp struct {
		Subscriptions []struct {
			Id         string `json:"id"`
			Title      string `json:"title"`
			Categories []struct {
				Id string `json:"id"`
			} `json:"categories"`
		} `json:"subscriptions"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, -1
	}
	var subs []subscription
	for _, s := range resp.Subscriptions {
		feed, err := strconv.ParseInt(strings.TrimPrefix(s.Id, "feed/"), 10, 64)
		if err != nil || len(s.Categories) != 1 {
			return nil, -1
		}
		folder, err := strconv.ParseInt(strings.TrimPrefix(s.Categories[0].Id, labelPrefix), 10, 64)
		if err != nil {
			return nil, -1
		}
		subs = append(subs, subscription{feed: feed, title: s.Title, folder: folder})
	}
	return subs, status
}

// tags lists the folder IDs tag/list reports, by label.
func (c *apiClient) tags() (map[string]int64, int) {
	status, body := c.get("greader tag/list", "tag/list", url.Values{"output": {"json"}})
	if status != http.StatusOK {
		return nil, status
	}
	var resp struct {
		Tags []struct {
			Id    string `json:"id"`
			Label string `json:"label"`
		} `json:"tags"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, -1
	}
	folders := map[string]int64{}
	for _, tag := range resp.Tags {
		if id, err := strconv.ParseInt(strings.TrimPrefix(tag.Id, labelPrefix), 10, 64); err == nil {
			folders[tag.Label] = id
		}
	}
	return folders, status
}

// clientLogin signs in as a GReader client does, returning the token.
func (e *e2eEnv) clientLogin(username, password string) (string, int) {
	c := e.newClient("")
	status, body, _ := c.send("greader ClientLogin", http.MethodPost, "/greader/accounts/ClientLogin",
		url.Values{"Email": {username}, "Passwd": {password}})
	if status != http.StatusOK {
		return "", status
	}
	var resp struct {
		Auth string `json:"Auth"`
	}
	if err := json.Unmarshal(body, &resp); err != nil || resp.Auth == "" {
		return "", -1
	}
	return resp.Auth, status
}

// webLogin signs in as the browser does, returning a client holding the
// session cookie.
func (e *e2eEnv) webLogin(username, password string) (*apiClient, int) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	c := &apiClient{env: e, http: &http.Client{Jar: jar, Timeout: time.Minute}}
	payload, _ := json.Marshal(map[string]string{"username": username, "password": password})

	start := time.Now()
	resp, err := c.http.Post(e.web.URL+"/auth", "application/json", bytes.NewReader(payload))
	if err != nil {
		e.tm.record("web login", time.Since(start), false)
		return nil, 0
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	e.tm.record("web login", time.Since(start), resp.StatusCode == http.StatusOK)
	return c, resp.StatusCode
}

// fever makes a Fever request, returning the decoded response.
func (e *e2eEnv) fever(op, key, query string, form url.Values) (map[string]any, int) {
	body := url.Values{"api_key": {key}}
	for k, v := range form {
		body[k] = v
	}
	start := time.Now()
	resp, err := http.Post(e.web.URL+"/fever/?api"+query, "application/x-www-form-urlencoded",
		strings.NewReader(body.Encode()))
	if err != nil {
		e.tm.record(op, time.Since(start), false)
		return nil, 0
	}
	defer resp.Body.Close()
	var decoded map[string]any
	err = json.NewDecoder(resp.Body).Decode(&decoded)
	e.tm.record(op, time.Since(start), resp.StatusCode == http.StatusOK && err == nil)
	return decoded, resp.StatusCode
}

func (e *e2eEnv) feverAuth(key string) float64 {
	resp, _ := e.fever("fever auth", key, "", nil)
	auth, _ := resp["auth"].(float64)
	return auth
}

/*******************************************************************************
 * Database checks
 *
 * These report failures with Errorf rather than Fatalf, since most are called
 * from goroutines acting for one user, where Fatalf cannot stop the test.
 ******************************************************************************/

func (e *e2eEnv) ids(t *testing.T, query string, args ...any) map[int64]bool {
	t.Helper()
	ids := map[int64]bool{}
	rows, err := e.raw.Query(query, args...)
	if err != nil {
		t.Errorf("query %q: %v", query, err)
		return ids
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			t.Errorf("query %q: %v", query, err)
			return ids
		}
		ids[id] = true
	}
	return ids
}

// labels maps the first column of each row, an ID, to the second.
func (e *e2eEnv) labels(t *testing.T, query string, args ...any) map[int64]string {
	t.Helper()
	labels := map[int64]string{}
	rows, err := e.raw.Query(query, args...)
	if err != nil {
		t.Errorf("query %q: %v", query, err)
		return labels
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var label string
		if err = rows.Scan(&id, &label); err != nil {
			t.Errorf("query %q: %v", query, err)
			return labels
		}
		labels[id] = label
	}
	return labels
}

// flags reads a boolean column of the given articles, by ID.
func (e *e2eEnv) flags(t *testing.T, u models.UserId, column string, ids []int64) map[int64]bool {
	t.Helper()
	got := map[int64]bool{}
	rows, err := e.raw.Query(fmt.Sprintf(`SELECT id, %s FROM Article WHERE userid = $1 AND id = ANY($2)`, column),
		u, pq.Array(ids))
	if err != nil {
		t.Errorf("reading %s: %v", column, err)
		return got
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var v bool
		if err = rows.Scan(&id, &v); err != nil {
			t.Errorf("reading %s: %v", column, err)
			return got
		}
		got[id] = v
	}
	return got
}

func (e *e2eEnv) scalar(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := e.raw.QueryRow(query, args...).Scan(&n); err != nil {
		t.Errorf("query %q: %v", query, err)
		return -1
	}
	return n
}

func (e *e2eEnv) unreadIDs(t *testing.T, u models.UserId) map[int64]bool {
	return e.ids(t, `SELECT a.id FROM Article a JOIN Feed f ON f.id = a.feed
		WHERE a.userid = $1 AND NOT a.read AND f.deleted IS NULL`, u)
}

func (e *e2eEnv) savedIDs(t *testing.T, u models.UserId) map[int64]bool {
	return e.ids(t, `SELECT a.id FROM Article a JOIN Feed f ON f.id = a.feed
		WHERE a.userid = $1 AND a.saved AND f.deleted IS NULL`, u)
}

func (e *e2eEnv) feedArticleIDs(t *testing.T, u models.UserId, feed int64) map[int64]bool {
	return e.ids(t, `SELECT id FROM Article WHERE userid = $1 AND feed = $2`, u, feed)
}

// liveFeeds maps each of a user's live feeds to its folder.
func (e *e2eEnv) liveFeeds(t *testing.T, u models.UserId) map[int64]int64 {
	t.Helper()
	feeds := map[int64]int64{}
	rows, err := e.raw.Query(`SELECT id, folder FROM Feed WHERE userid = $1 AND deleted IS NULL`, u)
	if err != nil {
		t.Errorf("listing feeds: %v", err)
		return feeds
	}
	defer rows.Close()
	for rows.Next() {
		var id, folder int64
		if err = rows.Scan(&id, &folder); err != nil {
			t.Errorf("listing feeds: %v", err)
			return feeds
		}
		feeds[id] = folder
	}
	return feeds
}

// unreadCounts counts every user's unread articles in live feeds.
func (e *e2eEnv) unreadCounts(t *testing.T) map[models.UserId]int64 {
	t.Helper()
	counts := map[models.UserId]int64{}
	rows, err := e.raw.Query(`SELECT a.userid, count(*) FROM Article a JOIN Feed f ON f.id = a.feed
		WHERE NOT a.read AND f.deleted IS NULL GROUP BY a.userid`)
	if err != nil {
		t.Errorf("counting unread: %v", err)
		return counts
	}
	defer rows.Close()
	for rows.Next() {
		var u models.UserId
		var n int64
		if err = rows.Scan(&u, &n); err != nil {
			t.Errorf("counting unread: %v", err)
			return counts
		}
		counts[u] = n
	}
	return counts
}

// sortedIDs returns up to n of a set's IDs, lowest first.
func sortedIDs(s map[int64]bool, n int) []int64 {
	ids := make([]int64, 0, len(s))
	for id := range s {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
	return ids[:min(n, len(ids))]
}

// limitedErrors reports the first few of what may be many failures of one
// kind, and how many more there were.
type limitedErrors struct {
	t     *testing.T
	limit int32
	n     atomic.Int32
}

func (l *limitedErrors) Errorf(format string, args ...any) {
	if l.n.Add(1) <= l.limit {
		l.t.Errorf(format, args...)
	}
}

func (l *limitedErrors) done() {
	if n := l.n.Load(); n > l.limit {
		l.t.Errorf("... and %d more failures like those", n-l.limit)
	}
}

// diff describes how two ID sets differ, or returns "" if they do not.
func diff(got, want map[int64]bool) string {
	var missing, extra []int64
	for id := range want {
		if !got[id] {
			missing = append(missing, id)
		}
	}
	for id := range got {
		if !want[id] {
			extra = append(extra, id)
		}
	}
	if len(missing) == 0 && len(extra) == 0 {
		return ""
	}
	sample := func(ids []int64) []int64 {
		sort.Slice(ids, func(a, b int) bool { return ids[a] < ids[b] })
		return ids[:min(len(ids), 5)]
	}
	return fmt.Sprintf("got %d, want %d; missing %d %v, unexpected %d %v",
		len(got), len(want), len(missing), sample(missing), len(extra), sample(extra))
}

func set(ids []int64) map[int64]bool {
	s := make(map[int64]bool, len(ids))
	for _, id := range ids {
		s[id] = true
	}
	return s
}

/*******************************************************************************
 * Timing
 ******************************************************************************/

type opStats struct {
	samples  []time.Duration
	failures int
}

type phaseTime struct {
	Name     string
	Duration time.Duration
}

// timings collects the latency of every request and the length of every
// phase.
type timings struct {
	mu     sync.Mutex
	ops    map[string]*opStats
	phases []phaseTime
}

func newTimings() *timings {
	return &timings{ops: map[string]*opStats{}}
}

func (tm *timings) record(op string, d time.Duration, ok bool) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	s := tm.ops[op]
	if s == nil {
		s = &opStats{}
		tm.ops[op] = s
	}
	s.samples = append(s.samples, d)
	if !ok {
		s.failures++
	}
}

// phase starts timing a phase, returning the function that ends it.
func (tm *timings) phase(name string) func() {
	start := time.Now()
	return func() {
		tm.mu.Lock()
		defer tm.mu.Unlock()
		tm.phases = append(tm.phases, phaseTime{Name: name, Duration: time.Since(start)})
	}
}

type opReport struct {
	Op       string  `json:"op"`
	Count    int     `json:"count"`
	NotOK    int     `json:"not_ok"`
	P50Ms    float64 `json:"p50_ms"`
	P90Ms    float64 `json:"p90_ms"`
	P99Ms    float64 `json:"p99_ms"`
	MaxMs    float64 `json:"max_ms"`
	TotalSec float64 `json:"total_s"`
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	i := int(math.Ceil(p*float64(len(sorted)))) - 1
	return sorted[max(0, min(i, len(sorted)-1))]
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// report logs a table of phase lengths and request latencies, and writes it as
// JSON if asked to.
func (tm *timings) report(t *testing.T, cfg e2eConfig, dataset map[string]int64) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	var ops []opReport
	for name, s := range tm.ops {
		sorted := append([]time.Duration(nil), s.samples...)
		sort.Slice(sorted, func(a, b int) bool { return sorted[a] < sorted[b] })
		var total time.Duration
		for _, d := range sorted {
			total += d
		}
		ops = append(ops, opReport{
			Op: name, Count: len(sorted), NotOK: s.failures,
			P50Ms: ms(percentile(sorted, 0.5)), P90Ms: ms(percentile(sorted, 0.9)),
			P99Ms: ms(percentile(sorted, 0.99)), MaxMs: ms(sorted[len(sorted)-1]),
			TotalSec: total.Seconds(),
		})
	}
	sort.Slice(ops, func(a, b int) bool { return ops[a].Op < ops[b].Op })

	var b strings.Builder
	fmt.Fprintf(&b, "\nConfig: %+v\nDataset: %v\n\n%-44s %10s\n", cfg, dataset, "PHASE", "WALL")
	for _, p := range tm.phases {
		fmt.Fprintf(&b, "%-44s %10s\n", p.Name, p.Duration.Round(time.Millisecond))
	}
	fmt.Fprintf(&b, "\n%-34s %7s %6s %9s %9s %9s %9s\n", "OPERATION", "COUNT", "NOT OK", "P50 ms", "P90 ms", "P99 ms", "MAX ms")
	for _, o := range ops {
		fmt.Fprintf(&b, "%-34s %7d %6d %9.1f %9.1f %9.1f %9.1f\n", o.Op, o.Count, o.NotOK, o.P50Ms, o.P90Ms, o.P99Ms, o.MaxMs)
	}
	t.Log(b.String())

	if path := os.Getenv("GOLIATH_E2E_REPORT"); path != "" {
		phases := map[string]float64{}
		for _, p := range tm.phases {
			phases[p.Name] = p.Duration.Seconds()
		}
		out, _ := json.MarshalIndent(map[string]any{
			"config": cfg, "dataset": dataset, "phases_s": phases, "ops": ops,
		}, "", "  ")
		if err := os.WriteFile(path, out, 0o644); err != nil {
			t.Errorf("writing report: %v", err)
		}
	}
}

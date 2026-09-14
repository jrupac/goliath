package main

import (
	"context"
	"fmt"
	"maps"
	"net/http"
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
	"github.com/jrupac/goliath/devseed"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestMultiUserEndToEnd creates users through the admin service and has them
// use the server at once: signing in every way there is, subscribing to feeds
// that overlap with one another's, reading and marking while those feeds
// update, trying to reach one another's data, and being signed out, re-keyed
// and deleted while the rest carry on.
//
// Users are given parts by position: the first two share a feed one of them
// leaves, the third is signed out, the fourth changes password, and the fifth
// is deleted.
func TestMultiUserEndToEnd(t *testing.T) {
	env := startE2E(t)
	defer func() { env.tm.report(t, env.cfg, datasetSummary(t, env)) }()

	var users []*tester
	if !t.Run("AddUsers", func(t *testing.T) { users = addUsers(t, env) }) {
		return
	}
	t.Run("AddUserRefusals", func(t *testing.T) { checkAddUserRefusals(t, env, users) })
	if !t.Run("SeedFromDatabase", func(t *testing.T) { seedUsers(t, env, users) }) {
		return
	}
	if !t.Run("SignIn", func(t *testing.T) { signIn(t, env, users) }) {
		return
	}
	if !t.Run("Subscribe", func(t *testing.T) { subscribe(t, env, users) }) {
		return
	}
	if !t.Run("InitialFetch", func(t *testing.T) { waitForInitialFetch(t, env, users) }) {
		return
	}
	t.Run("Consistency", func(t *testing.T) { checkConsistency(t, env, users) })
	t.Run("Isolation", func(t *testing.T) { checkIsolation(t, env, users) })
	t.Run("WorkloadWhileFeedsUpdate", func(t *testing.T) { runWorkload(t, env, users) })
	t.Run("UnsubscribeSharedFeed", func(t *testing.T) { unsubscribeShared(t, env, users) })
	t.Run("RevokeAndChangePassword", func(t *testing.T) { revokeAndChangePassword(t, env, users) })
	t.Run("DeleteUnderLoad", func(t *testing.T) { deleteUnderLoad(t, env, users) })
	t.Run("FinalConsistency", func(t *testing.T) {
		checkConsistency(t, env, users)
		checkNoDuplicates(t, env)
	})
}

func addUser(env *e2eEnv, name, password string) (*admin.AddUserResponse, error) {
	var resp *admin.AddUserResponse
	err := env.timed("admin AddUser", func() (err error) {
		resp, err = env.admin.AddUser(context.Background(), &admin.AddUserRequest{Username: name, Password: password})
		return err
	})
	return resp, err
}

func deleteUser(env *e2eEnv, name string) (*admin.DeleteUserResponse, error) {
	var resp *admin.DeleteUserResponse
	err := env.timed("admin DeleteUser", func() (err error) {
		resp, err = env.admin.DeleteUser(context.Background(), &admin.DeleteUserRequest{Username: name})
		return err
	})
	return resp, err
}

func listUsers(t *testing.T, env *e2eEnv) map[string]*admin.UserSummary {
	t.Helper()
	var resp *admin.ListUsersResponse
	if err := env.timed("admin ListUsers", func() (err error) {
		resp, err = env.admin.ListUsers(context.Background(), &admin.ListUsersRequest{})
		return err
	}); err != nil {
		t.Errorf("ListUsers: %v", err)
		return nil
	}
	byName := map[string]*admin.UserSummary{}
	for _, u := range resp.User {
		byName[u.Username] = u
	}
	return byName
}

func addUsers(t *testing.T, env *e2eEnv) []*tester {
	users := make([]*tester, env.cfg.Users)
	end := env.tm.phase("add users")
	parallel(len(users), func(i int) {
		name := fmt.Sprintf("tester%02d", i+1)
		password := name + "-password"
		resp, err := addUser(env, name, password)
		if err != nil {
			t.Errorf("AddUser(%s): %v", name, err)
			return
		}
		u, err := env.d.GetUserByUsername(name)
		if err != nil || string(u.UserId) != resp.UserId {
			t.Errorf("%s reads back as %q (%v), want ID %q", name, u.UserId, err, resp.UserId)
			return
		}
		users[i] = &tester{name: name, password: password, user: u,
			feverKey: models.DeriveUserKey(name, models.Secret(password)).Reveal()}
	})
	end()
	if t.Failed() {
		t.FailNow()
	}

	summaries := listUsers(t, env)
	for _, u := range users {
		s, ok := summaries[u.name]
		switch {
		case !ok:
			t.Errorf("ListUsers is missing %s", u.name)
		case s.UserId != string(u.user.UserId):
			t.Errorf("ListUsers gives %s the ID %s, want %s", u.name, s.UserId, u.user.UserId)
		case s.FeedCount != 0 || s.FolderCount != 0 || s.ArticleCount != 0 || s.SessionCount != 0:
			t.Errorf("new user %s starts with %v, want nothing", u.name, s)
		}
	}
	return users
}

func checkAddUserRefusals(t *testing.T, env *e2eEnv, users []*tester) {
	for _, c := range []struct {
		name, password string
		want           codes.Code
	}{
		{users[0].name, "other", codes.AlreadyExists},
		{strings.ToUpper(users[0].name), "other", codes.AlreadyExists},
		{"", "pw", codes.InvalidArgument},
		{"has:colon", "pw", codes.InvalidArgument},
		{" padded", "pw", codes.InvalidArgument},
		{"bell\a", "pw", codes.InvalidArgument},
		{"ünïcode", "pw", codes.InvalidArgument},
		{"with space", "pw", codes.InvalidArgument},
		{"nopassword", "", codes.InvalidArgument},
	} {
		if _, err := addUser(env, c.name, c.password); status.Code(err) != c.want {
			t.Errorf("AddUser(%q): %v, want %v", c.name, err, c.want)
		}
	}
	// A refused add must leave the account it collided with alone.
	if _, st := env.clientLogin(users[0].name, users[0].password); st != http.StatusOK {
		t.Errorf("%s cannot sign in after a colliding add: %d", users[0].name, st)
	}

	// Of several adds at once of one name in different cases, exactly one
	// wins, and the rest are told the name is taken rather than that
	// something went wrong.
	variants := []string{"Racer", "racer", "RACER", "rAcEr", "RaCeR", "raceR", "rACER", "Racer"}
	got := make([]codes.Code, len(variants))
	parallel(len(variants), func(i int) {
		_, err := addUser(env, variants[i], "pw")
		got[i] = status.Code(err)
	})
	var won, taken int
	for _, c := range got {
		switch c {
		case codes.OK:
			won++
		case codes.AlreadyExists:
			taken++
		}
	}
	if won != 1 || taken != len(variants)-1 {
		t.Errorf("racing adds of one name: %v, want one OK and the rest AlreadyExists", got)
	}

	// Whichever won is removed again, which is also a user holding nothing
	// but their root folder being deleted.
	for name := range listUsers(t, env) {
		if strings.EqualFold(name, "racer") {
			if resp, err := deleteUser(env, name); err != nil || resp.FeedCount != 0 || resp.ArticleCount != 0 {
				t.Errorf("deleting %s: %v, %v", name, resp, err)
			}
		}
	}
	purge(t, env)
	if n := env.scalar(t, `SELECT count(*) FROM UserTable WHERE lower(username) = 'racer'`); n != 0 {
		t.Errorf("%d racing users left behind", n)
	}
}

func seedUsers(t *testing.T, env *e2eEnv, users []*tester) {
	source := os.Getenv("GOLIATH_E2E_SEED_DB")
	if source == "" {
		t.Skip("GOLIATH_E2E_SEED_DB not set; users start with no feeds of their own")
	}
	var all []models.User
	for _, u := range users {
		all = append(all, u.user)
	}

	end := env.tm.phase("seed users from database")
	copied, err := devseed.Seed(context.Background(), env.raw, all, devseed.Options{
		SourceDB: source,
		Share:    env.cfg.SeedShare,
		// Every copy is served, with no items, by the local server, so that
		// nothing is fetched from the real publishers.
		Relocate: func(f devseed.SourceFeed) (string, string) {
			feedURL := env.feeds.Add(fmt.Sprintf("/seeded/%d", f.ID), f.Title, 0)
			return feedURL, feedURL + "/site"
		},
	})
	end()
	if err != nil {
		t.Fatalf("seeding from %s: %v", source, err)
	}

	byID := map[models.UserId]*tester{}
	for _, u := range users {
		byID[u.user.UserId] = u
	}
	var articles, unread int64
	for _, c := range copied {
		u := byID[c.User.UserId]
		u.seeded = append(u.seeded, c)
		articles += c.Articles
		unread += c.Unread
	}
	t.Logf("Copied %d feeds holding %d articles, %d unread, across %d users", len(copied), articles, unread, len(users))
}

func signIn(t *testing.T, env *e2eEnv, users []*tester) {
	end := env.tm.phase("sign in every user every way")
	defer end()
	parallel(len(users), func(i int) {
		u := users[i]
		other := users[(i+1)%len(users)]

		token, st := env.clientLogin(u.name, u.password)
		if st != http.StatusOK {
			t.Errorf("ClientLogin as %s: %d", u.name, st)
			return
		}
		u.api = env.newClient(token)
		if name, id, st := u.api.userInfo(); st != http.StatusOK || name != u.name || id != u.user.UserId {
			t.Errorf("user-info for %s by token: %d, %q, %q", u.name, st, name, id)
		}
		for _, wrong := range []string{u.password + "x", other.password, ""} {
			if _, st := env.clientLogin(u.name, wrong); st != http.StatusUnauthorized {
				t.Errorf("ClientLogin as %s with a wrong password: %d, want 401", u.name, st)
			}
		}

		if auth := env.feverAuth(u.feverKey); auth != 1 {
			t.Errorf("Fever with %s's key: auth=%v", u.name, auth)
		}
		wrongKey := models.DeriveUserKey(u.name, models.Secret(other.password)).Reveal()
		if auth := env.feverAuth(wrongKey); auth != 0 {
			t.Errorf("Fever with a wrong key for %s: auth=%v", u.name, auth)
		}

		web, st := env.webLogin(u.name, u.password)
		if st != http.StatusOK {
			t.Errorf("web login as %s: %d", u.name, st)
			return
		}
		u.web = web
		if name, _, st := u.web.userInfo(); st != http.StatusOK || name != u.name {
			t.Errorf("user-info for %s by cookie: %d, %q", u.name, st, name)
		}
		if _, st := env.webLogin(u.name, other.password); st != http.StatusUnauthorized {
			t.Errorf("web login as %s with a wrong password: %d, want 401", u.name, st)
		}
	})
	if t.Failed() {
		t.FailNow()
	}
}

func subscribe(t *testing.T, env *e2eEnv, users []*tester) {
	cfg := env.cfg
	for p := range cfg.PoolFeeds {
		env.feeds.Add(poolPath(p), fmt.Sprintf("Pool feed %d", p), cfg.Items)
	}
	// Windows of the pool spaced this far apart overlap, so most feeds have
	// several subscribers.
	stride := max(1, cfg.PoolFeeds/cfg.Users)

	end := env.tm.phase("subscribe to and file every pool feed")
	parallel(len(users), func(i int) {
		u := users[i]
		var mu sync.Mutex
		var wg sync.WaitGroup
		// A few at once per user, so that one user's folder is created by
		// concurrent requests naming it.
		sem := make(chan struct{}, 4)
		for j := range cfg.FeedsPerUser {
			p := (i*stride + j) % cfg.PoolFeeds
			wg.Go(func() {
				sem <- struct{}{}
				defer func() { <-sem }()
				feedURL := env.feeds.URL(poolPath(p))
				id, st := u.api.quickAdd(feedURL)
				if st != http.StatusOK {
					t.Errorf("%s adding %s: %d", u.name, feedURL, st)
					return
				}
				label := fmt.Sprintf("%sPool %d", labelPrefix, j%3)
				if st := u.api.subscriptionEdit(url.Values{
					"ac": {"edit"}, "s": {fmt.Sprintf("feed/%d", id)}, "a": {label},
				}); st != http.StatusOK {
					t.Errorf("%s filing feed %d under %s: %d", u.name, id, label, st)
				}
				mu.Lock()
				u.pool = append(u.pool, poolSub{index: p, url: feedURL, feedID: id})
				mu.Unlock()
			})
		}
		wg.Wait()
		sort.Slice(u.pool, func(a, b int) bool { return u.pool[a].index < u.pool[b].index })

		if len(u.pool) > 0 {
			if id, st := u.api.quickAdd(u.pool[0].url); st != http.StatusOK || id != u.pool[0].feedID {
				t.Errorf("%s adding %s again: feed %d (%d), want the existing %d",
					u.name, u.pool[0].url, id, st, u.pool[0].feedID)
			}
		}
	})
	end()
	if t.Failed() {
		t.FailNow()
	}
}

func waitForInitialFetch(t *testing.T, env *e2eEnv, users []*tester) {
	want := map[models.UserId]int64{}
	for _, u := range users {
		n := int64(len(u.pool) * env.cfg.Items)
		for _, c := range u.seeded {
			n += c.Unread
		}
		want[u.user.UserId] = n
	}
	env.waitFor(t, "first fetch of every subscription", func() (bool, string) {
		got := env.unreadCounts(t)
		for _, u := range users {
			if got[u.user.UserId] != want[u.user.UserId] {
				return false, fmt.Sprintf("%s has %d unread, want %d", u.name, got[u.user.UserId], want[u.user.UserId])
			}
		}
		return true, ""
	})
}

// checkConsistency compares what each user is shown, through every API, with
// what the database holds for them.
func checkConsistency(t *testing.T, env *e2eEnv, users []*tester) {
	end := env.tm.phase("compare every user's view with the database")
	defer end()
	summaries := listUsers(t, env)

	parallel(len(users), func(i int) {
		u := users[i]
		if u.deleted {
			return
		}
		id := u.user.UserId
		want := env.unreadIDs(t, id)

		got, st := u.api.streamIDs("greader reading-list", readingList, storage.MaxFetchedRows, true)
		if st != http.StatusOK {
			t.Errorf("%s reading list: %d", u.name, st)
		} else if d := diff(set(got), want); d != "" {
			t.Errorf("%s reading list against the database: %s", u.name, d)
		} else if len(got) != len(want) {
			t.Errorf("%s reading list repeats %d items", u.name, len(got)-len(want))
		}

		paged, st := u.api.streamIDs("greader reading-list paged", readingList, 97, true)
		if st != http.StatusOK {
			t.Errorf("%s reading list in pages: %d", u.name, st)
		} else if d := diff(set(paged), want); d != "" {
			t.Errorf("%s reading list in pages of 97: %s", u.name, d)
		} else if len(paged) != len(want) {
			t.Errorf("%s reading list in pages repeats %d items", u.name, len(paged)-len(want))
		}

		saved, st := u.api.streamIDs("greader starred", starred, storage.MaxFetchedRows, true)
		if st != http.StatusOK {
			t.Errorf("%s starred: %d", u.name, st)
		} else if d := diff(set(saved), env.savedIDs(t, id)); d != "" {
			t.Errorf("%s starred against the database: %s", u.name, d)
		}

		resp, st := env.fever("fever unread_item_ids", u.feverKey, "&unread_item_ids", nil)
		feverIDs := map[int64]bool{}
		if s, _ := resp["unread_item_ids"].(string); s != "" {
			for _, f := range strings.Split(s, ",") {
				n, _ := strconv.ParseInt(f, 10, 64)
				feverIDs[n] = true
			}
		}
		if st != http.StatusOK {
			t.Errorf("%s Fever unread: %d", u.name, st)
		} else if d := diff(feverIDs, want); d != "" {
			t.Errorf("%s Fever unread against the database: %s", u.name, d)
		}

		live := env.liveFeeds(t, id)
		subs, st := u.api.subscriptions()
		gotFeeds := map[int64]int64{}
		for _, s := range subs {
			gotFeeds[s.feed] = s.folder
		}
		if st != http.StatusOK {
			t.Errorf("%s subscription list: %d", u.name, st)
		} else if !maps.Equal(gotFeeds, live) {
			t.Errorf("%s subscription list has %d feeds, the database %d live, or their folders differ",
				u.name, len(gotFeeds), len(live))
		}

		folders := env.ids(t, `SELECT id FROM Folder WHERE userid = $1`, id)
		tags, st := u.api.tags()
		gotFolders := map[int64]bool{}
		for _, f := range tags {
			gotFolders[f] = true
		}
		if st != http.StatusOK {
			t.Errorf("%s tag list: %d", u.name, st)
		} else if d := diff(gotFolders, folders); d != "" {
			t.Errorf("%s tag list against the database's folders: %s", u.name, d)
		}

		s := summaries[u.name]
		articles := env.scalar(t, `SELECT count(*) FROM Article a JOIN Feed f ON f.id = a.feed
			WHERE a.userid = $1 AND f.deleted IS NULL`, id)
		switch {
		case s == nil:
			t.Errorf("ListUsers is missing %s", u.name)
		case s.FeedCount != int64(len(live)) || s.FolderCount != int64(len(folders)-1) ||
			s.ArticleCount != articles || s.UnreadCount != int64(len(want)):
			t.Errorf("ListUsers says %s has %v; the database has %d feeds, %d folders, %d articles, %d unread",
				u.name, s, len(live), len(folders)-1, articles, len(want))
		}
	})
}

// userState is everything about a user that another user must not be able to
// change.
type userState struct {
	unread, saved map[int64]bool
	feeds         map[int64]string
	folders       map[int64]string
}

func snapshot(t *testing.T, env *e2eEnv, u *tester) userState {
	id := u.user.UserId
	return userState{
		unread: env.unreadIDs(t, id),
		saved:  env.savedIDs(t, id),
		feeds: env.labels(t, `SELECT id, title || ' in ' || folder::STRING || ' at ' || url FROM Feed
			WHERE userid = $1 AND deleted IS NULL`, id),
		folders: env.labels(t, `SELECT id, name FROM Folder WHERE userid = $1`, id),
	}
}

// checkIsolation has each user try, through every endpoint that takes an ID,
// to read or change the next user's articles, feeds and folders, and then
// checks that nobody's data changed.
func checkIsolation(t *testing.T, env *e2eEnv, users []*tester) {
	before := make([]userState, len(users))
	for i, u := range users {
		before[i] = snapshot(t, env, u)
	}

	end := env.tm.phase("every user reaches for another's data")
	parallel(len(users), func(i int) {
		victim, b := users[i], users[(i+1)%len(users)]
		targets := sortedIDs(before[i].unread, 5)
		if len(targets) == 0 || len(victim.pool) == 0 {
			t.Errorf("%s has nothing to aim at", victim.name)
			return
		}
		feed := fmt.Sprintf("feed/%d", victim.pool[0].feedID)
		folderID := env.scalar(t, `SELECT id FROM Folder WHERE userid = $1 AND name = 'Pool 0'`, victim.user.UserId)
		folder := fmt.Sprintf("%s%d", labelPrefix, folderID)

		if items, st := b.api.contents(targets); st != http.StatusOK || len(items) != 0 {
			t.Errorf("%s reading %s's items: %d with %d items, want 200 with none", b.name, victim.name, st, len(items))
		}
		b.api.editTag(targets, readState, "")
		b.api.editTag(targets, starred, "")
		if _, st := b.api.streamIDs("greader feed stream", feed, 100, false); st != http.StatusNotFound {
			t.Errorf("%s reading %s's feed stream: %d, want 404", b.name, victim.name, st)
		}
		if _, st := b.api.streamIDs("greader folder stream", folder, 100, false); st != http.StatusNotFound {
			t.Errorf("%s reading %s's folder stream: %d, want 404", b.name, victim.name, st)
		}
		b.api.post("greader mark-all-as-read", "mark-all-as-read", url.Values{"s": {feed}})
		b.api.post("greader mark-all-as-read", "mark-all-as-read", url.Values{"t": {folder}})

		for _, form := range []url.Values{
			{"ac": {"edit"}, "s": {feed}, "t": {"Renamed by " + b.name}},
			{"ac": {"edit"}, "s": {feed}, "a": {labelPrefix + "Taken by " + b.name}},
			{"ac": {"unsubscribe"}, "s": {feed}},
		} {
			if st := b.api.subscriptionEdit(form); st == http.StatusOK {
				t.Errorf("%s editing %s's feed with %v succeeded", b.name, victim.name, form)
			}
		}
		if st, _ := b.api.post("greader rename-tag", "rename-tag",
			url.Values{"s": {folder}, "dest": {"Renamed by " + b.name}}); st == http.StatusOK {
			t.Errorf("%s renaming %s's folder succeeded", b.name, victim.name)
		}
		if st, _ := b.api.post("greader disable-tag", "disable-tag", url.Values{"s": {folder}}); st == http.StatusOK {
			t.Errorf("%s removing %s's folder succeeded", b.name, victim.name)
		}

		for _, form := range []url.Values{
			{"mark": {"item"}, "as": {"read"}, "id": {strconv.FormatInt(targets[0], 10)}},
			{"mark": {"item"}, "as": {"saved"}, "id": {strconv.FormatInt(targets[0], 10)}},
			{"mark": {"feed"}, "as": {"read"}, "id": {strconv.FormatInt(victim.pool[0].feedID, 10)}},
			{"mark": {"group"}, "as": {"read"}, "id": {strconv.FormatInt(folderID, 10)}},
		} {
			env.fever("fever mark", b.feverKey, "", form)
		}

		// Two subscriptions to one address are two feeds.
		for _, x := range victim.pool {
			for _, y := range b.pool {
				if x.index == y.index && x.feedID == y.feedID {
					t.Errorf("%s and %s share feed ID %d for %s", victim.name, b.name, x.feedID, x.url)
				}
			}
		}
	})
	end()

	for i, u := range users {
		after := snapshot(t, env, u)
		if d := diff(after.unread, before[i].unread); d != "" {
			t.Errorf("%s's unread articles changed: %s", u.name, d)
		}
		if d := diff(after.saved, before[i].saved); d != "" {
			t.Errorf("%s's starred articles changed: %s", u.name, d)
		}
		if !maps.Equal(after.feeds, before[i].feeds) {
			t.Errorf("%s's feeds changed:\nbefore %v\nafter  %v", u.name, before[i].feeds, after.feeds)
		}
		if !maps.Equal(after.folders, before[i].folders) {
			t.Errorf("%s's folders changed:\nbefore %v\nafter  %v", u.name, before[i].folders, after.folders)
		}
	}
}

// workloadModel is what one user did during the workload, to be checked
// against the database afterwards.
type workloadModel struct {
	read, star map[int64]bool
	// feeds are the user's own, which every item they are served must be in.
	feeds map[int64]int64
	// markAll is the feed they mark read wholesale from time to time. Its
	// items are left out of read and star, which it would overturn.
	markAll int64
}

// runWorkload has every user make a mix of reads and writes, round after
// round, while several shared feeds publish new items and are fetched for all
// their subscribers.
func runWorkload(t *testing.T, env *e2eEnv, users []*tester) {
	cfg := env.cfg
	type subscriber struct {
		u    *tester
		feed int64
	}
	subscribers := map[int][]subscriber{}
	for _, u := range users {
		for _, s := range u.pool {
			subscribers[s.index] = append(subscribers[s.index], subscriber{u, s.feedID})
		}
	}
	var shared []int
	for p, subs := range subscribers {
		if len(subs) >= 2 {
			shared = append(shared, p)
		}
	}
	sort.Slice(shared, func(a, b int) bool {
		if la, lb := len(subscribers[shared[a]]), len(subscribers[shared[b]]); la != lb {
			return la > lb
		}
		return shared[a] < shared[b]
	})
	shared = shared[:min(len(shared), 5)]
	const waves, perWave = 3, 3

	workloads := make([]*workloadModel, len(users))
	for i, u := range users {
		workloads[i] = &workloadModel{read: map[int64]bool{}, star: map[int64]bool{},
			feeds: env.liveFeeds(t, u.user.UserId)}
		if len(u.pool) > 0 {
			workloads[i].markAll = u.pool[len(u.pool)-1].feedID
		}
	}

	errs := &limitedErrors{t: t, limit: 20}
	end := env.tm.phase("mixed requests from every user while feeds update")
	var wg sync.WaitGroup
	wg.Go(func() {
		for range waves {
			for _, p := range shared {
				_ = env.feeds.Publish(poolPath(p), perWave)
				for _, s := range subscribers[p] {
					env.sched.Schedule(s.u.user, models.FeedId(s.feed))
				}
			}
			time.Sleep(250 * time.Millisecond)
		}
	})
	for i, u := range users {
		wg.Go(func() { workload(env, u, workloads[i], errs) })
	}
	wg.Wait()
	end()
	errs.done()

	want := int64(cfg.Items + waves*perWave)
	env.waitFor(t, "published items reach every subscriber", func() (bool, string) {
		for _, p := range shared {
			for _, s := range subscribers[p] {
				n := env.scalar(t, `SELECT count(*) FROM Article WHERE userid = $1 AND feed = $2`, s.u.user.UserId, s.feed)
				if n != want {
					return false, fmt.Sprintf("%s has %d articles in feed %d, want %d", s.u.name, n, s.feed, want)
				}
			}
		}
		return true, ""
	})

	for i, u := range users {
		m := workloads[i]
		for column, wantState := range map[string]map[int64]bool{"read": m.read, "saved": m.star} {
			ids := make([]int64, 0, len(wantState))
			for id := range wantState {
				ids = append(ids, id)
			}
			got := env.flags(t, u.user.UserId, column, ids)
			for id, w := range wantState {
				if g, ok := got[id]; !ok || g != w {
					t.Errorf("%s article %d: %s is %v (present %v), want %v", u.name, id, column, g, ok, w)
				}
			}
		}
	}
}

func workload(env *e2eEnv, u *tester, m *workloadModel, errs *limitedErrors) {
	for r := range env.cfg.Rounds {
		ids, st := u.api.streamIDs("greader reading-list", readingList, 200, false)
		if st != http.StatusOK {
			errs.Errorf("%s reading list: %d", u.name, st)
			continue
		}
		if len(ids) == 0 {
			continue
		}
		items, st := u.api.contents(ids[:min(20, len(ids))])
		if st != http.StatusOK {
			errs.Errorf("%s item contents: %d", u.name, st)
			continue
		}
		var page []int64
		for _, it := range items {
			if _, mine := m.feeds[it.feed]; !mine {
				errs.Errorf("%s was served item %d from feed %d, which is not theirs", u.name, it.id, it.feed)
			}
			if it.feed != m.markAll {
				page = append(page, it.id)
			}
		}
		if len(page) < 2 {
			continue
		}

		toRead := page[:min(5, len(page)-1)]
		if st := u.api.editTag(toRead, readState, ""); st != http.StatusOK {
			errs.Errorf("%s marking read: %d", u.name, st)
		} else {
			for _, id := range toRead {
				m.read[id] = true
			}
		}
		if r%4 == 1 {
			if st := u.api.editTag(toRead[:1], "", readState); st != http.StatusOK {
				errs.Errorf("%s marking unread: %d", u.name, st)
			} else {
				m.read[toRead[0]] = false
			}
		}

		last := page[len(page)-1]
		if st := u.api.editTag([]int64{last}, starred, ""); st != http.StatusOK {
			errs.Errorf("%s starring: %d", u.name, st)
		} else {
			m.star[last] = true
		}
		if r%3 == 2 {
			if st := u.api.editTag([]int64{last}, "", starred); st != http.StatusOK {
				errs.Errorf("%s unstarring: %d", u.name, st)
			} else {
				m.star[last] = false
			}
		}

		if _, st := u.api.subscriptions(); st != http.StatusOK {
			errs.Errorf("%s subscription list: %d", u.name, st)
		}
		if _, st := u.api.tags(); st != http.StatusOK {
			errs.Errorf("%s tag list: %d", u.name, st)
		}
		if _, st := u.web.streamIDs("greader reading-list by cookie", readingList, 50, false); st != http.StatusOK {
			errs.Errorf("%s reading list by cookie: %d", u.name, st)
		}
		if resp, st := u.api.env.fever("fever items", u.feverKey, "&items", nil); st != http.StatusOK || resp["auth"] != float64(1) {
			errs.Errorf("%s Fever items: %d, auth %v", u.name, st, resp["auth"])
		}
		if r%5 == 4 && m.markAll != 0 {
			if st, _ := u.api.post("greader mark-all-as-read", "mark-all-as-read",
				url.Values{"s": {fmt.Sprintf("feed/%d", m.markAll)}}); st != http.StatusOK {
				errs.Errorf("%s marking a feed read: %d", u.name, st)
			}
		}
	}
}

// unsubscribeShared has one of two subscribers to a feed leave it, and checks
// that the other is unaffected and that coming back restores it.
func unsubscribeShared(t *testing.T, env *e2eEnv, users []*tester) {
	a, b := users[0], users[1]
	var sa, sb poolSub
	found := false
	for _, x := range a.pool {
		for _, y := range b.pool {
			if x.index == y.index {
				sa, sb, found = x, y, true
			}
		}
	}
	if !found {
		t.Skipf("%s and %s share no feed", a.name, b.name)
	}
	aArticles := env.feedArticleIDs(t, a.user.UserId, sa.feedID)
	bBefore := len(env.feedArticleIDs(t, b.user.UserId, sb.feedID))
	folder := env.liveFeeds(t, a.user.UserId)[sa.feedID]
	feed := fmt.Sprintf("feed/%d", sa.feedID)

	if st := a.api.subscriptionEdit(url.Values{"ac": {"unsubscribe"}, "s": {feed}}); st != http.StatusOK {
		t.Fatalf("%s unsubscribing: %d", a.name, st)
	}
	ids, _ := a.api.streamIDs("greader reading-list", readingList, storage.MaxFetchedRows, true)
	for _, id := range ids {
		if aArticles[id] {
			t.Errorf("%s's reading list still has item %d from the feed they left", a.name, id)
			break
		}
	}
	if _, st := a.api.streamIDs("greader feed stream", feed, 10, false); st != http.StatusNotFound {
		t.Errorf("%s reading the feed they left: %d, want 404", a.name, st)
	}

	// Updated while a is away: b gets the new items and a does not, even
	// when a fetch for a's copy is asked for, as a stale one would be.
	_ = env.feeds.Publish(poolPath(sa.index), 2)
	env.sched.Schedule(a.user, models.FeedId(sa.feedID))
	env.sched.Schedule(b.user, models.FeedId(sb.feedID))
	env.waitFor(t, "update reaches the subscriber who stayed", func() (bool, string) {
		if n := len(env.feedArticleIDs(t, b.user.UserId, sb.feedID)); n != bBefore+2 {
			return false, fmt.Sprintf("%s has %d, want %d", b.name, n, bBefore+2)
		}
		return true, ""
	})
	if n := len(env.feedArticleIDs(t, a.user.UserId, sa.feedID)); n != len(aArticles) {
		t.Errorf("%s's copy went from %d to %d articles while unsubscribed", a.name, len(aArticles), n)
	}

	// Coming back restores the same feed, in the same folder, and catches up.
	id, st := a.api.quickAdd(sa.url)
	if st != http.StatusOK || id != sa.feedID {
		t.Fatalf("%s re-adding %s: feed %d (%d), want %d restored", a.name, sa.url, id, st, sa.feedID)
	}
	env.waitFor(t, "restored feed catches up", func() (bool, string) {
		if n := len(env.feedArticleIDs(t, a.user.UserId, sa.feedID)); n != len(aArticles)+2 {
			return false, fmt.Sprintf("%s has %d, want %d", a.name, n, len(aArticles)+2)
		}
		return true, ""
	})
	if got := env.liveFeeds(t, a.user.UserId)[sa.feedID]; got != folder {
		t.Errorf("%s's restored feed is in folder %d, want %d", a.name, got, folder)
	}
}

func revokeAndChangePassword(t *testing.T, env *e2eEnv, users []*tester) {
	ctx := context.Background()
	c, d, bystander := users[2], users[3], users[5]

	var sessions *admin.ListSessionsResponse
	if err := env.timed("admin ListSessions", func() (err error) {
		sessions, err = env.admin.ListSessions(ctx, &admin.ListSessionsRequest{Username: c.name})
		return err
	}); err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	schemes := map[admin.AuthScheme]bool{}
	for _, s := range sessions.Session {
		schemes[s.Scheme] = true
	}
	if !schemes[admin.AuthScheme_AUTH_SCHEME_GREADER] || !schemes[admin.AuthScheme_AUTH_SCHEME_WEB] {
		t.Errorf("%s's sessions %v lack a GReader or a web one", c.name, sessions.Session)
	}

	var revoked *admin.RevokeSessionsResponse
	if err := env.timed("admin RevokeSessions", func() (err error) {
		revoked, err = env.admin.RevokeSessions(ctx, &admin.RevokeSessionsRequest{
			Username: c.name, Target: &admin.RevokeSessionsRequest_All{All: &admin.AllSessions{}}})
		return err
	}); err != nil {
		t.Fatalf("RevokeSessions: %v", err)
	}
	if revoked.RevokedCount != int64(len(sessions.Session)) {
		t.Errorf("revoked %d of %s's %d sessions", revoked.RevokedCount, c.name, len(sessions.Session))
	}
	if _, _, st := c.api.userInfo(); st != http.StatusUnauthorized {
		t.Errorf("%s's revoked token: %d, want 401", c.name, st)
	}
	if _, _, st := c.web.userInfo(); st != http.StatusUnauthorized {
		t.Errorf("%s's revoked cookie: %d, want 401", c.name, st)
	}
	if _, _, st := bystander.api.userInfo(); st != http.StatusOK {
		t.Errorf("%s after %s was signed out: %d", bystander.name, c.name, st)
	}

	// Signing in again works, and the post token held from before, which names
	// the revoked session, is refused as stale and replaced.
	token, st := env.clientLogin(c.name, c.password)
	if st != http.StatusOK {
		t.Fatalf("%s signing in again: %d", c.name, st)
	}
	c.api.auth = token
	if target := sortedIDs(env.unreadIDs(t, c.user.UserId), 1); len(target) == 1 {
		if st := c.api.editTag(target, readState, ""); st != http.StatusOK {
			t.Errorf("%s writing after signing in again: %d", c.name, st)
		}
		c.api.editTag(target, "", readState)
	}
	if c.web, st = env.webLogin(c.name, c.password); st != http.StatusOK {
		t.Fatalf("%s web login again: %d", c.name, st)
	}

	newPassword := d.password + "-changed"
	var changed *admin.ChangePasswordResponse
	if err := env.timed("admin ChangePassword", func() (err error) {
		changed, err = env.admin.ChangePassword(ctx, &admin.ChangePasswordRequest{Username: d.name, Password: newPassword})
		return err
	}); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if changed.RevokedSessionCount < 2 {
		t.Errorf("changing %s's password revoked %d sessions, want at least 2", d.name, changed.RevokedSessionCount)
	}
	if _, _, st := d.api.userInfo(); st != http.StatusUnauthorized {
		t.Errorf("%s's token after a password change: %d, want 401", d.name, st)
	}
	if _, st := env.clientLogin(d.name, d.password); st != http.StatusUnauthorized {
		t.Errorf("%s's old password: %d, want 401", d.name, st)
	}
	if _, st := env.webLogin(d.name, d.password); st != http.StatusUnauthorized {
		t.Errorf("%s's old password on the web: %d, want 401", d.name, st)
	}
	if auth := env.feverAuth(d.feverKey); auth != 0 {
		t.Errorf("%s's old Fever key: auth=%v, want 0", d.name, auth)
	}
	d.password = newPassword
	d.feverKey = models.DeriveUserKey(d.name, models.Secret(newPassword)).Reveal()
	if auth := env.feverAuth(d.feverKey); auth != 1 {
		t.Errorf("%s's new Fever key: auth=%v, want 1", d.name, auth)
	}
	if token, st = env.clientLogin(d.name, newPassword); st != http.StatusOK {
		t.Fatalf("%s's new password: %d", d.name, st)
	}
	d.api = env.newClient(token)
	if d.web, st = env.webLogin(d.name, newPassword); st != http.StatusOK {
		t.Fatalf("%s's new password on the web: %d", d.name, st)
	}
	if _, _, st := bystander.api.userInfo(); st != http.StatusOK {
		t.Errorf("%s after %s changed password: %d", bystander.name, d.name, st)
	}
}

// deleteUnderLoad deletes a user while they and everyone else are making
// requests and their feeds are being fetched.
func deleteUnderLoad(t *testing.T, env *e2eEnv, users []*tester) {
	victim := users[4]
	before := listUsers(t, env)[victim.name]
	feeds := env.liveFeeds(t, victim.user.UserId)

	var (
		stop         = make(chan struct{})
		deletedAt    atomic.Int64
		othersFailed atomic.Int64
		servedAfter  atomic.Int64
		refusedAfter atomic.Int64
		wg           sync.WaitGroup
		stopped      = func() bool {
			select {
			case <-stop:
				return true
			default:
				return false
			}
		}
	)
	for _, u := range users {
		if u == victim || u.deleted {
			continue
		}
		wg.Go(func() {
			for !stopped() {
				if _, st := u.api.streamIDs("greader reading-list", readingList, 100, false); st != http.StatusOK {
					othersFailed.Add(1)
				}
				if _, st := u.api.subscriptions(); st != http.StatusOK {
					othersFailed.Add(1)
				}
			}
		})
	}
	for _, c := range []*apiClient{victim.api, victim.web} {
		wg.Go(func() {
			for !stopped() {
				started := time.Now().UnixNano()
				_, st := c.streamIDs("greader reading-list of deleted user", readingList, 100, false)
				// Only requests that began after the deletion returned have
				// a defined answer; one in flight across it may go either way.
				if at := deletedAt.Load(); at != 0 && started > at {
					if st == http.StatusOK {
						servedAfter.Add(1)
					} else {
						refusedAfter.Add(1)
					}
				}
			}
		})
	}
	// Their feeds are fetched right up to the deletion, so that a fetch may be
	// in flight across it. Nothing asks for them afterwards.
	wg.Go(func() {
		for !stopped() && deletedAt.Load() == 0 {
			for feed := range feeds {
				env.sched.Schedule(victim.user, models.FeedId(feed))
			}
			time.Sleep(20 * time.Millisecond)
		}
	})

	time.Sleep(300 * time.Millisecond)
	resp, err := deleteUser(env, victim.name)
	deletedAt.Store(time.Now().UnixNano())
	if err == nil {
		victim.deleted = true
	}
	time.Sleep(500 * time.Millisecond)
	close(stop)
	wg.Wait()
	if err != nil {
		t.Fatalf("DeleteUser(%s): %v", victim.name, err)
	}

	if before != nil && (resp.FeedCount != before.FeedCount || resp.ArticleCount != before.ArticleCount ||
		resp.SessionCount != before.SessionCount) {
		t.Errorf("deleting %s reported %v; ListUsers had said %v", victim.name, resp, before)
	}
	if n := othersFailed.Load(); n != 0 {
		t.Errorf("%d requests by other users failed while %s was deleted", n, victim.name)
	}
	if n := servedAfter.Load(); n != 0 {
		t.Errorf("%d requests by %s were served after the deletion", n, victim.name)
	}
	t.Logf("%s's requests begun after the deletion: %d refused", victim.name, refusedAfter.Load())
	// However many requests the loops managed to begin afterwards, every
	// credential the user held is now refused.
	if _, _, st := victim.api.userInfo(); st != http.StatusUnauthorized {
		t.Errorf("the deleted %s's token: %d, want 401", victim.name, st)
	}
	if _, _, st := victim.web.userInfo(); st != http.StatusUnauthorized {
		t.Errorf("the deleted %s's cookie: %d, want 401", victim.name, st)
	}
	if auth := env.feverAuth(victim.feverKey); auth != 0 {
		t.Errorf("the deleted %s's Fever key: auth=%v, want 0", victim.name, auth)
	}
	if _, st := env.clientLogin(victim.name, victim.password); st != http.StatusUnauthorized {
		t.Errorf("signing in as the deleted %s: %d, want 401", victim.name, st)
	}

	// What they own is kept, but their sessions go at once and their name
	// stays theirs.
	if n := env.scalar(t, `SELECT count(*) FROM Session WHERE userid = $1`, victim.user.UserId); n != 0 {
		t.Errorf("%d of %s's sessions survive the deletion", n, victim.name)
	}
	if n := env.scalar(t, `SELECT count(*) FROM Article WHERE userid = $1`, victim.user.UserId); before != nil &&
		n != before.ArticleCount {
		t.Errorf("%s holds %d articles after the deletion, had %d", victim.name, n, before.ArticleCount)
	}
	if s := listUsers(t, env)[victim.name]; s == nil || s.DeletedUnixSec == 0 {
		t.Errorf("ListUsers does not show %s as deleted: %v", victim.name, s)
	}
	for _, name := range []string{victim.name, strings.ToUpper(victim.name)} {
		if _, err := addUser(env, name, "pw"); status.Code(err) != codes.AlreadyExists {
			t.Errorf("adding %s while %s awaits purging: %v, want AlreadyExists", name, victim.name, err)
		}
	}
	if _, err := deleteUser(env, victim.name); status.Code(err) != codes.NotFound {
		t.Errorf("deleting %s twice: %v, want NotFound", victim.name, err)
	}
	if _, err := deleteUser(env, ""); status.Code(err) != codes.InvalidArgument {
		t.Errorf("deleting nobody: %v, want InvalidArgument", err)
	}

	// A feed they share updates: the others get the new items, and their own
	// copy is no longer fetched.
	type subscriber struct {
		u    *tester
		feed int64
	}
	var sub poolSub
	var others []subscriber
	for _, s := range victim.pool {
		others = nil
		for _, u := range users {
			if u == victim || u.deleted {
				continue
			}
			for _, o := range u.pool {
				if o.index == s.index {
					others = append(others, subscriber{u, o.feedID})
				}
			}
		}
		if len(others) > 0 {
			sub = s
			break
		}
	}
	if len(others) == 0 {
		t.Fatalf("%s shares no feed with anyone", victim.name)
	}
	victimHad := len(env.feedArticleIDs(t, victim.user.UserId, sub.feedID))
	otherHad := len(env.feedArticleIDs(t, others[0].u.user.UserId, others[0].feed))
	_ = env.feeds.Publish(poolPath(sub.index), 2)
	for _, o := range others {
		env.sched.Schedule(o.u.user, models.FeedId(o.feed))
	}
	env.waitFor(t, "update reaches the subscribers still here", func() (bool, string) {
		if n := len(env.feedArticleIDs(t, others[0].u.user.UserId, others[0].feed)); n != otherHad+2 {
			return false, fmt.Sprintf("%s has %d, want %d", others[0].u.name, n, otherHad+2)
		}
		return true, ""
	})
	// Long enough for a reconcile, which must not bring their feeds back.
	time.Sleep(2 * time.Second)
	if n := len(env.feedArticleIDs(t, victim.user.UserId, sub.feedID)); n != victimHad {
		t.Errorf("the deleted %s's copy went from %d to %d articles", victim.name, victimHad, n)
	}

	// Restored, they are back as they were, still signed out, and their feeds
	// catch up.
	if err := env.timed("admin RestoreUser", func() error {
		_, err := env.admin.RestoreUser(context.Background(), &admin.RestoreUserRequest{Username: victim.name})
		return err
	}); err != nil {
		t.Fatalf("RestoreUser(%s): %v", victim.name, err)
	}
	victim.deleted = false
	if _, _, st := victim.api.userInfo(); st != http.StatusUnauthorized {
		t.Errorf("a session revoked by deleting %s works after restoring them: %d", victim.name, st)
	}
	token, st := env.clientLogin(victim.name, victim.password)
	if st != http.StatusOK {
		t.Fatalf("signing in as the restored %s: %d", victim.name, st)
	}
	victim.api = env.newClient(token)
	if victim.web, st = env.webLogin(victim.name, victim.password); st != http.StatusOK {
		t.Errorf("web login as the restored %s: %d", victim.name, st)
	}
	env.waitFor(t, "restored user's feeds catch up", func() (bool, string) {
		if n := len(env.feedArticleIDs(t, victim.user.UserId, sub.feedID)); n != victimHad+2 {
			return false, fmt.Sprintf("%s has %d, want %d", victim.name, n, victimHad+2)
		}
		return true, ""
	})
	if s := listUsers(t, env)[victim.name]; s == nil || s.DeletedUnixSec != 0 ||
		(before != nil && s.FeedCount != before.FeedCount) {
		t.Errorf("ListUsers shows the restored %s as %v; before deletion, %v", victim.name, s, before)
	}
	if err := env.timed("admin RestoreUser", func() error {
		_, err := env.admin.RestoreUser(context.Background(), &admin.RestoreUserRequest{Username: victim.name})
		return err
	}); status.Code(err) != codes.NotFound {
		t.Errorf("restoring %s, who is not deleted: %v, want NotFound", victim.name, err)
	}

	// Deleted again and purged, nothing is left naming them.
	if _, err := deleteUser(env, victim.name); err != nil {
		t.Fatalf("deleting %s again: %v", victim.name, err)
	}
	victim.deleted = true
	purge(t, env)
	check := func(when string) {
		for _, table := range []string{"UserTable", "Session", "UserPrefs", "Folder", "FolderChildren", "Feed",
			"Article", "RetrievalCache", "UserUnmuteFeeds", "UserFeedMuteRegexes"} {
			column := "userid"
			if table == "UserTable" {
				column = "id"
			}
			if n := env.scalar(t, fmt.Sprintf(`SELECT count(*) FROM %s WHERE %s = $1`, table, column),
				victim.user.UserId); n != 0 {
				t.Errorf("%s: %d rows in %s still name %s", when, n, table, victim.name)
			}
		}
	}
	check("after the purge")
	if _, ok := listUsers(t, env)[victim.name]; ok {
		t.Errorf("ListUsers still lists %s", victim.name)
	}

	// The name can be taken again, by a new account that inherits nothing.
	added, err := addUser(env, victim.name, victim.password)
	if err != nil {
		t.Fatalf("adding %s again: %v", victim.name, err)
	}
	if added.UserId == string(victim.user.UserId) {
		t.Errorf("%s came back with the old ID", victim.name)
	}
	if _, _, st := victim.api.userInfo(); st != http.StatusUnauthorized {
		t.Errorf("the deleted %s's token works for the new account: %d", victim.name, st)
	}
	// Same name and password make the same Fever key, which now opens the new,
	// empty account.
	resp2, _ := env.fever("fever feeds", victim.feverKey, "&feeds", nil)
	if auth, _ := resp2["auth"].(float64); auth != 1 {
		t.Errorf("Fever for the new %s: auth=%v", victim.name, resp2["auth"])
	} else if f, _ := resp2["feeds"].([]any); len(f) != 0 {
		t.Errorf("the new %s has %d feeds, want none", victim.name, len(f))
	}
	if _, err := deleteUser(env, victim.name); err != nil {
		t.Errorf("deleting the new %s: %v", victim.name, err)
	}
	purge(t, env)
}

// purge runs the garbage collector's purge of deleted users as though their
// retention window had passed.
func purge(t *testing.T, env *e2eEnv) {
	t.Helper()
	start := time.Now()
	users, articles, err := env.d.PurgeDeletedUsers(time.Now().Add(time.Hour))
	env.tm.record("purge deleted users", time.Since(start), err == nil)
	if err != nil {
		t.Errorf("purging deleted users: %v", err)
		return
	}
	t.Logf("purged %d users and %d articles in %s", users, articles, time.Since(start).Round(time.Millisecond))
}

// checkNoDuplicates checks that no pool feed holds an item twice for anyone,
// which concurrent fetches of one feed would cause.
func checkNoDuplicates(t *testing.T, env *e2eEnv) {
	n := env.scalar(t, `SELECT count(*) FROM (
		SELECT a.userid, a.feed, a.link FROM Article a JOIN Feed f ON f.id = a.feed
		WHERE f.url LIKE $1 GROUP BY a.userid, a.feed, a.link HAVING count(*) > 1)`,
		env.feeds.URL("/pool/%"))
	if n != 0 {
		t.Errorf("%d pool items are stored more than once for the same user and feed", n)
	}
}

func datasetSummary(t *testing.T, env *e2eEnv) map[string]int64 {
	return map[string]int64{
		"users":    env.scalar(t, `SELECT count(*) FROM UserTable`),
		"feeds":    env.scalar(t, `SELECT count(*) FROM Feed WHERE deleted IS NULL`),
		"folders":  env.scalar(t, `SELECT count(*) FROM Folder`),
		"articles": env.scalar(t, `SELECT count(*) FROM Article`),
		"unread":   env.scalar(t, `SELECT count(*) FROM Article WHERE NOT read`),
		"sessions": env.scalar(t, `SELECT count(*) FROM Session`),
	}
}

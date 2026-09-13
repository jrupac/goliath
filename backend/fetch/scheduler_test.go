package fetch

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

var testUser = models.User{UserId: "test-user", Username: "test"}

func key(feedID int64) storage.UserFeedKey {
	return storage.UserFeedKey{UserID: testUser.UserId, FeedID: feedID}
}

func feedsOf(ids ...int64) map[storage.UserFeedKey]models.User {
	feeds := map[storage.UserFeedKey]models.User{}
	for _, id := range ids {
		feeds[key(id)] = testUser
	}
	return feeds
}

// clock is a time the test moves by hand.
type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }
func (c *clock) snapshot(ids ...int64) snapshot {
	return snapshot{started: c.t, feeds: feedsOf(ids...)}
}

// stateScheduler returns a scheduler for driving its state machine directly,
// one method at a time, from the test's own goroutine.
func stateScheduler(c *clock) *Scheduler {
	return &Scheduler{
		metadataInterval: time.Hour,
		now:              c.now,
		wake:             make(chan struct{}, 1),
		jobs:             map[storage.UserFeedKey]*job{},
		unscheduled:      map[storage.UserFeedKey]time.Time{},
	}
}

// runningScheduler starts a scheduler with the given workers, feeds and work,
// and stops it when the test ends.
func runningScheduler(t *testing.T, workers int, feeds map[storage.UserFeedKey]models.User, work func(task) outcome) *Scheduler {
	t.Helper()
	s := &Scheduler{
		workers:           workers,
		metadataInterval:  time.Hour,
		reconcileInterval: time.Hour,
		work:              work,
		liveFeeds:         func() (map[storage.UserFeedKey]models.User, error) { return feeds, nil },
		now:               time.Now,
		wake:              make(chan struct{}, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return s
}

func queued(s *Scheduler) []int64 {
	var ids []int64
	for _, j := range s.queue {
		ids = append(ids, j.key.FeedID)
	}
	return ids
}

func waitFor(t *testing.T, what string, ch <-chan int64) int64 {
	t.Helper()
	select {
	case id := <-ch:
		return id
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
		return 0
	}
}

// Every feed is fetched, and never more at once than there are workers.
func TestSchedulerFetchesEveryFeedWithBoundedConcurrency(t *testing.T) {
	const feeds, workers = 20, 3
	var inFlight, most atomic.Int32
	fetched := make(chan int64, feeds)

	ids := make([]int64, feeds)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	runningScheduler(t, workers, feedsOf(ids...), func(tk task) outcome {
		n := inFlight.Add(1)
		for {
			m := most.Load()
			if n <= m || most.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		inFlight.Add(-1)
		fetched <- tk.key.FeedID
		return outcome{key: tk.key, next: time.Now().Add(time.Hour)}
	})

	seen := map[int64]bool{}
	for range feeds {
		id := waitFor(t, "every feed to be fetched", fetched)
		if seen[id] {
			t.Errorf("feed %d fetched twice", id)
		}
		seen[id] = true
	}
	if m := most.Load(); m > workers {
		t.Errorf("%d fetches ran at once, want at most %d", m, workers)
	}
}

// A feed is fetched again when the worker said it would next be due.
func TestSchedulerRequeuesAtTheReportedTime(t *testing.T) {
	fetched := make(chan int64, 10)
	runningScheduler(t, 1, feedsOf(1), func(tk task) outcome {
		fetched <- tk.key.FeedID
		return outcome{key: tk.key, next: time.Now().Add(20 * time.Millisecond)}
	})

	for range 3 {
		waitFor(t, "the feed to be fetched again", fetched)
	}
}

// A feed added while fetching runs is fetched at once, not at the next
// reconcile.
func TestScheduleFetchesANewFeedAtOnce(t *testing.T) {
	fetched := make(chan int64, 1)
	s := runningScheduler(t, 1, feedsOf(), func(tk task) outcome {
		fetched <- tk.key.FeedID
		return outcome{key: tk.key, next: time.Now().Add(time.Hour)}
	})

	s.Schedule(testUser, 7)
	if id := waitFor(t, "the new feed to be fetched", fetched); id != 7 {
		t.Errorf("fetched feed %d, want 7", id)
	}
}

// Unsubscribing from a feed mid-fetch cancels the fetch without waiting for
// it, and the feed is not fetched again, however soon it said it was due.
func TestUnscheduleCancelsAFetchInFlightAndDoesNotRequeueIt(t *testing.T) {
	started := make(chan int64, 10)
	cancelled := make(chan int64, 1)
	s := runningScheduler(t, 1, feedsOf(1), func(tk task) outcome {
		started <- tk.key.FeedID
		select {
		case <-tk.ctx.Done():
			cancelled <- tk.key.FeedID
		case <-time.After(5 * time.Second):
		}
		return outcome{key: tk.key, next: time.Now()}
	})

	waitFor(t, "the fetch to start", started)
	s.Unschedule(testUser, 1)
	waitFor(t, "the fetch to be cancelled", cancelled)

	select {
	case <-started:
		t.Error("an unscheduled feed was fetched again")
	case <-time.After(200 * time.Millisecond):
	}
}

// Callers are serving requests, and must not hang because fetching has not
// started, or has stopped.
func TestScheduleAndUnscheduleDoNotBlockWithoutAScheduler(t *testing.T) {
	s := NewScheduler(&Fetcher{}, &storage.MockDB{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range int64(100) {
			s.Schedule(testUser, i)
			s.Unschedule(testUser, i)
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Schedule/Unschedule blocked with no scheduler running")
	}
}

func TestUnscheduleDropsAQueuedFeed(t *testing.T) {
	c := &clock{t: time.Now()}
	s := stateScheduler(c)
	s.reconcile(c.snapshot(1, 2), false)

	s.Unschedule(testUser, 1)
	s.applyCommands()

	if _, ok := s.jobs[key(1)]; ok {
		t.Error("unscheduled feed is still known")
	}
	if got := queued(s); len(got) != 1 || got[0] != 2 {
		t.Errorf("queue = %v, want [2]", got)
	}
}

// Scheduling a feed already queued brings it forward rather than adding it
// twice.
func TestScheduleOfAQueuedFeedMakesItDueNow(t *testing.T) {
	c := &clock{t: time.Now()}
	s := stateScheduler(c)
	s.reconcile(c.snapshot(1, 2), false)
	s.push(s.jobs[key(1)], c.t.Add(time.Hour))
	s.push(s.jobs[key(2)], c.t.Add(time.Minute))

	c.advance(time.Second)
	s.Schedule(testUser, 1)
	s.applyCommands()

	if got := queued(s); len(got) != 2 || got[0] != 1 {
		t.Errorf("queue = %v, want feed 1 at the head", got)
	}
	if due := s.jobs[key(1)].due; !due.Equal(c.t) {
		t.Errorf("feed 1 due at %s, want now (%s)", due, c.t)
	}
}

func TestReconcileAddsAndDropsFeeds(t *testing.T) {
	c := &clock{t: time.Now()}
	s := stateScheduler(c)
	s.reconcile(c.snapshot(1, 2), false)

	c.advance(time.Second)
	s.reconcile(c.snapshot(2, 3), false)

	for id, want := range map[int64]bool{1: false, 2: true, 3: true} {
		if _, ok := s.jobs[key(id)]; ok != want {
			t.Errorf("feed %d known = %t, want %t", id, ok, want)
		}
	}
}

// A snapshot read before a feed was added may not include it, and must not
// undo the add.
func TestReconcileKeepsAFeedScheduledAfterTheSnapshotStarted(t *testing.T) {
	c := &clock{t: time.Now()}
	s := stateScheduler(c)
	stale := c.snapshot()

	c.advance(time.Second)
	s.Schedule(testUser, 1)
	s.applyCommands()
	s.reconcile(stale, false)

	if _, ok := s.jobs[key(1)]; !ok {
		t.Error("a reconcile older than the add dropped the feed")
	}

	// A snapshot that began after the add, and still does not have it, is
	// news: the feed has since gone some other way.
	c.advance(time.Second)
	s.reconcile(c.snapshot(), false)
	if _, ok := s.jobs[key(1)]; ok {
		t.Error("a feed missing from a newer snapshot was kept")
	}
}

// Likewise a snapshot read before an unsubscribe may still list the feed, and
// must not bring it back.
func TestReconcileDoesNotRestoreAFeedUnscheduledAfterTheSnapshotStarted(t *testing.T) {
	c := &clock{t: time.Now()}
	s := stateScheduler(c)
	s.reconcile(c.snapshot(1), false)
	stale := c.snapshot(1)

	c.advance(time.Second)
	s.Unschedule(testUser, 1)
	s.applyCommands()
	s.reconcile(stale, false)

	if _, ok := s.jobs[key(1)]; ok {
		t.Error("a reconcile older than the unsubscribe brought the feed back")
	}

	// A snapshot that began after the unsubscribe and has the feed is news
	// too: it was re-added some other way.
	c.advance(time.Second)
	s.reconcile(c.snapshot(1), false)
	if _, ok := s.jobs[key(1)]; !ok {
		t.Error("a feed live in a newer snapshot was not added")
	}
	if len(s.unscheduled) != 0 {
		t.Errorf("unsubscribes a newer snapshot covers are still remembered: %v", s.unscheduled)
	}
}

// dispatch hands the job at the head of the queue to a pretend worker.
func dispatch(s *Scheduler) task {
	tk := s.taskFor(context.Background(), s.queue[0])
	s.dispatched()
	return tk
}

// Unsubscribing and re-adding a feed while it is being fetched leaves it
// scheduled, whatever the fetch that was cancelled reports.
func TestScheduleWhileRunningFetchesAgainWhenDone(t *testing.T) {
	c := &clock{t: time.Now()}
	s := stateScheduler(c)
	s.reconcile(c.snapshot(1), false)
	tk := dispatch(s)

	s.Unschedule(testUser, 1)
	s.Schedule(testUser, 1)
	s.applyCommands()
	if tk.ctx.Err() == nil {
		t.Error("the fetch in flight was not cancelled")
	}

	s.finish(outcome{key: key(1), gone: true})
	if got := queued(s); len(got) != 1 || !s.jobs[key(1)].due.Equal(c.t) {
		t.Errorf("queue = %v, want the feed due now", got)
	}
}

func TestFinishDropsAFeedThatIsGone(t *testing.T) {
	c := &clock{t: time.Now()}
	s := stateScheduler(c)
	s.reconcile(c.snapshot(1), false)
	dispatch(s)

	s.finish(outcome{key: key(1), gone: true})
	if len(s.jobs) != 0 || len(s.queue) != 0 {
		t.Errorf("a feed reported gone is still scheduled: %v", queued(s))
	}
}

// Metadata is refreshed on a feed's first fetch and then once per interval,
// not on every fetch.
func TestMetadataIsRefreshedOncePerInterval(t *testing.T) {
	c := &clock{t: time.Now()}
	s := stateScheduler(c)
	s.reconcile(c.snapshot(1), false)

	if tk := dispatch(s); !tk.refreshMetadata {
		t.Error("the first fetch of a feed did not refresh its metadata")
	}
	s.finish(outcome{key: key(1), next: c.t, refreshedMetadata: true})

	c.advance(s.metadataInterval / 2)
	if tk := dispatch(s); tk.refreshMetadata {
		t.Error("metadata refreshed again before the interval passed")
	}
	s.finish(outcome{key: key(1), next: c.t})

	c.advance(s.metadataInterval / 2)
	if tk := dispatch(s); !tk.refreshMetadata {
		t.Error("metadata not refreshed once the interval passed")
	}
}

// Per-feed series are removed with the feed, and when a rename moves them to
// new labels, so that neither leaves a series behind that is never updated.
func TestFeedMetricsFollowTheFeed(t *testing.T) {
	c := &clock{t: time.Now()}
	s := stateScheduler(c)
	s.reconcile(c.snapshot(1), false)

	series := func() int { return testutil.CollectAndCount(feedFetchAttemptsMetric) }
	before := series()
	fetch := func(title string) []string {
		labels := []string{testUser.Username, "1", title, "http://example.com/feed"}
		feedFetchAttemptsMetric.WithLabelValues(labels...).Inc()
		return labels
	}

	dispatch(s)
	s.finish(outcome{key: key(1), next: c.t, labels: fetch("Old")})
	dispatch(s)
	s.finish(outcome{key: key(1), next: c.t, labels: fetch("New")})
	if got := series(); got != before+1 {
		t.Errorf("%d series after a rename, want %d", got, before+1)
	}

	s.Unschedule(testUser, 1)
	s.applyCommands()
	if got := series(); got != before {
		t.Errorf("%d series after unscheduling, want %d", got, before)
	}
}

// The workers have to stop with the scheduler, or a shutdown waits on them.
func TestRunStopsItsWorkers(t *testing.T) {
	var running sync.WaitGroup
	running.Add(1)
	started := make(chan int64, 1)
	s := &Scheduler{
		workers:           2,
		metadataInterval:  time.Hour,
		reconcileInterval: time.Hour,
		work: func(tk task) outcome {
			started <- tk.key.FeedID
			<-tk.ctx.Done()
			return outcome{key: tk.key}
		},
		liveFeeds: func() (map[storage.UserFeedKey]models.User, error) { return feedsOf(1), nil },
		now:       time.Now,
		wake:      make(chan struct{}, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer running.Done()
		s.Run(ctx)
	}()

	waitFor(t, "the fetch to start", started)
	cancel()

	done := make(chan struct{})
	go func() {
		running.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after its context was cancelled")
	}
}

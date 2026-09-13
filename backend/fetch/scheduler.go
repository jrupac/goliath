package fetch

import (
	"container/heap"
	"context"
	"flag"
	"math/rand/v2"
	"slices"
	"sync"
	"time"

	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	fetchWorkers = flag.Int("fetchWorkers", 32,
		"Number of feeds fetched at once.")
	fetchReconcileInterval = flag.Duration("fetchReconcileInterval", time.Minute,
		"Interval between comparisons of the fetch schedule against the subscriptions in the database, "+
			"which is how a change made other than through this server is picked up.")
	fetchStartSpread = flag.Duration("fetchStartSpread", 2*time.Minute,
		"Window over which the first fetch of every feed is spread when fetching starts.")
)

var (
	fetchQueuedMetric = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "feed_fetch_queued",
		Help: "Number of feeds waiting for their next fetch.",
	})
	fetchRunningMetric = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "feed_fetch_running",
		Help: "Number of feeds being fetched.",
	})
	fetchLagMetric = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "feed_fetch_schedule_lag_seconds",
		Help: "Delay between when a feed was due to be fetched and when a worker started on it. " +
			"Sustained lag means there are too few workers for the feeds being fetched.",
		Buckets: prometheus.ExponentialBuckets(0.1, 4, 8),
	})
)

func init() {
	prometheus.MustRegister(fetchQueuedMetric)
	prometheus.MustRegister(fetchRunningMetric)
	prometheus.MustRegister(fetchLagMetric)
}

// Subscriptions is how a change to what a user subscribes to reaches the
// fetcher, so that it takes effect now rather than at the next reconcile.
// Neither method blocks.
type Subscriptions interface {
	// Schedule makes a feed due now, adding it if it is not already fetched.
	Schedule(u models.User, feedID int64)
	// Unschedule stops fetching a feed. A fetch of it in flight is cancelled,
	// and it is not fetched again.
	Unschedule(u models.User, feedID int64)
}

// Scheduler fetches every feed every user subscribes to, each when it is due,
// with a fixed number of workers.
//
// One goroutine owns the schedule: a queue of feeds ordered by when each is
// next due. It hands the feed at the head to a worker once it is due and a
// worker is free, and requeues it when the worker reports when it is next due.
// Changes to the schedule reach that goroutine as commands, from Schedule and
// Unschedule, and from a periodic reconcile against the database.
type Scheduler struct {
	workers           int
	metadataInterval  time.Duration
	reconcileInterval time.Duration
	startSpread       time.Duration

	// work fetches one feed; liveFeeds lists every feed that should be
	// fetched. Both are fields so that the schedule can be tested without a
	// network or a database.
	work      func(task) outcome
	liveFeeds func() (map[storage.UserFeedKey]models.User, error)
	now       func() time.Time

	mu      sync.Mutex
	pending []command
	wake    chan struct{}

	// Owned by the goroutine running Run.
	jobs  map[storage.UserFeedKey]*job
	queue jobQueue
	// unscheduled records when each feed was last explicitly unscheduled,
	// until a reconcile that could not have seen it as live has run.
	unscheduled map[storage.UserFeedKey]time.Time
	running     int
}

var _ Subscriptions = (*Scheduler)(nil)

// NewScheduler returns a scheduler that fetches with f every feed d says is
// subscribed to.
func NewScheduler(f *Fetcher, d storage.Database) *Scheduler {
	return &Scheduler{
		workers:           *fetchWorkers,
		metadataInterval:  *feedMetadataRefreshInterval,
		reconcileInterval: *fetchReconcileInterval,
		startSpread:       *fetchStartSpread,
		work:              f.fetchFeed,
		liveFeeds:         func() (map[storage.UserFeedKey]models.User, error) { return liveFeeds(d) },
		now:               time.Now,
		wake:              make(chan struct{}, 1),
	}
}

// liveFeeds lists every feed any user is subscribed to, with its user.
func liveFeeds(d storage.Database) (map[storage.UserFeedKey]models.User, error) {
	users, err := d.GetAllUsers()
	if err != nil {
		return nil, err
	}
	keys, err := d.GetLiveFeedKeys()
	if err != nil {
		return nil, err
	}

	byId := make(map[models.UserId]models.User, len(users))
	for _, u := range users {
		byId[u.UserId] = u
	}
	feeds := make(map[storage.UserFeedKey]models.User, len(keys))
	for key := range keys {
		if u, ok := byId[key.UserID]; ok {
			feeds[key] = u
		}
	}
	return feeds, nil
}

// job is the schedule's record of one feed.
type job struct {
	key  storage.UserFeedKey
	user models.User
	due  time.Time
	// index is the job's position in the queue, or -1 while it is not queued.
	index    int
	failures int
	// metadataRefreshed is when the feed's own metadata was last refreshed.
	metadataRefreshed time.Time
	// labels are those its per-feed metrics were last written under.
	labels []string
	// ctx is the context its next or current fetch runs under, and cancel
	// cancels it. Made when the job first reaches the head of the queue.
	ctx    context.Context
	cancel context.CancelFunc
	// running is set while a worker has the job.
	running bool
	// removed marks a job unscheduled while it ran, to be dropped rather than
	// requeued when it finishes; rerun marks one scheduled while it ran, to be
	// due again as soon as it finishes.
	removed bool
	rerun   bool
	// scheduled is when the job was last explicitly scheduled.
	scheduled time.Time
}

// jobQueue is a min-heap of jobs by due time.
type jobQueue []*job

func (q jobQueue) Len() int           { return len(q) }
func (q jobQueue) Less(i, j int) bool { return q[i].due.Before(q[j].due) }
func (q jobQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].index = i
	q[j].index = j
}

func (q *jobQueue) Push(x any) {
	j := x.(*job)
	j.index = len(*q)
	*q = append(*q, j)
}

func (q *jobQueue) Pop() any {
	old := *q
	n := len(old)
	j := old[n-1]
	old[n-1] = nil
	j.index = -1
	*q = old[:n-1]
	return j
}

// command is a Schedule or Unschedule, waiting for the scheduler to apply it.
type command struct {
	key      storage.UserFeedKey
	user     models.User
	schedule bool
}

// snapshot is the list of live feeds as a reconcile read it.
type snapshot struct {
	// started is when the read began. A change the scheduler applied after
	// this may or may not be reflected in feeds; one applied before it is.
	started time.Time
	feeds   map[storage.UserFeedKey]models.User
	err     error
}

func (s *Scheduler) Schedule(u models.User, feedID int64) {
	s.enqueue(command{key: storage.UserFeedKey{UserID: u.UserId, FeedID: feedID}, user: u, schedule: true})
}

func (s *Scheduler) Unschedule(u models.User, feedID int64) {
	s.enqueue(command{key: storage.UserFeedKey{UserID: u.UserId, FeedID: feedID}, user: u})
}

// enqueue hands a command to the scheduler without waiting for it. Callers
// are serving requests, and must not hang because fetching has not started or
// has already stopped.
func (s *Scheduler) enqueue(c command) {
	s.mu.Lock()
	s.pending = append(s.pending, c)
	s.mu.Unlock()

	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// Run fetches feeds until the context is cancelled, then cancels every fetch
// in flight and returns once the workers have stopped.
func (s *Scheduler) Run(ctx context.Context) {
	log.Infof("Starting feed fetching with %d workers.", s.workers)

	s.jobs = map[storage.UserFeedKey]*job{}
	s.unscheduled = map[storage.UserFeedKey]time.Time{}

	started := s.now()
	feeds, err := s.liveFeeds()
	s.reconcile(snapshot{started: started, feeds: feeds, err: err}, true)

	work := make(chan task)
	results := make(chan outcome)
	var workers sync.WaitGroup
	for range s.workers {
		workers.Go(func() {
			for t := range work {
				o := s.work(t)
				select {
				case results <- o:
				case <-ctx.Done():
				}
			}
		})
	}

	reconcileTicker := time.NewTicker(s.reconcileInterval)
	defer reconcileTicker.Stop()
	snapshots := make(chan snapshot, 1)
	reconciling := false

	timer := time.NewTimer(time.Hour)
	defer timer.Stop()

	defer func() {
		close(work)
		for _, j := range s.jobs {
			if j.cancel != nil {
				j.cancel()
			}
		}
		workers.Wait()
		fetchQueuedMetric.Set(0)
		fetchRunningMetric.Set(0)
		log.Infof("Stopped feed fetching.")
	}()

	for {
		fetchQueuedMetric.Set(float64(len(s.queue)))
		fetchRunningMetric.Set(float64(s.running))

		// Offered to the workers only once the head is due. The channel is
		// unbuffered, so the send completes only when a worker is free, and
		// until then nothing leaves the queue.
		var dispatch chan<- task
		var next task
		var wait <-chan time.Time
		if len(s.queue) > 0 {
			head := s.queue[0]
			if d := head.due.Sub(s.now()); d <= 0 {
				dispatch = work
				next = s.taskFor(ctx, head)
			} else {
				timer.Reset(d)
				wait = timer.C
			}
		}

		select {
		case dispatch <- next:
			s.dispatched()
		case <-wait:
		case <-s.wake:
			s.applyCommands()
		case o := <-results:
			s.finish(o)
		case <-reconcileTicker.C:
			if !reconciling {
				reconciling = true
				go func() {
					started := s.now()
					feeds, err := s.liveFeeds()
					snapshots <- snapshot{started: started, feeds: feeds, err: err}
				}()
			}
		case snap := <-snapshots:
			reconciling = false
			s.reconcile(snap, false)
		case <-ctx.Done():
			return
		}
	}
}

// taskFor makes the task for a job's next fetch.
func (s *Scheduler) taskFor(ctx context.Context, j *job) task {
	if j.ctx == nil {
		j.ctx, j.cancel = context.WithCancel(ctx)
	}
	return task{
		ctx:             j.ctx,
		key:             j.key,
		user:            j.user,
		failures:        j.failures,
		refreshMetadata: s.now().Sub(j.metadataRefreshed) >= s.metadataInterval,
	}
}

// dispatched records that the job at the head of the queue went to a worker.
func (s *Scheduler) dispatched() {
	j := heap.Pop(&s.queue).(*job)
	j.running = true
	s.running++
	fetchLagMetric.Observe(s.now().Sub(j.due).Seconds())
}

// finish applies a worker's outcome to its job.
func (s *Scheduler) finish(o outcome) {
	j := s.jobs[o.key]
	j.running = false
	s.running--
	j.cancel()
	j.ctx, j.cancel = nil, nil

	if o.labels != nil {
		if j.labels != nil && !slices.Equal(j.labels, o.labels) {
			deleteFeedMetrics(j.labels)
		}
		j.labels = o.labels
	}
	j.failures = o.failures
	if o.refreshedMetadata {
		j.metadataRefreshed = s.now()
	}

	switch {
	case j.rerun:
		// Scheduled again while it ran, which is what re-adding a feed that
		// had just been unsubscribed from looks like. Whatever this fetch
		// found, the newer request is the one to honour.
		j.rerun = false
		s.push(j, s.now())
	case j.removed || o.gone:
		s.drop(j)
	default:
		s.push(j, o.next)
	}
}

// applyCommands applies every Schedule and Unschedule waiting.
func (s *Scheduler) applyCommands() {
	s.mu.Lock()
	pending := s.pending
	s.pending = nil
	s.mu.Unlock()

	for _, c := range pending {
		now := s.now()
		j, known := s.jobs[c.key]

		if !c.schedule {
			s.unscheduled[c.key] = now
			if !known {
				continue
			}
			log.Infof("Unscheduling feed %d for %s", c.key.FeedID, c.user)
			s.unschedule(j)
			continue
		}

		delete(s.unscheduled, c.key)
		log.Infof("Scheduling feed %d for %s", c.key.FeedID, c.user)
		switch {
		case !known:
			j = newJob(c.key, c.user)
			j.scheduled = now
			s.push(j, now)
		case j.running:
			j.scheduled = now
			j.removed = false
			j.rerun = true
		default:
			j.scheduled = now
			s.push(j, now)
		}
	}
}

// unschedule stops fetching a job: at once if it is queued, and without
// requeuing it if it is running.
func (s *Scheduler) unschedule(j *job) {
	if j.running {
		j.removed = true
		j.rerun = false
		j.cancel()
		return
	}
	s.drop(j)
}

// reconcile brings the schedule into line with a list of live feeds.
//
// Only where the list is newer than what the scheduler was told: a feed
// scheduled or unscheduled after the list was read keeps what it was told,
// since the list may predate that change.
func (s *Scheduler) reconcile(snap snapshot, initial bool) {
	if snap.err != nil {
		log.Warningf("Failed to list feeds to fetch: %s", snap.err)
		return
	}

	now := s.now()
	var added, dropped int
	for key, u := range snap.feeds {
		if _, known := s.jobs[key]; known {
			continue
		}
		if at, ok := s.unscheduled[key]; ok && at.After(snap.started) {
			continue
		}
		due := now
		if initial && s.startSpread > 0 {
			// Every feed is due at once when fetching starts. Spread, so that
			// hosts serving many feeds are not asked for all of them together.
			due = now.Add(rand.N(s.startSpread))
		}
		s.push(newJob(key, u), due)
		added++
	}

	for key, j := range s.jobs {
		if _, live := snap.feeds[key]; live || j.removed || j.scheduled.After(snap.started) {
			continue
		}
		s.unschedule(j)
		dropped++
	}

	for key, at := range s.unscheduled {
		if !at.After(snap.started) {
			delete(s.unscheduled, key)
		}
	}

	if initial {
		log.Infof("Scheduled %d feeds for fetching.", added)
	} else if added > 0 || dropped > 0 {
		log.Infof("Reconciled fetch schedule: %d feeds added, %d dropped.", added, dropped)
	}
}

func newJob(key storage.UserFeedKey, u models.User) *job {
	return &job{key: key, user: u, index: -1}
}

// push queues a job to be due at the given time, or moves it there if it is
// already queued.
func (s *Scheduler) push(j *job, due time.Time) {
	j.due = due
	s.jobs[j.key] = j
	if j.index >= 0 {
		heap.Fix(&s.queue, j.index)
	} else {
		heap.Push(&s.queue, j)
	}
}

// drop forgets a job that is not running.
func (s *Scheduler) drop(j *job) {
	if j.index >= 0 {
		heap.Remove(&s.queue, j.index)
	}
	if j.cancel != nil {
		j.cancel()
	}
	delete(s.jobs, j.key)
	deleteFeedMetrics(j.labels)
}

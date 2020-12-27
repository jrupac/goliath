package cache

import (
	"context"
	"flag"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/seiflotfy/cuckoofilter"
	"sync"
	"sync/atomic"
	"time"
)

var (
	retrievalCacheWriteInterval = flag.Duration("retrievalCacheWriteInterval", 1*time.Minute, "Period between retrieval cache writes.")
)

// RetrievalCache is a probabilistic cache to determine if an article has ever retrieved seen before.
type RetrievalCache struct {
	caches map[string]*cuckoo.ScalableCuckooFilter
	lock   sync.Mutex
	ready  atomic.Value
}

func StartRetrievalCache(ctx context.Context, d *storage.Database) *RetrievalCache {
	r := RetrievalCache{}
	r.loadCache(d)
	go r.startPeriodicWriter(ctx, d)

	return &r
}

// Add adds a new a entry into the retrieval cache for the specified user.
func (r *RetrievalCache) Add(u models.User, entry string) {
	if r.ready.Load() == nil {
		log.Errorf("could not persist entry: %s", entry)
		return
	}

	r.lock.Lock()
	defer r.lock.Unlock()
	log.Infof("Add entry for user %s and entry %s", u.UserId, entry)

	if cache, ok := r.caches[u.Key]; !ok {
		log.Errorf("unknown user: %s", u.Key)
	} else {
		cache.InsertUnique([]byte(entry))
	}
}

// Lookup returns whether the specified entry is present in the retrieval cache for the specified user.
func (r *RetrievalCache) Lookup(u models.User, entry string) bool {
	if r.ready.Load() == nil {
		log.Errorf("could not lookup entry: %s", entry)
		return false
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	if cache, ok := r.caches[u.Key]; !ok {
		log.Errorf("unknown user: %s", u.Key)
		return false
	} else {
		return cache.Lookup([]byte(entry))
	}
}

func (r *RetrievalCache) loadCache(d *storage.Database) {
	// TODO: Read from DB.

	// If the read above failed, initialize an empty retrieval cache for each user.
	users, err := d.GetAllUsers()
	if err != nil {
		log.Errorf("could not load retrieval cache")
		return
	}

	r.lock.Lock()
	defer r.lock.Unlock()
	for _, user := range users {
		r.caches[user.Key] = cuckoo.NewScalableCuckooFilter()
	}

	r.ready.Store(true)
}

func (r *RetrievalCache) startPeriodicWriter(ctx context.Context, d *storage.Database) {
	initial := make(chan struct{})
	tick := make(<-chan time.Time)

	go func() {
		tick = time.After(*retrievalCacheWriteInterval)
		initial <- struct{}{}
	}()

	for {
		select {
		case <-initial:
			// This is to allow for the first interval to complete.
			continue
		case <-tick:
			r.persistCache(d)
			tick = time.After(*retrievalCacheWriteInterval)
		case <-ctx.Done():
			r.persistCache(d)
			return
		}
	}
}

func (r *RetrievalCache) persistCache(d *storage.Database) {
	r.lock.Lock()
	defer r.lock.Unlock()
	log.Infof("Persisting retrieval cache.")
	// TODO: Write into DB.
}

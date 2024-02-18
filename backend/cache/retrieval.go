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
	retrievalCacheWriteInterval = flag.Duration("retrievalCacheWriteInterval", 5*time.Minute, "Period between retrieval cache writes.")
)

// RetrievalCache is a probabilistic cache to determine if an article has ever retrieved seen before.
type RetrievalCache struct {
	caches map[string]*cuckoo.ScalableCuckooFilter
	lock   sync.Mutex
	ready  atomic.Value
}

// StartRetrievalCache creates a new RetrievalCache and starts background processing.
func StartRetrievalCache(ctx context.Context, d *storage.Database) *RetrievalCache {
	r := RetrievalCache{}
	r.loadCache(d)
	go r.startPeriodicWriter(ctx, d)

	return &r
}

// Add adds a new a entry into the retrieval cache for the specified user.
func (r *RetrievalCache) Add(u models.User, entry string) {
	if r.ready.Load() == nil {
		log.Errorf("retrieval cache not ready: %s", entry)
		return
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	if cache, ok := r.caches[string(u.UserId)]; !ok {
		log.Errorf("unknown user: %s", u.UserId)
	} else {
		cache.InsertUnique([]byte(entry))
	}
}

// Lookup returns whether the specified entry is present in the retrieval cache for the specified user.
func (r *RetrievalCache) Lookup(u models.User, entry string) bool {
	if r.ready.Load() == nil {
		log.Errorf("retrieval cache not ready: %s", entry)
		return false
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	if cache, ok := r.caches[string(u.UserId)]; !ok {
		log.Errorf("unknown user: %s", u.UserId)
		return false
	} else {
		res := cache.Lookup([]byte(entry))
		return res
	}
}

func (r *RetrievalCache) loadCache(d *storage.Database) {
	r.caches = map[string]*cuckoo.ScalableCuckooFilter{}

	retrievalCaches, err := d.GetAllRetrievalCaches()

	// If there is an error or no entries, just initialize as empty.
	if err != nil || len(retrievalCaches) == 0 {
		log.Errorf("could not load retrieval cache")

		users, err := d.GetAllUsers()
		if err != nil {
			log.Errorf("could not fetch users")
			return
		}

		r.lock.Lock()
		defer r.lock.Unlock()
		for _, user := range users {
			r.caches[string(user.UserId)] = cuckoo.NewScalableCuckooFilter()
		}
	} else {
		r.lock.Lock()
		defer r.lock.Unlock()
		for id, encoded := range retrievalCaches {
			cache, err := cuckoo.DecodeScalableFilter([]byte(encoded))

			// If the cache is corrupt, just create an empty one.
			if err != nil {
				log.Errorf("could not load cache for user %s", id)
				cache = cuckoo.NewScalableCuckooFilter()
			}
			r.caches[id] = cache
		}
	}

	log.Infof("Completed loading retrieval cache.")
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

	entries := map[string][]byte{}
	for id, cache := range r.caches {
		entries[id] = cache.Encode()
	}

	err := d.PersistAllRetrievalCaches(entries)
	if err != nil {
		log.Errorf("failed to persist retrieval cache: %s", err)
	}
}

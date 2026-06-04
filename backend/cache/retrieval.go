package cache

import (
	"context"
	"encoding/base64"
	"flag"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	cuckoo "github.com/seiflotfy/cuckoofilter"
)

var (
	retrievalCacheWriteInterval = flag.Duration("retrievalCacheWriteInterval", 5*time.Minute, "Period between retrieval cache writes.")
)

// CuckooFilterRetrievalCache is a probabilistic cache storing the hashes of all past retrieved articles.
type CuckooFilterRetrievalCache struct {
	caches map[storage.UserFeedKey]*cuckoo.ScalableCuckooFilter
	lock   sync.Mutex
	ready  atomic.Value
}

// StartRetrievalCache creates a new CuckooFilterRetrievalCache and starts background processing.
func StartRetrievalCache(ctx context.Context, d storage.Database) (RetrievalCache, error) {
	r := CuckooFilterRetrievalCache{}
	err := r.loadCache(d)
	if err != nil {
		return nil, err
	}

	go r.startPeriodicWriter(ctx, d)
	return &r, nil
}

// Add adds a new entry into the retrieval cache for the specified user and feed.
func (r *CuckooFilterRetrievalCache) Add(u models.User, feedId int64, entry string) {
	if r.ready.Load() == nil {
		log.Errorf("retrieval cache not ready: %s", entry)
		return
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	key := storage.UserFeedKey{UserID: u.UserId, FeedID: feedId}
	cache, ok := r.caches[key]
	if !ok {
		cache = cuckoo.NewScalableCuckooFilter()
		r.caches[key] = cache
	}
	cache.InsertUnique([]byte(entry))
}

// Lookup returns whether the specified entry is present in the retrieval cache for the specified user and feed.
func (r *CuckooFilterRetrievalCache) Lookup(u models.User, feedId int64, entry string) bool {
	if r.ready.Load() == nil {
		log.Errorf("retrieval cache not ready: %s", entry)
		return false
	}

	r.lock.Lock()
	defer r.lock.Unlock()

	key := storage.UserFeedKey{UserID: u.UserId, FeedID: feedId}
	if cache, ok := r.caches[key]; !ok {
		return false
	} else {
		res := cache.Lookup([]byte(entry))
		return res
	}
}

func (r *CuckooFilterRetrievalCache) loadCache(d storage.Database) error {
	r.caches = map[storage.UserFeedKey]*cuckoo.ScalableCuckooFilter{}

	retrievalCaches, err := d.GetAllRetrievalCaches()

	// If there is an error or no entries, just initialize as empty.
	if err != nil || len(retrievalCaches) == 0 {
		if err != nil {
			log.Errorf("while loading retrieval cache: %s", err)
		}
		// In the per-feed sharded cache design, we dynamically create
		// cuckoo filters as feeds are fetched, so we don't need to populate
		// anything upfront here.
	} else {
		r.lock.Lock()
		defer r.lock.Unlock()
		for key, encoded := range retrievalCaches {
			var cache *cuckoo.ScalableCuckooFilter

			// If the cache is corrupt, just create an empty one.
			cacheBytes, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				log.Errorf("while decoding retrieval cache from database: %s", err)
				cache = cuckoo.NewScalableCuckooFilter()
			} else {
				cache, err = cuckoo.DecodeScalableFilter(cacheBytes)
				if err != nil {
					log.Errorf("while recreating retrieval cache: %s", err)
					cache = cuckoo.NewScalableCuckooFilter()
				}
			}

			r.caches[key] = cache
		}
	}

	log.Infof("Completed loading retrieval cache.")
	r.ready.Store(true)
	return nil
}

func (r *CuckooFilterRetrievalCache) startPeriodicWriter(ctx context.Context, d storage.Database) {
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

func (r *CuckooFilterRetrievalCache) persistCache(d storage.Database) {
	r.lock.Lock()
	defer r.lock.Unlock()
	log.Infof("Persisting retrieval cache.")

	activeKeys, err := d.GetActiveFeedKeys()
	if err != nil {
		log.Errorf("failed to prune cache: could not get active feed keys: %s", err)
		return
	}

	for key := range r.caches {
		if !activeKeys[key] {
			delete(r.caches, key)
		}
	}

	entries := map[storage.UserFeedKey][]byte{}
	for key, cache := range r.caches {
		entries[key] = cache.Encode()
	}

	err = d.PersistAllRetrievalCaches(entries)
	if err != nil {
		log.Errorf("failed to persist retrieval cache: %s", err)
	}
}

package cache

import (
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

	d       storage.Database
	stop    chan struct{}
	stopped chan struct{}
}

// StartRetrievalCache loads the retrieval cache and starts writing it to the
// database periodically. Close stops the writes and makes a last one.
func StartRetrievalCache(d storage.Database) (*CuckooFilterRetrievalCache, error) {
	r := &CuckooFilterRetrievalCache{d: d, stop: make(chan struct{}), stopped: make(chan struct{})}
	if err := r.loadCache(d); err != nil {
		return nil, err
	}

	go r.writePeriodically()
	return r, nil
}

// Close stops the periodic writes and writes the cache a last time, so that
// what was added since the last write survives a restart.
//
// It is the caller's to sequence, rather than following the context the rest
// of the process stops on: the last write has to come after whatever adds to
// the cache has stopped, or it misses their additions, and before the
// database closes, or it fails.
func (r *CuckooFilterRetrievalCache) Close() {
	close(r.stop)
	<-r.stopped
	r.persistCache()
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

func (r *CuckooFilterRetrievalCache) writePeriodically() {
	defer close(r.stopped)
	tick := time.After(*retrievalCacheWriteInterval)

	for {
		select {
		case <-tick:
			r.persistCache()
			tick = time.After(*retrievalCacheWriteInterval)
		case <-r.stop:
			return
		}
	}
}

func (r *CuckooFilterRetrievalCache) persistCache() {
	r.lock.Lock()
	defer r.lock.Unlock()
	log.Infof("Persisting retrieval cache.")

	activeKeys, err := r.d.GetActiveFeedKeys()
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

	if err = r.d.PersistAllRetrievalCaches(entries); err != nil {
		log.Errorf("failed to persist retrieval cache: %s", err)
		return
	}
	log.Infof("Persisted retrieval cache for %d feeds.", len(entries))
}

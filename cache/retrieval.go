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

type RetrievalCache struct {
	cache *cuckoo.ScalableCuckooFilter
	lock  sync.Mutex
	ready atomic.Value
}

func StartRetrievalCache(ctx context.Context, d *storage.Database) *RetrievalCache {
	r := RetrievalCache{}
	r.loadCache(d)
	go r.startPeriodicWriter(ctx, d)

	return &r
}

func (r *RetrievalCache) Add(u models.User, entry string) {
	if r.ready.Load() == nil {
		log.Errorf("Could not persist entry: %s", entry)
		return
	}

	r.lock.Lock()
	defer r.lock.Unlock()
	log.Infof("Add entry for user %s and entry %s", u.UserId, entry)
	r.cache.InsertUnique([]byte(entry))
}

func (r *RetrievalCache) loadCache(d *storage.Database) {
	// TODO: Read from DB.

	r.lock.Lock()
	defer r.lock.Unlock()
	r.cache = cuckoo.NewScalableCuckooFilter()
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

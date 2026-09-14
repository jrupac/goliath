package cache

import (
	"encoding/base64"
	"testing"

	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	cuckoo "github.com/seiflotfy/cuckoofilter"
)

func TestAddAndLookup(t *testing.T) {
	db := &storage.MockDB{}
	user := models.User{UserId: "test-user"}
	db.OnGetAllRetrievalCaches = func() (map[storage.UserFeedKey]string, error) {
		return map[storage.UserFeedKey]string{}, nil
	}

	cache, err := StartRetrievalCache(db)
	if err != nil {
		t.Fatalf("Failed to start retrieval cache: %v", err)
	}

	entry1 := "hello"
	entry2 := "world"

	cache.Add(user, 123, entry1)

	if !cache.Lookup(user, 123, entry1) {
		t.Error("expected to find entry1 in cache")
	}

	if cache.Lookup(user, 123, entry2) {
		t.Error("did not expect to find entry2 in cache")
	}

	if cache.Lookup(user, 456, entry1) {
		t.Error("did not expect to find entry1 under a different feed ID in cache")
	}
}

// The last write is made on Close, which the caller makes once whatever adds
// to the cache has stopped, so an entry added just before it is written.
func TestCloseWritesTheCacheALastTime(t *testing.T) {
	user := models.User{UserId: "test-user"}
	key := storage.UserFeedKey{UserID: user.UserId, FeedID: 123}
	var written map[storage.UserFeedKey][]byte
	db := &storage.MockDB{
		OnGetActiveFeedKeys: func() (map[storage.UserFeedKey]bool, error) {
			return map[storage.UserFeedKey]bool{key: true}, nil
		},
		OnPersistAllRetrievalCaches: func(entries map[storage.UserFeedKey][]byte) error {
			written = entries
			return nil
		},
	}

	cache, err := StartRetrievalCache(db)
	if err != nil {
		t.Fatalf("Failed to start retrieval cache: %v", err)
	}
	cache.Add(user, 123, "added-before-close")
	cache.Close()

	encoded, ok := written[key]
	if !ok {
		t.Fatalf("Close wrote %v, want the added feed's filter", written)
	}
	cf, err := cuckoo.DecodeScalableFilter(encoded)
	if err != nil {
		t.Fatalf("decoding the written filter: %v", err)
	}
	if !cf.Lookup([]byte("added-before-close")) {
		t.Error("the last write does not hold the entry added before Close")
	}
}

func TestLoadCache(t *testing.T) {
	user := models.User{UserId: "test-user"}

	t.Run("no existing caches", func(t *testing.T) {
		db := &storage.MockDB{}
		db.OnGetAllRetrievalCaches = func() (map[storage.UserFeedKey]string, error) {
			return map[storage.UserFeedKey]string{}, nil
		}

		cache, err := StartRetrievalCache(db)
		if err != nil {
			t.Fatalf("Failed to start retrieval cache: %v", err)
		}

		if cache.Lookup(user, 123, "any") {
			t.Error("expected new cache to be empty")
		}
	})

	t.Run("with existing caches", func(t *testing.T) {
		// Create a filter, add an item, and encode it.
		cf := cuckoo.NewScalableCuckooFilter()
		cf.InsertUnique([]byte("existing-entry"))
		encoded := base64.StdEncoding.EncodeToString(cf.Encode())

		db := &storage.MockDB{}
		key := storage.UserFeedKey{UserID: user.UserId, FeedID: 123}
		db.OnGetAllRetrievalCaches = func() (map[storage.UserFeedKey]string, error) {
			return map[storage.UserFeedKey]string{key: encoded}, nil
		}

		cache, err := StartRetrievalCache(db)
		if err != nil {
			t.Fatalf("Failed to start retrieval cache: %v", err)
		}

		if !cache.Lookup(user, 123, "existing-entry") {
			t.Error("expected to find existing entry in loaded cache")
		}
	})
}

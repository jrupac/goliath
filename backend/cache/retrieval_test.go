package cache

import (
	"context"
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

	cache, err := StartRetrievalCache(context.Background(), db)
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

func TestLoadCache(t *testing.T) {
	user := models.User{UserId: "test-user"}

	t.Run("no existing caches", func(t *testing.T) {
		db := &storage.MockDB{}
		db.OnGetAllRetrievalCaches = func() (map[storage.UserFeedKey]string, error) {
			return map[storage.UserFeedKey]string{}, nil
		}

		cache, err := StartRetrievalCache(context.Background(), db)
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

		cache, err := StartRetrievalCache(context.Background(), db)
		if err != nil {
			t.Fatalf("Failed to start retrieval cache: %v", err)
		}

		if !cache.Lookup(user, 123, "existing-entry") {
			t.Error("expected to find existing entry in loaded cache")
		}
	})
}

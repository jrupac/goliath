package cache

import (
	"fmt"
	"sync"

	"github.com/jrupac/goliath/models"
)

// MockRetrievalCache is a mock implementation of the RetrievalCache interface for testing.
type MockRetrievalCache struct {
	cache map[string]bool
	lock  sync.Mutex
}

// NewMockRetrievalCache creates a new mock retrieval cache.
func NewMockRetrievalCache() *MockRetrievalCache {
	return &MockRetrievalCache{
		cache: make(map[string]bool),
	}
}

// Add adds a new entry into the mock cache.
func (m *MockRetrievalCache) Add(u models.User, feedId int64, entry string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	key := fmt.Sprintf("%s:%d:%s", u.UserId, feedId, entry)
	m.cache[key] = true
}

// Lookup returns whether the specified entry is present in the mock cache.
func (m *MockRetrievalCache) Lookup(u models.User, feedId int64, entry string) bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	key := fmt.Sprintf("%s:%d:%s", u.UserId, feedId, entry)
	return m.cache[key]
}

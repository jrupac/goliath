package cache

import (
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
func (m *MockRetrievalCache) Add(u models.User, entry string) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.cache[string(u.UserId)+":"+entry] = true
}

// Lookup returns whether the specified entry is present in the mock cache.
func (m *MockRetrievalCache) Lookup(u models.User, entry string) bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.cache[string(u.UserId)+":"+entry]
}

package cache

import "github.com/jrupac/goliath/models"

// RetrievalCache defines an interface for the retrieval cache, allowing for mocking.
type RetrievalCache interface {
	Add(u models.User, entry string)
	Lookup(u models.User, entry string) bool
}

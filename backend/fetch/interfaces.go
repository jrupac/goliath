package fetch

import "github.com/mat/besticon/v3/besticon"

// IconFinder defines an interface for finding icons, allowing for mocking in tests.
type IconFinder interface {
	FetchIcons(url string) ([]besticon.Icon, error)
}

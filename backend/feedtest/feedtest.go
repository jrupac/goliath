// Package feedtest serves synthetic RSS feeds from a local address, standing
// in for the publisher of every feed a test subscribes to, so that nothing a
// test does reaches a real site.
//
// Feeds are paths. Items are added to them on demand, and every request for
// each is counted, which is what a test of how often the server fetches a
// feed needs. The same operations are reachable over HTTP, for a server left
// running while a person or a browser drives the client:
//
//	POST /_publish?path=<path>&n=<count>   add items to a feed
//	GET  /_feeds                           every feed, its items and requests
package feedtest

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// controlPrefix begins every path that is not a feed.
const controlPrefix = "/_"

// Server serves feeds until closed.
type Server struct {
	srv  *http.Server
	lis  net.Listener
	base string

	mu    sync.Mutex
	feeds map[string]*feed
}

type feed struct {
	title    string
	items    []rssItem
	seq      int
	last     time.Time
	requests int
}

// Start serves feeds on addr, which may name port 0 to take any free one.
func Start(addr string) (*Server, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	s := &Server{lis: lis, base: "http://" + lis.Addr().String(), feeds: map[string]*feed{}}
	s.srv = &http.Server{Handler: http.HandlerFunc(s.serve), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = s.srv.Serve(lis) }()
	return s, nil
}

// Close stops serving.
func (s *Server) Close() error { return s.srv.Close() }

// Addr is the host:port being served, as a fetcher's address allowlist names
// it.
func (s *Server) Addr() string { return s.lis.Addr().String() }

// URL returns the address of the feed at path.
func (s *Server) URL(path string) string { return s.base + path }

// Add creates a feed at path holding n items, unless there is one there
// already, and returns its URL.
func (s *Server) Add(path, title string, n int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.feeds[path]; !ok {
		// Dated back from now, a second apart, so that the items are in the
		// past and the next one published is newer than all of them.
		f := &feed{title: title, last: time.Now().Truncate(time.Second).Add(-time.Duration(n+1) * time.Second)}
		s.feeds[path] = f
		s.append(path, f, n)
	}
	return s.URL(path)
}

// Publish adds n items to the feed at path.
func (s *Server) Publish(path string, n int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.feeds[path]
	if !ok {
		return fmt.Errorf("no feed at %s", path)
	}
	s.append(path, f, n)
	return nil
}

// Requests counts the requests made for the feed at path.
func (s *Server) Requests(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.feeds[path]; ok {
		return f.requests
	}
	return 0
}

// append adds items dated strictly after the feed's newest, and no earlier
// than now. A fetcher discards an item no newer than the newest it already
// has, and feed dates have whole-second precision.
func (s *Server) append(path string, f *feed, n int) {
	for range n {
		f.seq++
		f.last = f.last.Add(time.Second)
		if now := time.Now().Truncate(time.Second); now.After(f.last) {
			f.last = now
		}
		link := fmt.Sprintf("%s%s/item/%d", s.base, path, f.seq)
		f.items = append(f.items, rssItem{
			Title:       fmt.Sprintf("Item %d of %s", f.seq, path),
			Link:        link,
			GUID:        link,
			PubDate:     f.last.Format(time.RFC1123Z),
			Description: fmt.Sprintf("<p>Body of item %d.</p>", f.seq),
		})
	}
}

type rssDoc struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, controlPrefix) {
		s.control(w, r)
		return
	}

	s.mu.Lock()
	f, ok := s.feeds[r.URL.Path]
	var doc rssDoc
	if ok {
		f.requests++
		items := make([]rssItem, len(f.items))
		for i, it := range f.items {
			items[len(f.items)-1-i] = it
		}
		doc = rssDoc{Version: "2.0", Channel: rssChannel{
			Title: f.title,
			// The site a feed belongs to is here too, and is not a feed, so
			// that anything looking there for an icon finds nothing rather
			// than reaching elsewhere.
			Link:        s.URL(r.URL.Path + "/site"),
			Description: "Served by feedtest.",
			Items:       items,
		}}
	}
	s.mu.Unlock()

	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml")
	_, _ = io.WriteString(w, xml.Header)
	_ = xml.NewEncoder(w).Encode(doc)
}

type feedInfo struct {
	Path     string `json:"path"`
	URL      string `json:"url"`
	Title    string `json:"title"`
	Items    int    `json:"items"`
	Requests int    `json:"requests"`
}

func (s *Server) control(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/_publish":
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", http.StatusMethodNotAllowed)
			return
		}
		n, err := strconv.Atoi(r.URL.Query().Get("n"))
		if err != nil || n <= 0 {
			n = 1
		}
		if err = s.Publish(r.URL.Query().Get("path"), n); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		_, _ = fmt.Fprintf(w, "published %d\n", n)
	case "/_feeds":
		s.mu.Lock()
		var feeds []feedInfo
		for path, f := range s.feeds {
			feeds = append(feeds, feedInfo{Path: path, URL: s.URL(path), Title: f.title,
				Items: len(f.items), Requests: f.requests})
		}
		s.mu.Unlock()
		sort.Slice(feeds, func(a, b int) bool { return feeds[a].Path < feeds[b].Path })
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(feeds)
	default:
		http.NotFound(w, r)
	}
}

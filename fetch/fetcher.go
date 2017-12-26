package fetch

import (
	"context"
	"errors"
	"flag"
	"github.com/SlyMarbo/rss"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/goliath/utils"
	"github.com/mat/besticon/besticon"
	"io/ioutil"
	"net/url"
	"sync"
	"time"
)

var (
	parseArticles = flag.Bool("parseArticles", false, "If true, parse article content via Mercury API.")
)

type imagePair struct {
	id      int64
	mime    string
	favicon []byte
}

// Start starts continuous feed fetching and writes fetched articles to the database.
func Start(ctx context.Context, d *storage.Database) {
	log.Infof("Starting continuous feed fetching.")

	// Turn off logging of HTTP icon requests.
	besticon.SetLogOutput(ioutil.Discard)

	feeds, err := d.GetAllFeeds()
	if err != nil {
		log.Infof("Failed to fetch all feeds: %s", err)
	}
	utils.DebugPrint("Feed list", feeds)

	wg := &sync.WaitGroup{}
	wg.Add(len(feeds))
	ac := make(chan models.Article)
	defer close(ac)
	ic := make(chan imagePair)
	defer close(ic)

	for _, f := range feeds {
		go func(f models.Feed) {
			defer wg.Done()
			do(ctx, d, ac, ic, f)
		}(f)
	}

	for {
		select {
		case a := <-ac:
			utils.DebugPrint("Received a new article:", a)
			if err2 := d.InsertArticle(a); err2 != nil {
				log.Warningf("Failed to persist article: %+v: %s", a, err2)
			}
		case ip := <-ic:
			utils.DebugPrint("Received a new image:", ip)
			if err2 := d.InsertFavicon(ip.id, ip.mime, ip.favicon); err2 != nil {
				log.Warningf("Failed to persist icon for feed %d: %s", ip.id, err2)
			}
		case <-ctx.Done():
			log.Infof("Stopping fetching feeds...")
			wg.Wait()
			log.Infof("Stopped fetching feeds.")
			return
		}
	}
}

func do(ctx context.Context, d *storage.Database, ac chan models.Article, ic chan imagePair, feed models.Feed) {
	log.Infof("Fetching %s", feed.URL)
	f, err := rss.Fetch(feed.URL)
	if err != nil {
		log.Warningf("Error fetching %s: %s", feed.URL, err)
		return
	}
	handleItems(&feed, d, f.Items, ac)
	handleImage(feed, f, ic)

	tick := time.After(time.Until(f.Refresh))
	log.Infof("Waiting to fetch %s until %s\n", feed.URL, f.Refresh)

	for {
		select {
		case <-tick:
			log.Infof("Fetching feed %s", feed.URL)
			var refresh time.Time
			if f, err = rss.Fetch(feed.URL); err != nil {
				log.Warningf("Error fetching %s: %s", feed.URL, err)
				// If the request transiently fails, try again after a fixed interval.
				refresh = time.Now().Add(10 * time.Minute)
			} else {
				handleItems(&feed, d, f.Items, ac)
				refresh = f.Refresh
			}
			log.Infof("Waiting to fetch %s until %s\n", feed.URL, refresh)
			tick = time.After(time.Until(refresh))
		case <-ctx.Done():
			return
		}
	}
}

func handleItems(feed *models.Feed, d *storage.Database, items []*rss.Item, send chan models.Article) {
	latest := feed.Latest
	for _, item := range items {
		parsed := ""
		if *parseArticles {
			if p, err := parseArticleContent(item.Link); err != nil {
				log.Warningf("Parsing content failed: %s", err)
			} else {
				parsed = p
			}
		}

		a := models.Article{
			FeedID:    feed.ID,
			FolderID:  feed.FolderID,
			Title:     item.Title,
			Summary:   item.Summary,
			Content:   item.Content,
			Parsed:    parsed,
			Link:      item.Link,
			Date:      item.Date,
			Read:      item.Read,
			Retrieved: time.Now(),
		}
		if a.Date.After(latest) {
			send <- a
			latest = a.Date
		} else {
			log.V(2).Infof("Not persisting too old article: %+v", a)
		}
	}

	err := d.UpdateLatestTimeForFeed(feed.ID, latest)
	if err != nil {
		log.Warningf("Failed to update latest feed time: %s", err)
	} else {
		feed.Latest = latest
	}
}

func handleImage(feed models.Feed, f *rss.Feed, send chan imagePair) {
	var icon besticon.Icon
	var feedHost string

	u, err := url.Parse(f.Link)
	if err == nil {
		feedHost = u.Hostname()
	}

	if i, err2 := tryIconFetch(f.Image.URL); err2 == nil {
		icon = i
	} else if i, err2 = tryIconFetch(f.Link); err2 == nil {
		icon = i
	} else if i, err2 = tryIconFetch(feedHost); err2 == nil {
		icon = i
	} else {
		return
	}

	send <- imagePair{feed.ID, "image/" + icon.Format, icon.ImageData}
}

func tryIconFetch(link string) (besticon.Icon, error) {
	icon := besticon.Icon{}

	if link == "" {
		return icon, errors.New("invalid URL")
	}

	finder := besticon.IconFinder{}

	icons, err := finder.FetchIcons(link)
	if err != nil {
		return icon, err
	}

	if len(icons) == 0 {
		return icon, errors.New("no icons found")
	}

	for _, i := range icons {
		if i.URL != "" && i.Format != "" {
			return i, nil
		}
	}

	return icon, errors.New("no suitable icons found")
}

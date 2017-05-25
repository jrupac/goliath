package fetch

import (
	"context"
	"github.com/SlyMarbo/rss"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/goliath/utils"
	"sync"
	"time"
	"github.com/mat/besticon/besticon"
	"net/url"
	"errors"
	"io/ioutil"
)

type imagePair struct {
	id      int64
	mime    string
	favicon []byte
}

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
			do(ctx, ac, ic, f)
		}(f)
	}

	for {
		select {
		case a := <-ac:
			utils.DebugPrint("Received a new article:", a)
			if err := d.InsertArticle(a); err != nil {
				log.Warningf("Failed to persist article: %+v: %s", a, err)
			}
		case ip := <-ic:
			utils.DebugPrint("Received a new image:", ip)
			if err := d.InsertFavicon(ip.id, ip.mime, ip.favicon); err != nil {
				log.Warningf("Failed to persist icon for feed %d: %s", ip.id, err)
			}
		case <-ctx.Done():
			log.Infof("Stopping fetching feeds...")
			wg.Wait()
			log.Infof("Stopped fetching feeds.")
			return
		}
	}
}

func do(ctx context.Context, ac chan models.Article, ic chan imagePair, feed models.Feed) {
	log.Infof("Fetching %s", feed.Url)
	f, err := rss.Fetch(feed.Url)
	if err != nil {
		log.Warningf("Error fetching %s: %s", feed.Url, err)
		return
	}
	handleItems(feed, f.Items, ac)
	handleImage(feed, f, ic)

	tick := time.After(time.Until(f.Refresh))
	log.Infof("Waiting to fetch %s until %s\n", feed.Url, f.Refresh)

	for {
		select {
		case <-tick:
			log.Infof("Fetching feed %s", feed.Url)
			if err = f.Update(); err != nil {
				log.Warningf("Error fetching %s: %s", feed.Url, err)
			} else {
				handleItems(feed, f.Items, ac)
			}
			log.Infof("Waiting to fetch %s until %s\n", feed.Url, f.Refresh)
			tick = time.After(time.Until(f.Refresh))
		case <-ctx.Done():
			return
		}
	}
}

func handleItems(feed models.Feed, items []*rss.Item, send chan models.Article) {
	for _, item := range items {
		a := models.Article{
			FeedId:   feed.Id,
			FolderId: feed.FolderId,
			Title:    item.Title,
			Summary:  item.Summary,
			Content:  item.Content,
			Link:     item.Link,
			Date:     item.Date,
			Read:     item.Read,
			Retrieved: time.Now(),
		}
		send <- a
	}
}

func handleImage(feed models.Feed, f *rss.Feed, send chan imagePair) {
	var icon besticon.Icon
	var feedHost string

	u, err := url.Parse(f.Link)
	if err == nil {
		feedHost = u.Hostname()
	}

	if i, err := tryIconFetch(f.Image.Url); err == nil {
		icon = i
	} else if i, err := tryIconFetch(f.Link); err == nil {
		icon = i
	} else if i, err = tryIconFetch(feedHost); err == nil {
		icon = i
	} else {
		return
	}

	send <- imagePair{feed.Id, "image/" + icon.Format, icon.ImageData}
}

func tryIconFetch(link string) (besticon.Icon, error) {
	icon := besticon.Icon{}

	if link == "" {
		return icon, errors.New("Invalid URL.")
	}

	finder := besticon.IconFinder{}

	icons, err := finder.FetchIcons(link);
	if err != nil {
		return icon, err
	}

	if len(icons) == 0 {
		return icon, errors.New("No icons found.")
	}

	for _, i := range icons {
		if i.URL != "" && i.Format != "" {
			return i, nil
		}
	}

	return icon, errors.New("No suitable icons found.")
}
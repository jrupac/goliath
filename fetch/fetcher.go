package fetch

import (
	"context"
	"github.com/SlyMarbo/rss"
	log "github.com/golang/glog"
	"github.com/jrupac/goliath/models"
	"github.com/jrupac/goliath/storage"
	"github.com/jrupac/goliath/utils"
	"io/ioutil"
	"sync"
	"time"
)

type imagePair struct {
	id  int64
	img []byte
}

func Start(ctx context.Context, d *storage.Database, feeds []models.Feed) {
	log.Infof("Starting continuous feed fetching.")

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
			if err := d.InsertFeedIcon(ip.id, ip.img); err != nil {
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
	handleImage(feed, f.Image, ic)
	handleItems(feed, ac, f.Items)

	tick := time.After(time.Until(f.Refresh))
	log.Infof("Waiting to fetch %s until %s\n", feed.Url, f.Refresh)

	for {
		select {
		case <-tick:
			log.Infof("Fetching feed %s", feed.Url)
			if err = f.Update(); err != nil {
				log.Warningf("Error fetching %s: %s", feed.Url, err)
			} else {
				handleItems(feed, ac, f.Items)
			}
			log.Infof("Waiting to fetch %s until %s\n", feed.Url, f.Refresh)
			tick = time.After(time.Until(f.Refresh))
		case <-ctx.Done():
			return
		}
	}
}

func handleItems(feed models.Feed, send chan models.Article, items []*rss.Item) {
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
		}
		send <- a
	}
}

func handleImage(feed models.Feed, img *rss.Image, send chan imagePair) {
	body, err := img.Get()
	if err != nil {
		log.Warningf("Unable to fetch icon for feed %d: %s", feed.Id, err)
		return
	}
	defer body.Close()

	src, err := ioutil.ReadAll(body)
	if err != nil {
		log.Warningf("Unable to decode icon for feed %d: %s", feed.Id, err)
	}
	send <- imagePair{feed.Id, src}
}

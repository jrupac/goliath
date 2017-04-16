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
)

func Do(ctx context.Context, d *storage.Database, feeds []models.Feed) {
	wg := &sync.WaitGroup{}
	wg.Add(len(feeds))
	ac := make(chan models.Article)
	for _, f := range feeds {
		go do(ctx, ac, wg, f)
	}

	for {
		select {
		case a := <-ac:
			utils.DebugPrint("Received a new article:", a)
			if err := d.InsertArticle(a); err != nil {
				log.Warningf("Failed to persist article: %+v: %s", a, err)
			}
		case <-ctx.Done():
			log.Infof("Stopping fetching feeds...")
			wg.Wait()
			close(ac)
			log.Infof("Stopped fetching feeds.")
			return
		}
	}
}

func do(ctx context.Context, send chan models.Article, wg *sync.WaitGroup, feed models.Feed) {
	defer wg.Done()
	log.Infof("Fetching %s", feed.Url)
	f, err := rss.Fetch(feed.Url)
	if err != nil {
		log.Warningf("Error fetching %s: %s", feed.Url, err)
		return
	}
	handleItems(feed, send, f.Items)

	tick := time.After(time.Until(f.Refresh))
	log.Infof("Waiting to fetch %s until %s\n", feed.Url, f.Refresh)

	for {
		select {
		case <-tick:
			log.Infof("Fetching feed %s", feed.Url)
			if err = f.Update(); err != nil {
				log.Warningf("Error fetching %s: %s", feed.Url, err)
				break
			} else {
				handleItems(feed, send, f.Items)
				tick = time.After(time.Until(f.Refresh))
				log.Infof("Waiting to fetch %s until %s\n", feed.Url, f.Refresh)
			}
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

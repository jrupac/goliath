package storage

import (
	"context"
	"flag"
	"time"

	log "github.com/golang/glog"
)

var (
	gcInterval     = flag.Duration("gcInterval", 24*time.Hour, "Duration between GC runs against the articles table.")
	gcKeepDuration = flag.Duration("gcKeepDuration", 7*24*time.Hour, "Duration to keep read articles.")
)

// StartGC starts continuously garbage-collecting old read articles on a regular interval.
func StartGC(ctx context.Context, d Database) {
	log.Infof("Starting initial GC run.")
	performGCRun(d)

	tick := time.After(*gcInterval)

	for {
		select {
		case <-tick:
			log.Infof("Starting GC run.")
			performGCRun(d)
			tick = time.After(*gcInterval)
		case <-ctx.Done():
			return
		}
	}
}

func performGCRun(d Database) {
	collectFeeds(d)
	collectArticles(d)
	collectSessions(d)
}

// collectFeeds removes the feeds users have unsubscribed from, and their
// articles. Unsubscribing only marks a feed, so that the request does not wait
// on deleting everything it held; this is where that deletion happens.
func collectFeeds(d Database) {
	feeds, articles, err := d.PurgeDeletedFeeds(time.Now())
	if err != nil {
		log.Warningf("Feed GC run failed: %s", err)
		return
	}
	log.Infof("Feed GC complete; deleted %d unsubscribed feeds and %d articles.", feeds, articles)
}

func collectArticles(d Database) {
	users, err := d.GetAllUsers()
	if err != nil {
		log.Warningf("Failed to query all users: %s", err)
		return
	}

	minTimestamp := time.Now().Add(-1 * *gcKeepDuration)
	log.Infof("GC'ing all read articles older than: %s", minTimestamp)

	for _, user := range users {
		count, err := d.DeleteArticlesForUser(user, minTimestamp)
		if err != nil {
			log.Warningf("GC run failed for user %s: %s", user, err)
		} else {
			log.Infof("GC complete for user %s; deleted %d articles.", user, count)
		}
	}
}

// collectSessions reclaims rows for sessions that have gone idle past the
// expiry window. Expiry is enforced on every lookup, so an expired session is
// already unusable well before this runs; deleting it only keeps the table from
// accumulating rows nothing will ever match again.
func collectSessions(d Database) {
	cutoff := SessionExpiryCutoff()
	log.Infof("GC'ing all sessions unused since: %s", cutoff)

	count, err := d.DeleteExpiredSessions(cutoff)
	if err != nil {
		log.Warningf("Session GC run failed: %s", err)
		return
	}
	log.Infof("Session GC complete; deleted %d sessions.", count)
}

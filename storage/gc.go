package storage

import (
	"flag"
	"time"
	"context"
	log "github.com/golang/glog"
)

var (
	gcInterval = flag.Duration("gcInterval", 24 * time.Hour, "Duration between GC runs against the articles table.")
	gcKeepDuration = flag.Duration("gcKeepDuration", 7 * 24 * time.Hour, "Duration to keep read articles.")
)

func StartGc(ctx context.Context, d *Database) {
	log.Infof("Starting initial GC run.")
	PerformGcRun(d)

	tick := time.After(*gcInterval)

	for {
		select {
		case <- tick:
			log.Infof("Starting GC run.")
			PerformGcRun(d)
			tick = time.After(*gcInterval)
		case <- ctx.Done():
			return
		}
	}
}

func PerformGcRun(d *Database) {
	minTimestamp := time.Now().Add(-1 * *gcKeepDuration)
	log.Infof("GC'ing all read articles older than: %s", minTimestamp)
	count, err := d.DeleteArticles(minTimestamp)
	if err != nil {
		log.Warningf("GC run failed: %s", err)
	} else {
		log.Infof("GC complete; deleted %d articles.", count)
	}
}
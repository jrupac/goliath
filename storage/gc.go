package storage

import (
	"flag"
	"time"
	"context"
	log "github.com/golang/glog"
)

var (
	gcInterval = flag.Duration("gcInterval", 24 * time.Hour, "Duration between GC runs against the articles table.")
)

func StartGc(ctx context.Context, d *Database) {
	var interval time.Duration
	if *gcInterval < 1 * time.Hour {
		log.Warningf("GC interval too short, setting to 1 hour.")
		interval = 1 * time.Hour
	} else {
		interval = *gcInterval
	}

	log.Infof("Starting initial GC run.")
	err := PerformGcRun(d)
	if err != nil {
		log.Warningf("GC run failed: %s", err)
	}

	tick := time.After(interval)

	for {
		select {
		case <- tick:
			log.Infof("Starting GC run.")
			err := PerformGcRun(d)
			if err != nil {
				log.Warningf("GC run failed: %s", err)
			}
			tick = time.After(interval)
		case <- ctx.Done():
			return
		}
	}

}

func PerformGcRun(_ *Database) error {
	log.Warningf("TODO: Implement GC!")
	return nil
}
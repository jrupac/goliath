package main

import (
	log "github.com/golang/glog"
	"net/http"
)

func HandleFever(_ http.ResponseWriter, r *http.Request) {
	log.Infof("%s", *r)
}

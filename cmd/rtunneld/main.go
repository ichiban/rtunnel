package main

import (
	"flag"
	"net/http"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/ichiban/rtunnel"
)

func main() {
	flag.Parse()

	var wg sync.WaitGroup
	for _, a := range flag.Args() {
		a := a
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := http.ListenAndServe(a, &rtunnel.Tunnel{}); err != nil {
				log.WithFields(log.Fields{
					"addr": a,
					"err":  err,
				}).Error("failed to serve tunnel")
			}
		}()
	}
	wg.Wait()
}

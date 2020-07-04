package main

import (
	"flag"
	"sync"

	"github.com/ichiban/rtunnel"
)

func main() {
	flag.Parse()

	var wg sync.WaitGroup
	for _, h := range flag.Args() {
		a := rtunnel.Agent{Tunnel: h}
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.Run()
		}()
	}
	wg.Wait()
}

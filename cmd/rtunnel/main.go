package main

import (
	"flag"
	"sync"

	"github.com/ichiban/rtunnel"
)

func main() {
	flag.Parse()

	var wg sync.WaitGroup
	for _, u := range flag.Args() {
		e := rtunnel.Exit{Entrance: u}
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.Run()
		}()
	}
	wg.Wait()
}

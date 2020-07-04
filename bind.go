package rtunnel

import (
	"io"
	"sync"
)

func Bind(a, b io.ReadWriteCloser) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { _ = a.Close() }()

		for {
			n, err := io.Copy(a, b)
			if err != nil {
				return
			}
			if n == 0 {
				return
			}
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() { _ = b.Close() }()

		for {
			n, err := io.Copy(b, a)
			if err != nil {
				return
			}
			if n == 0 {
				return
			}
		}
	}()
	wg.Wait()
}

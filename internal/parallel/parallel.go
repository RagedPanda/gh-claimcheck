// Package parallel runs indexed work with bounded concurrency.
package parallel

import "sync"

// ForEach calls work for every index in [0, count), with at most limit calls running at once.
// Callers write results into their own slot of a pre-sized slice, so no locking is needed.
func ForEach(count, limit int, work func(index int)) {
	slots := make(chan struct{}, max(limit, 1))
	var group sync.WaitGroup
	for index := 0; index < count; index++ {
		group.Add(1)
		slots <- struct{}{}
		go func(index int) {
			defer func() { <-slots; group.Done() }()
			work(index)
		}(index)
	}
	group.Wait()
}

package parallel

import (
	"sync/atomic"
	"testing"
)

func TestForEachRunsAllWithinLimit(t *testing.T) {
	var running, peak atomic.Int32
	results := make([]int, 50)
	ForEach(len(results), 3, func(index int) {
		now := running.Add(1)
		for {
			old := peak.Load()
			if now <= old || peak.CompareAndSwap(old, now) {
				break
			}
		}
		results[index] = index * 2
		running.Add(-1)
	})
	for index, value := range results {
		if value != index*2 {
			t.Fatalf("results[%d] = %d", index, value)
		}
	}
	if peak.Load() > 3 {
		t.Errorf("peak concurrency = %d, want <= 3", peak.Load())
	}
}

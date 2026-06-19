package audio

import (
	"sync"
	"testing"

	"github.com/audiolibrelab/jamcapture/internal/config"
)

// TestGetChannelStatusConcurrentNoRace exercises GetChannelStatus from multiple
// goroutines at once. GetChannelStatus only takes an RLock, but the
// scanChannelStatus it calls writes the shared r.channelStatusCache and
// r.channelStatusCacheTime fields. Concurrent readers therefore write those
// fields under a shared (read) lock -> data race.
//
// The config has no channels, so scanChannelStatus performs no pw-link calls:
// the test is hardware-free, fast, and deterministic. Run with -race to detect
// the bug; it must stay clean once the locking is fixed.
func TestGetChannelStatusConcurrentNoRace(t *testing.T) {
	cfg := &config.Config{} // no channels -> scanChannelStatus shells out to nothing
	r := NewPipeWireRecorder(cfg, nil)

	const goroutines = 8
	const iterations = 200

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = r.GetChannelStatus()
			}
		}()
	}
	wg.Wait()
}

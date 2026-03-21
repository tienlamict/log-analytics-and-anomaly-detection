package detection

import "time"

// slidingWindow tracks event timestamps in a fixed-capacity circular buffer.
// Timestamps are not required to be monotonic (Kafka delivery may be out of order),
// so CountWithin scans all entries rather than short-circuiting.
type slidingWindow struct {
	timestamps []time.Time
	head       int // next write position (mod len)
	count      int // number of valid entries (≤ capacity)
}

// newSlidingWindow allocates a sliding window with the given capacity.
func newSlidingWindow(capacity int) *slidingWindow {
	return &slidingWindow{timestamps: make([]time.Time, capacity)}
}

// Add records a new timestamp, overwriting the oldest entry when the buffer is full.
func (w *slidingWindow) Add(t time.Time) {
	w.timestamps[w.head%len(w.timestamps)] = t
	w.head++
	if w.count < len(w.timestamps) {
		w.count++
	}
}

// CountWithin returns the number of timestamps that fall within the last d duration.
// All entries are scanned because Kafka delivery order is not guaranteed monotonic.
func (w *slidingWindow) CountWithin(d time.Duration) int {
	cutoff := time.Now().Add(-d)
	n := 0
	for i := 0; i < w.count; i++ {
		idx := (w.head - 1 - i + len(w.timestamps)) % len(w.timestamps)
		if w.timestamps[idx].After(cutoff) {
			n++
		}
	}
	return n
}

// Evict removes timestamps older than olderThan from the buffer, compacting in place.
func (w *slidingWindow) Evict(olderThan time.Duration) {
	cutoff := time.Now().Add(-olderThan)
	newTs := w.timestamps[:0]
	for i := 0; i < w.count; i++ {
		idx := (w.head - w.count + i + len(w.timestamps)) % len(w.timestamps)
		if w.timestamps[idx].After(cutoff) {
			newTs = append(newTs, w.timestamps[idx])
		}
	}
	copy(w.timestamps, newTs)
	w.count = len(newTs)
	w.head = w.count
}

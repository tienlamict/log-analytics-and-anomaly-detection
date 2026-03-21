package detection

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// TestSlidingWindow_AddAndCount verifies that timestamps added within the last
// window duration are counted correctly.
func TestSlidingWindow_AddAndCount(t *testing.T) {
	w := newSlidingWindow(100)
	now := time.Now()
	for i := 0; i < 5; i++ {
		w.Add(now.Add(-time.Duration(i) * time.Second))
	}
	assert.Equal(t, 5, w.CountWithin(1*time.Minute))
}

// TestSlidingWindow_CountExcludesOld verifies that timestamps outside the window
// are not counted.
func TestSlidingWindow_CountExcludesOld(t *testing.T) {
	w := newSlidingWindow(100)
	now := time.Now()
	// 3 old timestamps (10 minutes ago)
	for i := 0; i < 3; i++ {
		w.Add(now.Add(-10 * time.Minute))
	}
	// 2 recent timestamps (30 seconds ago)
	for i := 0; i < 2; i++ {
		w.Add(now.Add(-30 * time.Second))
	}
	assert.Equal(t, 2, w.CountWithin(5*time.Minute))
}

// TestSlidingWindow_CircularOverwrite verifies that the capacity cap is enforced
// and that overwritten entries are replaced correctly.
func TestSlidingWindow_CircularOverwrite(t *testing.T) {
	capacity := 3
	w := newSlidingWindow(capacity)
	now := time.Now()

	// Add 5 entries — buffer must overwrite oldest 2.
	for i := 0; i < 5; i++ {
		w.Add(now.Add(-time.Duration(i) * time.Second))
	}

	// Count must never exceed capacity.
	require.Equal(t, capacity, w.count)

	// All entries that are within 1 minute should be counted (most recent 3 of 5).
	assert.Equal(t, 3, w.CountWithin(1*time.Minute))
}

// TestSlidingWindow_Evict verifies that Evict removes entries older than the
// supplied duration and retains recent ones.
func TestSlidingWindow_Evict(t *testing.T) {
	w := newSlidingWindow(100)
	now := time.Now()

	// 3 old entries (10 minutes ago)
	for i := 0; i < 3; i++ {
		w.Add(now.Add(-10 * time.Minute))
	}
	// 2 recent entries (30 seconds ago)
	for i := 0; i < 2; i++ {
		w.Add(now.Add(-30 * time.Second))
	}

	w.Evict(5 * time.Minute)

	// After eviction only 2 recent entries remain.
	assert.Equal(t, 2, w.count)
	assert.Equal(t, 2, w.CountWithin(5*time.Minute))
}

// TestSlidingWindow_Empty verifies that a fresh window has count zero.
func TestSlidingWindow_Empty(t *testing.T) {
	w := newSlidingWindow(100)
	assert.Equal(t, 0, w.CountWithin(1*time.Minute))
	assert.Equal(t, 0, w.count)
}

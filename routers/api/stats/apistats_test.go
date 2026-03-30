package stats

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// resetState is a helper function to ensure the global accumulator is cleared
// before each test function runs, preventing tests from interfering with each other.
func resetState() {
	// To reset, we just call the function that gets and resets.
	// We don't need the return value here.
	GetStatisticsAndReset()
}

// TestAdd_SingleItem tests the basic case of adding just one data point.
func TestAdd_SingleItem(t *testing.T) {
	// ARRANGE: Ensure a clean state before the test.
	resetState()

	// ACT: Add a single value.
	Add(10.5, 200)

	// ASSERT: Check if the stats are correct for a single item.
	stats := GetStatisticsAndReset()
	assert.Equal(t, int64(1), stats.Count, "Count should be 1")
	assert.Equal(t, 10.5, stats.Min, "Min should be the value itself")
	assert.Equal(t, 10.5, stats.Max, "Max should be the value itself")
	assert.Equal(t, 10.5, stats.Avg, "Average should be the value itself")
	assert.Equal(t, int64(1), stats.Status2xx, "Status2xx count should be 1")
	assert.Zero(t, stats.Status1xx, "Other status counts should be zero")
	assert.Zero(t, stats.Status3xx, "Other status counts should be zero")
	assert.Zero(t, stats.Status4xx, "Other status counts should be zero")
	assert.Zero(t, stats.Status5xx, "Other status counts should be zero")
	assert.Zero(t, stats.StatusUnknown, "Other status counts should be zero")
}

// TestAdd_MultipleItems tests the accumulator with a variety of data points.
func TestAdd_MultipleItems(t *testing.T) {
	// ARRANGE
	resetState()
	// The values are: 10, 20, 60. Sum = 90. Count = 3. Avg = 30.
	// Min = 10, Max = 60.
	// Statuses: 2xx: 1, 4xx: 1, 5xx: 1

	// ACT
	Add(10, 200)
	Add(20, 404)
	Add(60, 503)

	// ASSERT
	stats := GetStatisticsAndReset()
	assert.Equal(t, int64(3), stats.Count)
	assert.Equal(t, 10.0, stats.Min)
	assert.Equal(t, 60.0, stats.Max)
	// Use InDelta for floating point comparisons to avoid precision issues.
	assert.InDelta(t, 30.0, stats.Avg, 0.001)
	assert.Equal(t, int64(1), stats.Status2xx)
	assert.Equal(t, int64(1), stats.Status4xx)
	assert.Equal(t, int64(1), stats.Status5xx)
	assert.Zero(t, stats.StatusUnknown)
}

// TestGetStatisticsAndReset verifies that the function returns the correct data
// AND clears the global state afterward.
func TestGetStatisticsAndReset(t *testing.T) {
	// ARRANGE
	resetState()
	Add(100, 200)

	// ACT: Get the stats, which should also reset them.
	stats := GetStatisticsAndReset()

	// ASSERT (Part 1): Check if the returned stats are correct.
	assert.Equal(t, int64(1), stats.Count)
	assert.Equal(t, 100.0, stats.Avg)

	// ASSERT (Part 2): Call the function again and check if it's now empty.
	emptyStats := GetStatisticsAndReset()
	assert.Zero(t, emptyStats.Count, "Count should be 0 after reset")
	assert.Zero(t, emptyStats.Avg, "Avg should be 0 after reset")
	assert.Zero(t, emptyStats.Min, "Min should be 0 after reset")
	assert.Zero(t, emptyStats.Max, "Max should be 0 after reset")
	assert.Zero(t, emptyStats.Status2xx, "Status2xx should be 0 after reset")
}

// TestAdd_Concurrency tests that the mutex correctly protects the accumulator
// from data races when Add is called from multiple goroutines.
func TestAdd_Concurrency(t *testing.T) {
	// ARRANGE
	resetState()
	var wg sync.WaitGroup

	// Number of concurrent operations
	goroutines := 100
	iterations := 10
	totalOps := int64(goroutines * iterations)

	// ACT: Start many goroutines that all call Add concurrently.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// We add a consistent value to make the average predictable.
				// We also add a consistent status code.
				Add(10.0, 200)
			}
		}()
	}

	// Wait for all goroutines to finish.
	wg.Wait()

	// ASSERT: Check the final state.
	stats := GetStatisticsAndReset()
	assert.Equal(t, totalOps, stats.Count, "Count should be the total number of operations")
	assert.Equal(t, 10.0, stats.Min, "Min should be 10")
	assert.Equal(t, 10.0, stats.Max, "Max should be 10")
	assert.InDelta(t, 10.0, stats.Avg, 0.001, "Average should be 10")
	assert.Equal(t, totalOps, stats.Status2xx, "Status2xx count should match total operations")
}

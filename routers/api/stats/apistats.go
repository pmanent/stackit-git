package stats

import "sync"

// Stats holds the accumulated statistical data, including HTTP status counts.
type Stats struct {
	Count         int64
	Min           float64
	Max           float64
	Avg           float64
	Status1xx     int64
	Status2xx     int64
	Status3xx     int64
	Status4xx     int64
	Status5xx     int64
	StatusUnknown int64
}

// Global variables for the accumulator and the mutex to protect it.
var (
	accumulator Stats
	mutex       sync.Mutex
)

// Add updates the stats with a new value and an HTTP status code in a thread-safe manner.
func Add(value float64, httpStatus int) {
	mutex.Lock()
	defer mutex.Unlock()

	// --- Update value statistics (min, max, avg) ---
	old_count := float64(accumulator.Count)
	accumulator.Count++

	if accumulator.Count == 1 {
		accumulator.Min = value
		accumulator.Max = value
		accumulator.Avg = value
	} else {
		if value < accumulator.Min {
			accumulator.Min = value
		}
		if value > accumulator.Max {
			accumulator.Max = value
		}
		accumulator.Avg = (accumulator.Avg*old_count + value) / float64(accumulator.Count)
	}

	// --- Update HTTP status counts ---
	switch {
	case httpStatus >= 100 && httpStatus < 200:
		accumulator.Status1xx++
	case httpStatus >= 200 && httpStatus < 300:
		accumulator.Status2xx++
	case httpStatus >= 300 && httpStatus < 400:
		accumulator.Status3xx++
	case httpStatus >= 400 && httpStatus < 500:
		accumulator.Status4xx++
	case httpStatus >= 500 && httpStatus < 600:
		accumulator.Status5xx++
	default:
		accumulator.StatusUnknown++
	}
}

// GetStatisticsAndReset returns a copy of the current stats and then resets the
// global accumulator to zero. The entire operation is thread-safe.
func GetStatisticsAndReset() Stats {
	mutex.Lock()
	defer mutex.Unlock()

	statsToReturn := accumulator
	accumulator = Stats{}
	return statsToReturn
}

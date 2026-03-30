package numbers

import (
	"math"
)

func RoundUpToTwoSignificantDigits(value float64) float64 {
	if value == 0 {
		return 0 // O puedes decidir qué hacer con 0, en este caso lo mantenemos.
	}

	magnitude := math.Floor(math.Log10(math.Abs(value)))

	// scaleFactor := math.Pow(10, 1-magnitude)

	adjustedValue := math.Abs(value) * math.Pow(10, 2-magnitude)
	roundedValue := math.Ceil(adjustedValue) / math.Pow(10, 2-magnitude)

	return math.Copysign(roundedValue, value)
}

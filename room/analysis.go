package room

import (
	"math"
)

const MS float64 = 1.0 / 1000.0

func EnergyOverWindow(arrivals []Arrival, windowMS float64, floor float64) (float64, error) {
	totalGain := 0.0
	for _, arrival := range arrivals {
		if arrival.Distance/SPEED_OF_SOUND/MS < windowMS {
			totalGain = totalGain + (math.Abs(floor - toDB(arrival.Gain)))
		}
	}
	return totalGain, nil
}

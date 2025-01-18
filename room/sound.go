package room

import (
	"math"
)

const SPEED_OF_SOUND = 343.0

func toDB(gain float64) float64 {
	return 10 * math.Log10(gain)
}

func fromDB(gainDB float64) float64 {
	return math.Pow(10, gainDB/10)
}

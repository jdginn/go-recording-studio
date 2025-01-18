package main

import (
	"fmt"

	"github.com/fogleman/pt/pt"

	goroom "github.com/jdginn/go-recording-studio/room"
)

const MS float64 = 1.0 / 1000.0

func main() {
	room, err := goroom.NewRoom("testdata/Cutout.3mf")
	if err != nil {
		panic(err)
	}

	fmt.Println(room.M.BoundingBox().Max)

	source := goroom.Source{
		Directivity: goroom.NewDirectivity(map[float64]float64{0: 1}, map[float64]float64{0: 1}),
		Position: pt.Vector{
			X: 0.25,
			Y: 0.25,
			Z: 0.25,
		},
		NormalDirection: pt.Vector{
			X: 1.0,
			Y: 0,
			Z: 0,
		},
	}

	listenPos := pt.Vector{
		X: 2.0,
		Y: 1.5,
		Z: 1.7,
	}

	arrivals := []goroom.Arrival{}

	for _, shot := range source.Sample(100000, 180, 180) {
		arrival, err := room.TraceShot(shot, listenPos, goroom.TraceParams{
			Order:         10,
			GainThreshold: -20,
			TimeThreshold: 1 * MS,
			RFZRadius:     0.1,
		})
		if err != nil {
			panic(err)
		}
		if arrival.Distance != goroom.INF {
			arrivals = append(arrivals, arrival)
		}
	}

	// for _, arrival := range arrivals {
	// 	if arrival.Distance != goroom.INF {
	// 		fmt.Printf("Delay: %fms; Gain: %f, Total reflections: %d\n", arrival.Distance/goroom.SPEED_OF_SOUND*1000, arrival.Gain, len(arrival.AllPos))
	// 	}
	// }
	fmt.Printf("%d arrivals", len(arrivals))
}

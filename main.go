package main

import (
	"sort"

	"github.com/fogleman/gg"

	goroom "github.com/jdginn/go-recording-studio/room"
)

const MS float64 = 1.0 / 1000.0

const SCALE float64 = 100

func main() {
	room, err := goroom.NewFrom3MF("testdata/Cutout.3mf")
	if err != nil {
		panic(err)
	}

	listening_triangle := goroom.ListeningTriangle{
		ReferencePosition: goroom.V(0, 2.0, 0.5),
		ReferenceNormal:   goroom.V(-1, 0, 0),
		DistFromFront:     0.5,
		DistFromCenter:    0.7,
		SourceHeight:      1.7,
		ListenHeight:      1.4,
	}

	source := goroom.Source{
		// D:               goroom.NewDirectivity(map[float64]float64{0: 1}, map[float64]float64{0: 1}),
		Position:        listening_triangle.LeftSourcePosition(),
		NormalDirection: listening_triangle.LeftSourceNormal(),
	}

	arrivals := []goroom.Arrival{}

	for _, shot := range source.Sample(100, 180, 180) {
		arrival, err := room.TraceShot(shot, listening_triangle.ListenPosition(), goroom.TraceParams{
			Order:         10,
			GainThreshold: -20,
			TimeThreshold: 1 * MS,
			RFZRadius:     0.5,
		})
		if err != nil {
			panic(err)
		}
		if arrival.Distance != goroom.INF {
			arrivals = append(arrivals, arrival)
		}
	}

	sort.Slice(arrivals, func(i int, j int) bool {
		return arrivals[i].Distance < arrivals[j].Distance
	})

	p1 := goroom.MakePlane(goroom.V(0.25, 0.5, 0), goroom.V(1, 0, 0))
	p2 := goroom.MakePlane(goroom.V(0.25, 0.5, 0.75), goroom.V(0, 0, 1))

	view := goroom.View{
		C:          gg.NewContext(1000, 1000),
		TranslateX: 400,
		TranslateY: 400,
		Scale:      100,
		Plane:      p1,
	}

	scene := goroom.Scene{
		Sources:           []goroom.Source{source},
		ListeningPosition: listening_triangle.ListenPosition(),
		Room:              &room,
	}

	scene.PlotArrivals(arrivals, view)
	view.Save("out1.png")
	view.Plane = p2
	scene.PlotArrivals(arrivals, view)
	view.Save("out2.png")
}

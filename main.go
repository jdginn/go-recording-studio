package main

import (
	"fmt"
	"sort"

	"github.com/fogleman/gg"
	"github.com/fogleman/pt/pt"

	goroom "github.com/jdginn/go-recording-studio/room"
)

const MS float64 = 1.0 / 1000.0

const SCALE float64 = 100

func main() {
	room, err := goroom.NewFrom3MF("testdata/Cutout.3mf")
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

	for _, shot := range source.Sample(3, 180, 180) {
		arrival, err := room.TraceShot(shot, listenPos, goroom.TraceParams{
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
	p2 := goroom.MakePlane(goroom.V(0.25, 0.5, 0), goroom.V(0, 1, 0))

	view := goroom.View{
		C:          gg.NewContext(1000, 1000),
		TranslateX: 400,
		TranslateY: 400,
		Scale:      100,
		Plane:      p1,
	}

	scene := goroom.Scene{
		Sources:           []goroom.Source{source},
		ListeningPosition: listenPos,
		Room:              &room,
	}

	scene.PlotArrivals(arrivals, view)
	view.Save("out1.png")
	view.Plane = p2
	scene.PlotArrivals(arrivals, view)
	view.Save("out2.png")
}

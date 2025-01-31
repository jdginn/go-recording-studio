package main

import (
	"sort"

	"github.com/fogleman/gg"

	goroom "github.com/jdginn/go-recording-studio/room"
)

const MS float64 = 1.0 / 1000.0

const SCALE float64 = 100

var (
	BRICK         = goroom.Material{Alpha: 0.04}
	WOOD          = goroom.Material{Alpha: 0.1}
	GYPSUM        = goroom.Material{Alpha: 0.05}
	DIFFUSER      = goroom.Material{Alpha: 0.99}
	ROCKWOOL_12CM = goroom.Material{Alpha: 0.9}
	ROCKWOOL_24CM = goroom.Material{Alpha: 0.996}
	ROCKWOOL_30CM = goroom.Material{Alpha: 0.999}
	GLASS         = goroom.Material{Alpha: 0.0}
)

func main() {
	room, err := goroom.NewFrom3MF("testdata/Cutout.3mf", map[string]goroom.Material{
		"default":            BRICK,
		"Floor":              WOOD,
		"Front A":            GYPSUM,
		"Front B":            GYPSUM,
		"Back Diffuser":      DIFFUSER,
		"Ceiling Diffuser":   ROCKWOOL_24CM,
		"Street A":           ROCKWOOL_24CM,
		"Street B":           ROCKWOOL_24CM,
		"Street C":           ROCKWOOL_24CM,
		"Street D":           ROCKWOOL_24CM,
		"Street E":           ROCKWOOL_24CM,
		"Hall A":             ROCKWOOL_24CM,
		"Hall B":             ROCKWOOL_24CM,
		"Hall E":             ROCKWOOL_24CM,
		"Entry Back":         ROCKWOOL_24CM,
		"Entry Front":        ROCKWOOL_24CM,
		"Cutout Top":         ROCKWOOL_24CM,
		"Window A":           GLASS,
		"Window B":           GLASS,
		"left speaker wall":  GYPSUM,
		"right speaker wall": GYPSUM,
	})
	if err != nil {
		panic(err)
	}

	lt := goroom.ListeningTriangle{
		ReferencePosition: goroom.V(0, 2.0, 0.5),
		ReferenceNormal:   goroom.V(1, 0, 0),
		DistFromFront:     0.526,
		DistFromCenter:    1.3,
		SourceHeight:      1.86,
		ListenHeight:      1.4,
	}

	mum8Spec := goroom.LoudSpeakerSpec{
		Xdim:        0.38,
		Ydim:        0.256,
		Zdim:        0.52,
		Yoff:        0.096,
		Zoff:        0.412,
		Directivity: goroom.NewDirectivity(map[float64]float64{0: 0, 30: 0, 60: -12, 70: -100}, map[float64]float64{0: 0, 30: -9, 60: -15, 70: -19, 80: -30}),
	}

	sources := []goroom.Speaker{
		goroom.NewSpeaker(mum8Spec, lt.LeftSourcePosition(), lt.LeftSourceNormal()),
		goroom.NewSpeaker(mum8Spec, lt.RightSourcePosition(), lt.RightSourceNormal()),
	}

	arrivals := []goroom.Arrival{}

	for _, source := range sources {
		ok, err := source.IsInsideRoom(room.M, lt.ListenPosition())
		if err != nil {
			panic(err)
		}
		if !ok {
			panic("Speaker is not inside room!")
		}
	}

	for _, source := range sources {
		for _, shot := range source.Sample(100, 180, 180) {
			arrival, err := room.TraceShot(shot, lt.ListenPosition(), goroom.TraceParams{
				Order:         10,
				GainThreshold: -20,
				TimeThreshold: 100 * MS,
				RFZRadius:     0.5,
			})
			if err != nil {
				panic(err)
			}
			if arrival.Distance != goroom.INF {
				arrivals = append(arrivals, arrival)
			}
		}
	}

	sort.Slice(arrivals, func(i int, j int) bool {
		return arrivals[i].Distance < arrivals[j].Distance
	})

	p1 := goroom.MakePlane(goroom.V(0.25, 0.5, 0), goroom.V(0, 1, 0))
	p2 := goroom.MakePlane(goroom.V(0.25, 0.5, 0.75), goroom.V(0, 0, 1))

	view := goroom.View{
		C:          gg.NewContext(1000, 1000),
		TranslateX: 400,
		TranslateY: 400,
		Scale:      100,
		Plane:      p1,
	}

	scene := goroom.Scene{
		Sources:           sources,
		ListeningPosition: lt.ListenPosition(),
		Room:              &room,
	}

	scene.PlotArrivals3D(arrivals, view)
	view.Save("out1.png")
	view.Plane = p2
	scene.PlotArrivals3D(arrivals, view)
	view.Save("out2.png")
	scene.PlotITD(arrivals, 30)
}

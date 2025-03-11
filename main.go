package main

import (
	"image"
	"image/png"
	"os"
	"sort"

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

func saveImage(filename string, i image.Image) error {
	f, err := os.Create("out1.png")
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, i)
}

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

	RFZ_RADIUS := 0.5
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
		Directivity: goroom.NewDirectivity(map[float64]float64{0: 0, 30: -3, 60: -12, 70: -100}, map[float64]float64{0: 0, 30: -9, 60: -15, 70: -19, 80: -30}),
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

	room.AddWall(lt.LeftSourcePosition(), lt.LeftSourceNormal())
	room.AddWall(lt.RightSourcePosition(), lt.RightSourceNormal())

	for _, source := range sources {
		for _, shot := range source.Sample(5_000, 180, 180) {
			arrival, err := room.TraceShot(shot, lt.ListenPosition(), goroom.TraceParams{
				Order:         10,
				GainThreshold: -15,
				TimeThreshold: 100 * MS,
				RFZRadius:     RFZ_RADIUS,
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

	room.M.SaveSTL("room.stl")

	if err := goroom.SavePointsArrivalsZonesToJSON("annotations.json", nil, arrivals, []goroom.Zone{{
		Center: lt.ListenPosition(),
		Radius: RFZ_RADIUS,
	}}); err != nil {
		panic(err)
	}
}

package main

import (
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"sort"

	"github.com/alecthomas/kong"

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

var kh310_directivity = goroom.NewDirectivity(
	map[float64]float64{0: 0, 30: -0, 40: -1, 50: -2, 60: -3, 70: -4, 80: -6, 90: -7, 100: -8, 120: -11, 150: -20, 160: -50},
	map[float64]float64{0: 0, 30: -4, 60: -6, 70: -10, 80: -12, 100: -13, 120: -15},
)

var lp8_directivity = goroom.NewDirectivity(
	map[float64]float64{0: 0, 30: -1, 40: -3, 50: -3, 60: -4, 70: -6, 80: -9, 90: -12, 120: -11, 150: -20},
	map[float64]float64{0: 0, 30: 0, 60: -4, 70: -7, 80: -9, 100: -9, 120: -9, 150: -15},
)

func saveImage(filename string, i image.Image) error {
	f, err := os.Create("out1.png")
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, i)
}

var CLI struct {
	Simulate SimulateCmd `cmd:"" help:"Simualte a room"`
}

type SimulateCmd struct {
	Mesh                   string `arg:"" name:"mesh" help:"mesh of room to simulate"`
	SkipSpeakerInRoomCheck bool   `name:"skip-speaker-in-room-check" help:"don't check whether speaker is inside room"`
	SkipAddSpeakerWall     bool   `name:"skip-add-speaker-wall" help:"don't add a wall for the speaker to be flushmounted in"`
}

func (c SimulateCmd) Run() error {
	room, err := goroom.NewFrom3MF(c.Mesh, map[string]goroom.Material{
		"default":                      BRICK,
		"Floor":                        WOOD,
		"Front A":                      GYPSUM,
		"Front B":                      GYPSUM,
		"Back Diffuser":                DIFFUSER,
		"Ceiling Absorber":             ROCKWOOL_24CM,
		"Secondary Ceiling Absorber L": ROCKWOOL_24CM,
		"Secondary Ceiling Absorber R": ROCKWOOL_24CM,
		"Street Absorber":              ROCKWOOL_24CM,
		"Front Hall Absorber":          ROCKWOOL_24CM,
		"Back Hall Absorber":           ROCKWOOL_24CM,
		"Cutout Top":                   ROCKWOOL_24CM,
		"Door":                         ROCKWOOL_12CM,
		"L Speaker Gap":                ROCKWOOL_24CM,
		"R Speaker Gap":                ROCKWOOL_24CM,
		"Window A":                     GLASS,
		"Window B":                     GLASS,
		"left speaker wall":            GYPSUM,
		"right speaker wall":           GYPSUM,
	})
	if err != nil {
		return err
	}

	RFZ_RADIUS := 0.5
	lt := goroom.ListeningTriangle{
		ReferencePosition: goroom.V(0, 2.37, 0.0),
		ReferenceNormal:   goroom.V(1, 0, 0),
		DistFromFront:     0.516,
		DistFromCenter:    1.352,
		// SourceHeight:      1.86,
		SourceHeight: 1.7,
		ListenHeight: 1.4,
	}

	mum8Spec := goroom.LoudSpeakerSpec{
		Xdim:        0.38,
		Ydim:        0.256,
		Zdim:        0.52,
		Yoff:        0.096,
		Zoff:        0.412,
		Directivity: lp8_directivity,
	}

	sources := []goroom.Speaker{
		goroom.NewSpeaker(mum8Spec, lt.LeftSourcePosition(), lt.LeftSourceNormal()),
		goroom.NewSpeaker(mum8Spec, lt.RightSourcePosition(), lt.RightSourceNormal()),
	}

	arrivals := []goroom.Arrival{}

	if !c.SkipSpeakerInRoomCheck {
		for i, source := range sources {
			offendingVertex, intersectingPoint, ok := source.IsInsideRoom(room.M, lt.ListenPosition())
			if !ok {
				room.M.SaveSTL("room.stl")
				p1 := goroom.Point{
					Position: offendingVertex,
					Color:    goroom.PastelRed,
					Name:     fmt.Sprint("source_%d", i),
				}
				p2 := goroom.Point{
					Position: intersectingPoint,
					Color:    goroom.PastelRed,
					Name:     fmt.Sprint("source_%d_bad_intersection", i),
				}
				if err := goroom.SavePointsArrivalsZonesToJSON("annotations.json", []goroom.Point{p1, p2}, []goroom.PsalmPath{
					{
						Points:    []goroom.Point{p1, p2},
						Name:      "",
						Color:     goroom.PastelRed,
						Thickness: 0,
					},
				}, nil, nil); err != nil {
					return err
				}
				return fmt.Errorf("ERROR: speaker does not fit in room")
			}
		}
	}

	if !c.SkipAddSpeakerWall {
		room.AddWall(lt.LeftSourcePosition(), lt.LeftSourceNormal())
		room.AddWall(lt.RightSourcePosition(), lt.RightSourceNormal())
	}

	for _, source := range sources {
		for _, shot := range source.Sample(50_000, 180, 180) {
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

	if err := goroom.SavePointsArrivalsZonesToJSON("annotations.json", nil, nil, arrivals, []goroom.Zone{{
		Center: lt.ListenPosition(),
		Radius: RFZ_RADIUS,
	}}); err != nil {
		return err
	}
	return nil
}

func main() {
	ctx := kong.Parse(&CLI)
	err := ctx.Run()
	if err != nil {
		log.Fatal(err)
	}
}

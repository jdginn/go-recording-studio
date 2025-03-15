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
	roomConfig "github.com/jdginn/go-recording-studio/room/config"
	roomExperiment "github.com/jdginn/go-recording-studio/room/experiment"
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

// var kh310_directivity = goroom.NewDirectivity(
// 	map[float64]float64{0: 0, 30: -0, 40: -1, 50: -2, 60: -3, 70: -4, 80: -6, 90: -7, 100: -8, 120: -11, 150: -20, 160: -50},
// 	map[float64]float64{0: 0, 30: -4, 60: -6, 70: -10, 80: -12, 100: -13, 120: -15},
// )

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
	Config                 string `arg:"" name:"config" help:"config file to simulate"`
	SkipSpeakerInRoomCheck bool   `name:"skip-speaker-in-room-check" help:"don't check whether speaker is inside room"`
	SkipAddSpeakerWall     bool   `name:"skip-add-speaker-wall" help:"don't add a wall for the speaker to be flushmounted in"`
}

func (c SimulateCmd) Run() error {
	config, err := roomConfig.LoadFromFile(c.Config, roomConfig.LoadOptions{
		ValidateImmediately: true,
		ResolvePaths:        true,
		MergeFiles:          true,
	})
	if err != nil {
		return err
	}

	expDir, err := roomExperiment.CreateExperimentDirectory()
	if err != nil {
		return fmt.Errorf("creating experiment directory: %w", err)
	}
	if err := expDir.CopyConfigFile(c.Config); err != nil {
		return fmt.Errorf("copying config file: %w", err)
	}

	room, err := goroom.NewFrom3MF(config.Input.Mesh.Path, config.SurfaceAssignmentMap())
	if err != nil {
		return err
	}

	lt := config.ListeningTriangle.Create()

	fmt.Printf("Listening position is %fm from front wall.\n\n", lt.ListenPosition().X)

	speakerSpec := config.Speaker.Create()

	sources := []goroom.Speaker{
		goroom.NewSpeaker(speakerSpec, lt.LeftSourcePosition(), lt.LeftSourceNormal()),
		goroom.NewSpeaker(speakerSpec, lt.RightSourcePosition(), lt.RightSourceNormal()),
	}

	arrivals := []goroom.Arrival{}

	if !c.SkipSpeakerInRoomCheck {
		for i, source := range sources {
			offendingVertex, intersectingPoint, ok := source.IsInsideRoom(room.M, lt.ListenPosition())
			if !ok {
				room.M.SaveSTL(expDir.GetFilePath("room.stl"))
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
		room.AddWall(lt.LeftSourcePosition(), lt.LeftSourceNormal(), "Left Speaker Wall", config.GetSurfaceAssignment("Left Speaker Wall"))
		room.AddWall(lt.RightSourcePosition(), lt.RightSourceNormal(), "Right Speaker Wall", config.GetSurfaceAssignment("Right Speaker Wall"))
	}

	for _, source := range sources {
		for _, shot := range source.Sample(config.Simulation.ShotCount, config.Simulation.ShotAngleRange, config.Simulation.ShotAngleRange) {
			arrival, err := room.TraceShot(shot, lt.ListenPosition(), goroom.TraceParams{
				Order:         config.Simulation.Order,
				GainThreshold: config.Simulation.GainThresholdDB,
				TimeThreshold: config.Simulation.TimeThresholdMS * MS,
				RFZRadius:     config.Simulation.RFZRadius,
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

	fmt.Printf("ITD: %fms\n", arrivals[0].ITD())

	room.M.SaveSTL(expDir.GetFilePath("room.stl"))

	if err := goroom.SavePointsArrivalsZonesToJSON(expDir.GetFilePath("annotations.json"), nil, nil, arrivals, []goroom.Zone{{
		Center: lt.ListenPosition(),
		Radius: config.Simulation.RFZRadius,
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

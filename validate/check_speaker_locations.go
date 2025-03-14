package main

import (
	"fmt"
	"log"

	"github.com/alecthomas/kong"

	goroom "github.com/jdginn/go-recording-studio/room"
	roomConfig "github.com/jdginn/go-recording-studio/room/config"
	roomExperiment "github.com/jdginn/go-recording-studio/room/experiment"
)

type ValidateCmd struct {
	Config string `arg:"" name:"config" help:"config file to simulate"`
}

func (c ValidateCmd) Run() error {
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

	speakerSpec := config.Speaker.Create()

	sources := []goroom.Speaker{
		goroom.NewSpeaker(speakerSpec, lt.LeftSourcePosition(), lt.LeftSourceNormal()),
		goroom.NewSpeaker(speakerSpec, lt.RightSourcePosition(), lt.RightSourceNormal()),
	}

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
	return nil
}

func main() {
	ctx := kong.Parse(&CLI)
	err := ctx.Run()
	if err != nil {
		log.Fatal(err)
	}
}

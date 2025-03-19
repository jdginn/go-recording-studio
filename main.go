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

func addCeilingAbsorbers(r *goroom.Room, lt goroom.ListeningTriangle, config roomConfig.ExperimentConfig) error {
	center := config.CeilingPanels.Center

	if center != nil {
		fmt.Printf("%+v\n", center)
		r.AddPrism(
			goroom.Bounds{
				Min: center.XMin,
				Max: center.XMax,
			},
			goroom.Bounds{
				Min: lt.ReferencePosition.Y - center.Width/2,
				Max: lt.ReferencePosition.Y + center.Width/2,
			},
			goroom.Bounds{
				Min: center.Height,
				Max: center.Height + center.Thickness,
			},
			"Center Ceiling Absorber",
			goroom.Material{Alpha: 0.9999},
			// config.GetSurfaceAssignment("Center Ceiling Absorber"),
		)
	}

	sides := config.CeilingPanels.Sides
	if sides != nil {
		fmt.Printf("%+v\n", sides)
		r.AddPrism(
			goroom.Bounds{
				Min: sides.XMin,
				Max: sides.XMax,
			},
			goroom.Bounds{
				Min: lt.ReferencePosition.Y - sides.Spacing/2 - sides.Width/2,
				Max: lt.ReferencePosition.Y - sides.Spacing/2 + sides.Width/2,
			},
			goroom.Bounds{
				Min: sides.Height,
				Max: sides.Height + center.Thickness,
			},
			"Left Ceiling Absorber",
			goroom.Material{Alpha: 0.999},
			// config.GetSurfaceAssignment("Left Ceiling Absorber"),
		)
		r.AddPrism(
			goroom.Bounds{
				Min: sides.XMin,
				Max: sides.XMax,
			},
			goroom.Bounds{
				Min: lt.ReferencePosition.Y + sides.Spacing/2 - sides.Width/2,
				Max: lt.ReferencePosition.Y + sides.Spacing/2 + sides.Width/2,
			},
			goroom.Bounds{
				Min: sides.Height,
				Max: sides.Height + center.Thickness,
			},
			"Right Ceiling Absorber",
			goroom.Material{Alpha: 0.999},
			// config.GetSurfaceAssignment("Right Ceiling Absorber"),
		)
	}

	return nil
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

	room, surfaces, err := goroom.NewFrom3MF(config.Input.Mesh.Path, config.SurfaceAssignmentMap())
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

	if !(c.SkipSpeakerInRoomCheck || config.Flags.SkipSpeakerInRoomCheck) {
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
				if err := goroom.SaveAnnotationsToJson("annotations.json", []goroom.Point{p1, p2}, []goroom.PsalmPath{
					{
						Points:    []goroom.Point{p1, p2},
						Name:      "",
						Color:     goroom.PastelRed,
						Thickness: 0,
					},
				}, nil, nil); err != nil {
					return err
				}

				goroom.SaveResultsSummaryToJSON(expDir.GetFilePath("summary.json"), goroom.ResultsSummary{
					Status:  "validation_error",
					Errors:  []string{"speaker_outside_room"},
					Results: goroom.AnalysisResults{},
				})
			}
		}
	}

	if !(c.SkipAddSpeakerWall || config.Flags.SkipAddSpeakerWall) {
		room.AddWall(lt.LeftSourcePosition(), lt.LeftSourceNormal(), "Left Speaker Wall", config.GetSurfaceAssignment("Left Speaker Wall"))
		room.AddWall(lt.RightSourcePosition(), lt.RightSourceNormal(), "Right Speaker Wall", config.GetSurfaceAssignment("Right Speaker Wall"))
	}

	addCeilingAbsorbers(room, lt, *config)

	for surface := range surfaces {
		fmt.Println(surface)
	}

	absorbers := []string{
		"Hall B",
		"Street A",
		// "Cutout Side",
		"Door Side A",
		"Hall E",
		"Street D",
		"Street B",
		"Door Side B",
		"Entry Back",
		"Street C",
		"Street E",
		"Hall A",
		"Entry Front",
		"Door",
		"Back A",
		"Back B",
	}

	for _, name := range absorbers {
		room.AddSurface(surfaces[name].Absorber(0.14, 1.5, goroom.Material{
			Alpha: 0.999,
		}))
	}

	totalShots := 0
	for _, source := range sources {
		for _, shot := range source.Sample(config.Simulation.ShotCount, config.Simulation.ShotAngleRange, config.Simulation.ShotAngleRange) {
			totalShots += 1
			arrival, err := room.TraceShot(shot, lt.ListenPosition(), goroom.TraceParams{
				Order:         config.Simulation.Order,
				GainThreshold: config.Simulation.GainThresholdDB,
				TimeThreshold: config.Simulation.TimeThresholdMS * MS,
				RFZRadius:     config.Simulation.RFZRadius,
			})
			if err != nil {
				if saveErr := goroom.SaveResultsSummaryToJSON(expDir.GetFilePath("summary.json"), goroom.ResultsSummary{
					Status: "simulation_error",
					Errors: []string{err.Error()},
					Results: goroom.AnalysisResults{
						ListenPosDist: lt.ListenPosition().X, // TODO: technically, this is an unsafe assumption since the room is not guaranteed to always be oriented along he X axis
					},
				}); saveErr != nil {
					return err
				}
				return err
			}
			arrivals = append(arrivals, arrival...)
		}
	}

	sort.Slice(arrivals, func(i int, j int) bool {
		return arrivals[i].Distance < arrivals[j].Distance
	})

	energyOverWindow, err := goroom.EnergyOverWindow(arrivals, 25, -15)
	if err != nil {
		print(err)
	}

	room.M.SaveSTL(expDir.GetFilePath("room.stl"))

	if err := goroom.SaveResultsSummaryToJSON(expDir.GetFilePath("summary.json"), goroom.ResultsSummary{
		Status: "success",
		Results: goroom.AnalysisResults{
			ITD:              arrivals[0].ITD(),
			EnergyOverWindow: energyOverWindow / float64(totalShots),
			ListenPosDist:    lt.ListenPosition().X, // TODO: technically, this is an unsafe assumption since the room is not guaranteed to always be oriented along he X axis
		},
	}); err != nil {
		fmt.Println(err)
	}

	if err := goroom.SaveAnnotationsToJson(expDir.GetFilePath("annotations.json"), nil, nil, arrivals, []goroom.Zone{{
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

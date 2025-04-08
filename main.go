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
	Simulate SimulateCmd `cmd:"" help:"Simulate a room"`
}

type SimulateCmd struct {
	Config                 string `arg:"" name:"config" help:"config file to simulate"`
	OutputDir              string `arg:"" optional:"" name:"output-dir" help:"directory to store output in"`
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

	var expDir *roomExperiment.ExperimentDir
	if c.OutputDir != "" {
		expDir, err = roomExperiment.UseExistingExperimentDirectory(c.OutputDir)
	} else {
		expDir, err = roomExperiment.CreateExperimentDirectory("experiments")
	}
	if err := expDir.CopyConfigFile(c.Config); err != nil {
		return fmt.Errorf("copying config file: %w", err)
	}

	room, surfaces, err := goroom.NewFrom3MF(config.Input.Mesh.Path, config.SurfaceAssignmentMap())
	if err != nil {
		return err
	}

	lt := config.ListeningTriangle.Create()
	listenPos, equilateralPos := lt.ListenPosition()

	speakerSpec := config.Speaker.Create()

	sources := []goroom.Speaker{
		goroom.NewSpeaker(speakerSpec, lt.LeftSourcePosition(), lt.LeftSourceNormal()),
		goroom.NewSpeaker(speakerSpec, lt.RightSourcePosition(), lt.RightSourceNormal()),
	}

	lDirectPath := goroom.PsalmPath{Points: []goroom.Point{{Position: lt.LeftSourcePosition()}, {Position: equilateralPos}}, Color: goroom.BrightRed}
	rDirectPath := goroom.PsalmPath{Points: []goroom.Point{{Position: lt.RightSourcePosition()}, {Position: equilateralPos}}, Color: goroom.BrightRed}

	paths := []goroom.PsalmPath{lDirectPath, rDirectPath}

	lSpeakerCone, err := room.GetSpeakerCone(sources[0], 30, 16, goroom.PastelGreen)
	rSpeakerCone, err := room.GetSpeakerCone(sources[1], 30, 16, goroom.PastelLavender)
	paths = append(paths, append(lSpeakerCone, rSpeakerCone...)...)

	arrivals := []goroom.Arrival{}

	if !(c.SkipSpeakerInRoomCheck || config.Flags.SkipSpeakerInRoomCheck) {
		for i, source := range sources {
			offendingVertex, intersectingPoint, ok := source.IsInsideRoom(room.M, listenPos)
			if !ok {
				room.M.SaveSTL(expDir.GetFilePath("room.stl"))
				p1 := goroom.Point{
					Position: offendingVertex,
					Color:    goroom.PastelRed,
					Name:     fmt.Sprintf("source_%d", i),
				}
				p2 := goroom.Point{
					Position: intersectingPoint,
					Color:    goroom.PastelRed,
					Name:     fmt.Sprintf("source_%d_bad_intersection", i),
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
				return nil
			}
		}
	}

	if !(c.SkipAddSpeakerWall || config.Flags.SkipAddSpeakerWall) {
		room.AddWall(lt.LeftSourcePosition(), lt.LeftSourceNormal(), "Left Speaker Wall", config.GetSurfaceAssignment("Left Speaker Wall"))
		room.AddWall(lt.RightSourcePosition(), lt.RightSourceNormal(), "Right Speaker Wall", config.GetSurfaceAssignment("Right Speaker Wall"))
	}

	addCeilingAbsorbers(room, lt, *config)

	for name, height := range config.WallAbsorbers.Heights {
		room.AddSurface(surfaces[name].Absorber(config.WallAbsorbers.Thickness, height, goroom.Material{
			Alpha: 0.9,
		}))
	}

	totalShots := 0
	for _, source := range sources {
		for _, shot := range source.Sample(config.Simulation.ShotCount, config.Simulation.ShotAngleRange, config.Simulation.ShotAngleRange) {
			totalShots += 1
			arrival, err := room.TraceShot(shot, listenPos, goroom.TraceParams{
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
						ListenPosDist: listenPos.X, // TODO: technically, this is an unsafe assumption since the room is not guaranteed to always be oriented along he X axis
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

	var ITD float64
	if len(arrivals) > 0 {
		ITD = arrivals[0].ITD()
	} else {
		ITD = config.Simulation.TimeThresholdMS
	}
	energyOverWindow, err := goroom.EnergyOverWindow(arrivals, 25, -15)
	if err != nil {
		print(err)
	}

	room.M.SaveSTL(expDir.GetFilePath("room.stl"))

	if err := goroom.SaveResultsSummaryToJSON(expDir.GetFilePath("summary.json"), goroom.ResultsSummary{
		Status: "success",
		Results: goroom.AnalysisResults{
			ITD:              ITD,
			EnergyOverWindow: energyOverWindow / float64(totalShots),
			ListenPosDist:    listenPos.X, // TODO: technically, this is an unsafe assumption since the room is not guaranteed to always be oriented along he X axis
		},
	}); err != nil {
		fmt.Println(err)
	}

	if err := goroom.SaveAnnotationsToJson(expDir.GetFilePath("annotations.json"), nil, paths, arrivals, []goroom.Zone{{
		Center: listenPos,
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

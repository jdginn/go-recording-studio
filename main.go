package main

import (
	"fmt"
	"log"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/fogleman/pt/pt"

	goroom "github.com/jdginn/go-recording-studio/room"
	roomConfig "github.com/jdginn/go-recording-studio/room/config"
	roomExperiment "github.com/jdginn/go-recording-studio/room/experiment"
)

const MS float64 = 1.0 / 1000.0

const SCALE float64 = 100

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
			config.GetSurfaceAssignment("Center Ceiling Absorber"),
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
			config.GetSurfaceAssignment("Left Ceiling Absorber"),
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
			config.GetSurfaceAssignment("Right Ceiling Absorber"),
		)
	}

	return nil
}

var CLI struct {
	Simulate SimulateCmd `cmd:"" help:"Simulate a room"`
	Trace    TraceCmd    `cmd:"" help:"Trace to a specific position"`
}

type SimulateCmd struct {
	Config                 string   `arg:"" name:"config" help:"config file to simulate"`
	OutputDir              string   `arg:"" optional:"" name:"output-dir" help:"directory to store output in"`
	SkipSpeakerInRoomCheck bool     `name:"skip-speaker-in-room-check" help:"don't check whether speaker is inside room"`
	SkipAddSpeakerWall     bool     `name:"skip-add-speaker-wall" help:"don't add a wall for the speaker to be flushmounted in"`
	SkipTracing            bool     `name:"skip-tracing" help:"don't perform any of the tracing steps"`
	SimulateLSpeaker       bool     `name:"simulate-lspeaker" help:"simulate the left speaker"`
	SimulateRSpeaker       bool     `name:"simulate-rspeaker" help:"simulate the right speaker"`
	Counterfactual         []string `name:"counterfactual" help:"simulate reflections that *would* arrive at the listening position if the given surface were a perfect reflector"`
	OverrideListenPosition string   `name:"override-listen-position" help:"override the listening position with a specific x,y,z position (in m from origin)"`
	OverrideRFZRadius      float64  `name:"override-rfz-radius" help:"override the radius of the reflection-free zone around the listening position (in m)" default:"0"`
}

func (c SimulateCmd) Run() (err error) {
	config, err := roomConfig.LoadFromFile(c.Config, roomConfig.LoadOptions{
		ValidateImmediately: true,
		ResolvePaths:        true,
		MergeFiles:          true,
	})
	if err != nil {
		return err
	}

	// Create a directory to store the results of this experiment
	var expDir *roomExperiment.ExperimentDir
	if c.OutputDir != "" {
		expDir, err = roomExperiment.UseExistingExperimentDirectory(c.OutputDir)
	} else {
		expDir, err = roomExperiment.CreateExperimentDirectory("experiments")
	}
	if err := expDir.CopyConfigFile(c.Config); err != nil {
		return fmt.Errorf("copying config file: %w", err)
	}

	// Initialize output data structures
	//
	// We will accumulate results into these structures as we compute them
	annotations := goroom.NewAnnotations()
	summary := goroom.NewSummary()

	room, _, err := goroom.NewFrom3MF(config.Input.Mesh.Path, config.SurfaceAssignmentMap())
	if err != nil {
		return err
	}

	// Whatever happens from here on out, write results to experiment directory
	defer func() {
		room.M.SaveSTL(expDir.GetFilePath("room.stl"))
		if saveErr := summary.WriteToJSON(expDir.GetFilePath("summary.json")); saveErr != nil {
			err = fmt.Errorf("Error saving summary: %w", saveErr)
		}
		if saveErr := annotations.WriteToJSON(expDir.GetFilePath("annotations.json")); saveErr != nil {
			err = fmt.Errorf("Error writing annotations: %w", saveErr)
		}
	}()

	volume, err := room.Volume()
	summary.Results.Volume = volume
	// Calculate decay characteristics that can be known without the listening position
	t60Sabine, err := room.T60Sabine(150)
	if err != nil {
		return err
	}
	summary.Results.T60Sabine = t60Sabine
	t60Eyering, err := room.T60Eyring(150)
	if err != nil {
		return err
	}
	summary.Results.T60Eyering = t60Eyering
	schroederFreq, err := room.SchroederFreq()
	if err != nil {
		return err
	}
	summary.Results.SchroederFreq = schroederFreq

	// Compute the position of the speakers as well as the listening position
	lt := config.ListeningTriangle.Create()
	listenPos, equilateralPos := lt.ListenPosition()
	speakerNormalListenPos := listenPos
	if c.OverrideListenPosition != "" {
		overridePos, err := parseTargetPosition(c.OverrideListenPosition)
		if err != nil {
			return fmt.Errorf("parsing override listen position: %w", err)
		}
		listenPos = overridePos
		fmt.Println("Overriding listen position to ", listenPos)
	}
	radius := config.Simulation.RFZRadius
	if c.OverrideRFZRadius != 0 {
		radius = c.OverrideRFZRadius
		fmt.Println("Overriding RFZ radius to ", radius)
	}
	summary.Results.ListenPosX = listenPos.X // TODO: technically, this is an unsafe assumption since the room is not guaranteed to always be oriented along the X axis
	annotations.Zones = append(annotations.Zones, goroom.Zone{
		Center: listenPos,
		Radius: radius,
	})

	// Create the speakers
	speakerSpec := config.Speaker.Create()
	sources := []goroom.Speaker{}
	paths := []goroom.PsalmPath{}

	if c.SimulateLSpeaker || config.Flags.SimulateLSpeaker {
		lSource := goroom.NewSpeaker(speakerSpec, lt.LeftSourcePosition(), lt.LeftSourceNormal(), "Left")
		sources = append(sources, lSource)
		paths = append(paths, goroom.PsalmPath{Points: []goroom.Point{{Position: lt.LeftSourcePosition()}, {Position: equilateralPos}}, Color: goroom.BrightRed})
		lSpeakerCone, err := room.GetSpeakerCone(lSource, 30, 16, goroom.PastelGreen)
		if err != nil {
			return err
		}
		paths = append(paths, lSpeakerCone...)
	}
	if c.SimulateRSpeaker || config.Flags.SimulateRSpeaker {
		rSource := goroom.NewSpeaker(speakerSpec, lt.RightSourcePosition(), lt.RightSourceNormal(), "Right")
		sources = append(sources, rSource)
		paths = append(paths, goroom.PsalmPath{Points: []goroom.Point{{Position: lt.RightSourcePosition()}, {Position: equilateralPos}}, Color: goroom.BrightRed})
		rSpeakerCone, err := room.GetSpeakerCone(rSource, 30, 16, goroom.PastelLavender)
		if err != nil {
			return err
		}
		paths = append(paths, rSpeakerCone...)
	}

	if !(c.SkipSpeakerInRoomCheck || config.Flags.SkipSpeakerInRoomCheck) {
		// Check whether the speakers are inside the room
		for i, source := range sources {
			offendingVertex, intersectingPoint, ok := source.IsInsideRoom(room.M, speakerNormalListenPos)
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
				annotations.Points = append(annotations.Points, p1, p2)
				annotations.Paths = append(annotations.Paths,
					goroom.PsalmPath{
						Points:    []goroom.Point{p1, p2},
						Name:      "",
						Color:     goroom.PastelRed,
						Thickness: 0,
					})
				if saveErr := annotations.WriteToJSON("annotations.json"); err != nil {
					return fmt.Errorf("Error writing annotations: %w\n\ttaken after validation error %w", saveErr, err)
				}
				summary.AddError(goroom.ErrValidation, fmt.Errorf("speaker_outside_room"))
				// Keep in mind, the defer will still write summary and annotations to the experiment directory
				return nil
			}
		}
	}

	if !(c.SkipAddSpeakerWall || config.Flags.SkipAddSpeakerWall) {
		room.AddWall(lt.LeftSourcePosition(), lt.LeftSourceNormal(), "Left Speaker Wall", config.GetSurfaceAssignment("Left Speaker Wall"))
		room.AddWall(lt.RightSourcePosition(), lt.RightSourceNormal(), "Right Speaker Wall", config.GetSurfaceAssignment("Right Speaker Wall"))
	}

	// TODO: remove this hard-code-y hack and replace with something more principled
	// addCeilingAbsorbers(room, lt, *config)

	if !c.SkipTracing {

		var totalShots int
		// Simulate reflections
		arrivals := []goroom.Arrival{}
		for _, source := range sources {
			for _, shot := range source.Sample(config.Simulation.ShotCount, config.Simulation.ShotAngleRange, config.Simulation.ShotAngleRange) {
				totalShots += 1
				arrival, err := room.TraceShot(shot, listenPos, goroom.TraceParams{
					Order:         config.Simulation.Order,
					GainThreshold: config.Simulation.GainThresholdDB,
					TimeThreshold: config.Simulation.TimeThresholdMS * MS,
					RFZRadius:     radius,
				})
				if err != nil {
					summary.AddError(goroom.ErrSimulation, err)
					return err
				}
				arrivals = append(arrivals, arrival...)
			}
		}
		summary.Successful()
		sort.Slice(arrivals, func(i int, j int) bool {
			return arrivals[i].Distance < arrivals[j].Distance
		})
		summary.Results.ITD = arrivals[0].ITD()

		if len(c.Counterfactual) == 0 {
			annotations.Arrivals = arrivals
		}

		if len(c.Counterfactual) > 0 {
			reflectsOffSurface := func(arr goroom.Arrival, surfaces []string) bool {
				for _, ref := range arr.AllReflections {
					if ref.Surface.Name == "Diffuser Hall" {
						fmt.Printf("Surface: %s\n\tCounterfactual: %v\n", ref.Surface.Name, c.Counterfactual)
					}
					if slices.Contains(surfaces, ref.Surface.Name) {
						fmt.Println("Contains")
						return true
					}
				}
				return false
			}
			surfaceMap := config.SurfaceAssignmentMap()
			for _, c := range c.Counterfactual {
				fmt.Printf("Making %s a perfect absorber\n", c)
				surfaceMap[c] = goroom.PerfectAbsorber()
			}
			cfRoom, _, err := goroom.NewFrom3MF(config.Input.Mesh.Path, surfaceMap)
			if err != nil {
				return err
			}
			// Simulate reflections AS IF the counterfactual surface were a perfect absorber
			cfArrivals := []goroom.Arrival{}
			for _, source := range sources {
				for _, shot := range source.Sample(config.Simulation.ShotCount, config.Simulation.ShotAngleRange, config.Simulation.ShotAngleRange) {
					arrival, err := cfRoom.TraceShot(shot, listenPos, goroom.TraceParams{
						Order:         config.Simulation.Order,
						GainThreshold: config.Simulation.GainThresholdDB,
						TimeThreshold: config.Simulation.TimeThresholdMS * MS,
						RFZRadius:     radius,
					})
					if err != nil {
						summary.AddError(goroom.ErrSimulation, err)
						return err
					}
					for _, a := range arrival {
						if a.Distance != goroom.INF {
							if reflectsOffSurface(a, c.Counterfactual) {
								cfArrivals = append(cfArrivals, a)
							}
						}
					}
				}
			}
			sort.Slice(cfArrivals, func(i int, j int) bool {
				return cfArrivals[i].Distance < cfArrivals[j].Distance
			})
			summary.Successful()

			// filteredArrivals := []goroom.Arrival{}
			filteredArrivals := arrivals
			// for _, arr := range arrivals {
			// 	if reflectsOffSurface(arr, c.Counterfactual) {
			// 		filteredArrivals = append(filteredArrivals, arr)
			// 	}
			// }

			cfUniqueArrivals := []goroom.Arrival{}
		outer:
			for _, fArr := range filteredArrivals {
				for _, cfArr := range cfArrivals {
					if cfArr.Shot.Equal(fArr.Shot) {
						continue outer
					}
				}
				cfUniqueArrivals = append(cfUniqueArrivals, fArr)
				for _, ref := range fArr.AllReflections {
					if slices.Contains(c.Counterfactual, ref.Surface.Name) {
						annotations.Points = append(annotations.Points, goroom.Point{
							Position: ref.Position,
							Name:     "",
							Color:    goroom.PastelGreen,
						})
					}
				}
			}
			fmt.Println("Total arrivals: ", len(arrivals))
			fmt.Println("Total filtered arrivals: ", len(filteredArrivals))
			fmt.Println("Total counterfactual arrivals: ", len(cfArrivals))
			fmt.Println("Total unique counterfactual arrivals: ", len(cfUniqueArrivals))
			// for i, arr := range filteredArrivals {
			// 	annotations.Arrivals = append(annotations.Arrivals, arr)
			// 	annotations.PathColors[i] = goroom.PastelLavender
			// }
			annotations.Arrivals = append(annotations.Arrivals, cfUniqueArrivals...)
		}
		annotations.Arrivals = arrivals

		var ITD float64
		if len(arrivals) > 0 {
			ITD = arrivals[0].ITD()
		} else {
			ITD = config.Simulation.TimeThresholdMS
		}
		summary.Results.ITD = ITD
		energyOverWindow, err := goroom.EnergyOverWindow(arrivals, 25, -15)
		if err != nil {
			print(err)
		}
		summary.Results.EnergyOverWindow = energyOverWindow / float64(totalShots)
	}

	// Write output to experiment directory
	room.M.SaveSTL(expDir.GetFilePath("room.stl"))
	if saveErr := summary.WriteToJSON(expDir.GetFilePath("summary.json")); saveErr != nil {
		return fmt.Errorf("Error saving summary: %w", saveErr)
	}
	if saveErr := annotations.WriteToJSON(expDir.GetFilePath("annotations.json")); saveErr != nil {
		return fmt.Errorf("Error writing annotations: %w", saveErr)
	}

	return nil
}

func parseTargetPosition(pos string) (pt.Vector, error) {
	parts := strings.Split(pos, ",")
	if len(parts) != 3 {
		return pt.Vector{
			X: 0,
			Y: 0,
			Z: 0,
		}, fmt.Errorf("expected 3 comma-separated values, got %d", len(parts))
	}
	x, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return pt.Vector{
			X: 0,
			Y: 0,
			Z: 0,
		}, fmt.Errorf("failed to parse x: %w", err)
	}
	y, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return pt.Vector{
			X: 0,
			Y: 0,
			Z: 0,
		}, fmt.Errorf("failed to parse y: %w", err)
	}
	z, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	if err != nil {
		return pt.Vector{
			X: 0,
			Y: 0,
			Z: 0,
		}, fmt.Errorf("failed to parse z: %w", err)
	}
	return pt.Vector{
		X: x,
		Y: y,
		Z: z,
	}, nil
}

type TraceCmd struct {
	Config                 string  `arg:"" name:"config" help:"config file to simulate"`
	OutputDir              string  `arg:"" optional:"" name:"output-dir" help:"directory to store output in"`
	TargetPosition         string  `name:"target-position" help:"position to trace to, in x,y,z format (in m from origin)"`
	Spread                 float64 `name:"spread" help:"angular spread of shots around target normal, in degrees" default:"1"`
	ShotCount              int     `name:"shot-count" help:"number of shots to simulate per source" default:"1"`
	SkipSpeakerInRoomCheck bool    `name:"skip-speaker-in-room-check" help:"don't check whether speaker is inside room"`
	SkipAddSpeakerWall     bool    `name:"skip-add-speaker-wall" help:"don't add a wall for the speaker to be flushmounted in"`
	SimulateLSpeaker       bool    `name:"simulate-lspeaker" help:"simulate the left speaker"`
	SimulateRSpeaker       bool    `name:"simulate-rspeaker" help:"simulate the right speaker"`
}

func (c TraceCmd) Run() (err error) {
	config, err := roomConfig.LoadFromFile(c.Config, roomConfig.LoadOptions{
		ValidateImmediately: true,
		ResolvePaths:        true,
		MergeFiles:          true,
	})
	if err != nil {
		return err
	}

	// Create a directory to store the results of this experiment
	var expDir *roomExperiment.ExperimentDir
	if c.OutputDir != "" {
		expDir, err = roomExperiment.UseExistingExperimentDirectory(c.OutputDir)
	} else {
		expDir, err = roomExperiment.CreateExperimentDirectory("experiments")
	}
	if err := expDir.CopyConfigFile(c.Config); err != nil {
		return fmt.Errorf("copying config file: %w", err)
	}

	// Initialize output data structures
	//
	// We will accumulate results into these structures as we compute them
	annotations := goroom.NewAnnotations()
	summary := goroom.NewSummary()

	room, _, err := goroom.NewFrom3MF(config.Input.Mesh.Path, config.SurfaceAssignmentMap())
	if err != nil {
		return err
	}

	// Whatever happens from here on out, write results to experiment directory
	defer func() {
		room.M.SaveSTL(expDir.GetFilePath("room.stl"))
		if saveErr := summary.WriteToJSON(expDir.GetFilePath("summary.json")); saveErr != nil {
			err = fmt.Errorf("Error saving summary: %w", saveErr)
		}
		if saveErr := annotations.WriteToJSON(expDir.GetFilePath("annotations.json")); saveErr != nil {
			err = fmt.Errorf("Error writing annotations: %w", saveErr)
		}
	}()

	// Calculate volume
	volume, err := room.Volume()
	summary.Results.Volume = volume
	// Calculate decay characteristics that can be known without the listening position
	t60Sabine, err := room.T60Sabine(150)
	if err != nil {
		return err
	}
	summary.Results.T60Sabine = t60Sabine
	t60Eyering, err := room.T60Eyring(150)
	if err != nil {
		return err
	}
	summary.Results.T60Eyering = t60Eyering
	schroederFreq, err := room.SchroederFreq()
	if err != nil {
		return err
	}
	summary.Results.SchroederFreq = schroederFreq

	// Compute the position of the speakers as well as the listening position
	lt := config.ListeningTriangle.Create()
	listenPos, equilateralPos := lt.ListenPosition()
	summary.Results.ListenPosX = listenPos.X // TODO: technically, this is an unsafe assumption since the room is not guaranteed to always be oriented along the X axis
	annotations.Zones = append(annotations.Zones, goroom.Zone{
		Center: listenPos,
		Radius: config.Simulation.RFZRadius,
	})

	// Create the speakers
	speakerSpec := config.Speaker.Create()
	sources := []goroom.Speaker{}
	paths := []goroom.PsalmPath{}

	if c.SimulateLSpeaker || config.Flags.SimulateLSpeaker {
		lSource := goroom.NewSpeaker(speakerSpec, lt.LeftSourcePosition(), lt.LeftSourceNormal(), "Left")
		sources = append(sources, lSource)
		paths = append(paths, goroom.PsalmPath{Points: []goroom.Point{{Position: lt.LeftSourcePosition()}, {Position: equilateralPos}}, Color: goroom.BrightRed})
		lSpeakerCone, err := room.GetSpeakerCone(lSource, 30, 16, goroom.PastelGreen)
		if err != nil {
			return err
		}
		paths = append(paths, lSpeakerCone...)
	}
	if c.SimulateRSpeaker || config.Flags.SimulateRSpeaker {
		rSource := goroom.NewSpeaker(speakerSpec, lt.RightSourcePosition(), lt.RightSourceNormal(), "Right")
		sources = append(sources, rSource)
		paths = append(paths, goroom.PsalmPath{Points: []goroom.Point{{Position: lt.RightSourcePosition()}, {Position: equilateralPos}}, Color: goroom.BrightRed})
		rSpeakerCone, err := room.GetSpeakerCone(rSource, 30, 16, goroom.PastelLavender)
		if err != nil {
			return err
		}
		paths = append(paths, rSpeakerCone...)
	}

	if !(c.SkipSpeakerInRoomCheck || config.Flags.SkipSpeakerInRoomCheck) {
		// Check whether the speakers are inside the room
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
				annotations.Points = append(annotations.Points, p1, p2)
				annotations.Paths = append(annotations.Paths,
					goroom.PsalmPath{
						Points:    []goroom.Point{p1, p2},
						Name:      "",
						Color:     goroom.PastelRed,
						Thickness: 0,
					})
				if saveErr := annotations.WriteToJSON("annotations.json"); err != nil {
					return fmt.Errorf("Error writing annotations: %w\n\ttaken after validation error %w", saveErr, err)
				}
				summary.AddError(goroom.ErrValidation, fmt.Errorf("speaker_outside_room"))
				// Keep in mind, the defer will still write summary and annotations to the experiment directory
				return nil
			}
		}
	}

	if !(c.SkipAddSpeakerWall || config.Flags.SkipAddSpeakerWall) {
		room.AddWall(lt.LeftSourcePosition(), lt.LeftSourceNormal(), "Left Speaker Wall", config.GetSurfaceAssignment("Left Speaker Wall"))
		room.AddWall(lt.RightSourcePosition(), lt.RightSourceNormal(), "Right Speaker Wall", config.GetSurfaceAssignment("Right Speaker Wall"))
	}

	// TODO: remove this hard-code-y hack and replace with something more principled
	// addCeilingAbsorbers(room, lt, *config)

	// Simulate reflections
	arrivals := []goroom.Arrival{}

	targetPos, err := parseTargetPosition(c.TargetPosition)
	if err != nil {
		return err
	}

	totalShots := 0
	for _, source := range sources {
		fmt.Println("Shooting from ", source.Position, " to ", targetPos)
		normal := targetPos.Sub(source.Position).Normalize()
		for _, shot := range source.SampleWithNormal(normal, c.ShotCount, c.Spread, c.Spread) {
			totalShots += 1
			arrival, err := room.TraceShotUnconditional(shot, listenPos, goroom.TraceParams{
				Order:         config.Simulation.Order,
				GainThreshold: config.Simulation.GainThresholdDB,
				TimeThreshold: config.Simulation.TimeThresholdMS * MS,
				RFZRadius:     config.Simulation.RFZRadius,
			})
			if err != nil {
				summary.AddError(goroom.ErrSimulation, err)
				return err
			}
			arrivals = append(arrivals, arrival...)
		}
	}
	summary.Successful()
	sort.Slice(arrivals, func(i int, j int) bool {
		return arrivals[i].Distance < arrivals[j].Distance
	})
	if len(arrivals) > 0 {
		summary.Results.ITD = arrivals[0].ITD()
	} else {
		summary.Results.ITD = config.Simulation.TimeThresholdMS
	}

	annotations.Arrivals = arrivals

	var ITD float64
	if len(arrivals) > 0 {
		ITD = arrivals[0].ITD()
	} else {
		ITD = config.Simulation.TimeThresholdMS
	}
	summary.Results.ITD = ITD
	energyOverWindow, err := goroom.EnergyOverWindow(arrivals, 25, -15)
	if err != nil {
		print(err)
	}
	summary.Results.EnergyOverWindow = energyOverWindow / float64(totalShots)

	// Write output to experiment directory
	room.M.SaveSTL(expDir.GetFilePath("room.stl"))
	if saveErr := summary.WriteToJSON(expDir.GetFilePath("summary.json")); saveErr != nil {
		return fmt.Errorf("Error saving summary: %w", saveErr)
	}
	if saveErr := annotations.WriteToJSON(expDir.GetFilePath("annotations.json")); saveErr != nil {
		return fmt.Errorf("Error writing annotations: %w", saveErr)
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

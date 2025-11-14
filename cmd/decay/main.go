package main

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/fogleman/pt/pt"

	goroom "github.com/jdginn/go-recording-studio/room"
	roomConfig "github.com/jdginn/go-recording-studio/room/config"
	roomExperiment "github.com/jdginn/go-recording-studio/room/experiment"
)

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

const MS float64 = 1.0 / 1000.0

const SCALE float64 = 100

var testFrequencies = []float64{
	63, 125, 250, 500, 1000, 2000, 4000, 8000, 16000,
}

var CLI struct {
	Simulate SimulateCmd `cmd:"" help:"Simulate T60 for a room"`
}

type SimulateCmd struct {
	Config       string  `arg:"" name:"config" help:"config file to simulate"`
	OutputDir    string  `arg:"" optional:"" name:"output-dir" help:"directory to store output in"`
	SourcePos    string  `name:"source-pos" help:"position of the sound source in the room" default:"0.5,0.5,0.5"`
	ListenerPos  string  `name:"listener-pos" help:"position of the listener in the room" default:"0.5,0.5,0.5"`
	ListenRadius float64 `name:"listen-radius" help:"radius around the listener position to consider" default:"0.1"`
	NumShots     int     `name:"num-shots" help:"number of ray shots to simulate" default:"100000"`
}

func (c SimulateCmd) Run() (err error) {
	config, err := roomConfig.LoadFromFile(c.Config, roomConfig.LoadOptions{
		ValidateImmediately: false,
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

	room, _, err := goroom.NewFrom3MF(config.Input.Mesh.Path, config.SurfaceAssignmentMap())
	if err != nil {
		return err
	}

	// Sabine/Eyering part

	vol, err := room.Volume()
	fmt.Printf("Room volume: %.2f m³\n\n", vol)

	schroeder, err := room.SchroederFreq()
	if err != nil {
		return err
	}
	fmt.Printf("Schroeder frequency: %.0f Hz\n\n", schroeder)

	t60s := make([]float64, len(testFrequencies))
	for i, freq := range testFrequencies {
		t60, err := room.T60Sabine(freq)
		if err != nil {
			return err
		}
		t60s[i] = t60
	}

	for i, t60 := range t60s {
		fmt.Printf("T60 at %.0f Hz: %.2f ms\n", testFrequencies[i], t60/MS)
	}

	// Ray tracing part
	sourcePos, err := parseTargetPosition(c.SourcePos)
	if err != nil {
		return err
	}
	listenPos, err := parseTargetPosition(c.ListenerPos)
	if err != nil {
		return err
	}
	source := goroom.Source{
		Position: sourcePos,
		// Assume omnidirectional sources for now, so normal direction doesn't matter
		NormalDirection: pt.Vector{1, 0, 0},
		Name:            "Source",
	}
	horizSteps := int(math.Floor(math.Sqrt(float64(c.NumShots))))
	vertSteps := c.NumShots / horizSteps

	shots := make([]goroom.Shot, 0, c.NumShots)
	for x := 0; x < horizSteps; x++ {
		yaw := -180 + 360*(float64(x)/float64(horizSteps))
		yawRads := yaw / 180 * math.Pi
		for y := 0; y < vertSteps; y++ {
			pitch := -180 + 360*(float64(y)/float64(vertSteps))
			pitchRads := pitch / 180 * math.Pi
			direction := source.NormalDirection.MulScalar(math.Cos(pitchRads) * math.Cos(yawRads))
			shots = append(shots, goroom.Shot{
				Ray: pt.Ray{
					Origin:    sourcePos,
					Direction: direction,
				}, Normal: pt.Ray{
					Origin:    sourcePos,
					Direction: source.NormalDirection,
				},
				Gain:       1, // Gain always 1 because we are an omnidirectional source
				Yaw:        yaw,
				Pitch:      pitch,
				SourceName: source.Name,
			})
		}
	}

	arrivals := make([]goroom.Arrival, 0, c.NumShots)
	for _, shot := range shots {
		arrival, err := room.TraceShotUnconditional(shot, listenPos, goroom.TraceParams{
			Order:         100,
			GainThreshold: -60,
			TimeThreshold: 800 * MS,
			RFZRadius:     c.ListenRadius,
		})
		if err != nil {
			return err
		}
		arrivals = append(arrivals, arrival...)
	}

	// 2. Bin the arrivals
	binWidth := 1.0     // ms
	maxTime := 600 * MS // last arrival time
	binCount := int(maxTime/binWidth) + 1
	binnedEnergy := make([]float64, binCount)

	for _, arrival := range arrivals {
		binIdx := int(arrival.ITD() / binWidth)
		energy := math.Pow(10, arrival.Gain/10) // Convert dB to energy
		binnedEnergy[binIdx] += energy
	}

	// 3. Compute reverse cumulative energy
	cumEnergy := make([]float64, binCount)
	total := 0.0
	for i := binCount - 1; i >= 0; i-- {
		total += binnedEnergy[i]
		cumEnergy[i] = total
	}

	// 4. Get decay curve: log10(cumEnergy), carefully handle zeroes
	decayDB := make([]float64, binCount)
	for i, e := range cumEnergy {
		if e > 0 {
			decayDB[i] = 10 * math.Log10(e)
		} else {
			decayDB[i] = -1000 // or some large negative value
		}
	}

	// Find dB max (start of decay)
	maxDB := decayDB[0]

	// Choose range for linear regression: -5dB to -35dB below peak
	startThresh := maxDB - 5
	endThresh := maxDB - 35

	var xvals []float64 // times in ms
	var yvals []float64 // decayDB values

	for i := 0; i < binCount; i++ {
		if decayDB[i] <= startThresh && decayDB[i] >= endThresh {
			xvals = append(xvals, float64(i)*binWidth)
			yvals = append(yvals, decayDB[i])
		}
	}

	// Linear regression: fit y = m*x + b
	// We'll implement the simple least squares fit
	var sumX, sumY, sumXY, sumXX float64
	n := float64(len(xvals))
	for i := range xvals {
		sumX += xvals[i]
		sumY += yvals[i]
		sumXY += xvals[i] * yvals[i]
		sumXX += xvals[i] * xvals[i]
	}
	slope := (n*sumXY - sumX*sumY) / (n*sumXX - sumX*sumX)

	// Finally, T60 calculation
	T60 := 60.0 / math.Abs(slope) // T60 in ms (if binWidth is ms)
	fmt.Printf("Estimated T60 decay time: %.2f ms\n", T60)

	return nil
}

func main() {
	ctx := kong.Parse(&CLI)
	err := ctx.Run()
	if err != nil {
		log.Fatal(err)
	}
}

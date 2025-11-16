package main

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"

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

	// for _, surf := range surfaces {
	// 	fmt.Printf("Surface: %s\n", surf.Name)
	// }

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
		fmt.Printf("T60 Sabine at %.0f Hz: %.2f ms\n", testFrequencies[i], t60/MS)
	}

	t60Eyring := make([]float64, len(testFrequencies))
	for i, freq := range testFrequencies {
		t60, err := room.T60Eyring(freq)
		if err != nil {
			return err
		}
		t60Eyring[i] = t60
	}

	for i, t60 := range t60Eyring {
		fmt.Printf("T60 Eyring at %.0f Hz: %.2f ms\n", testFrequencies[i], t60/MS)
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
	normal := pt.Vector{1, 0, 0}
	speakerSpec := config.Speaker.Create()
	source := goroom.NewSpeaker(speakerSpec, sourcePos, normal, "Source") // Normal direction doesn't matter for omnidirectional source
	arrivalsChan := make(chan []goroom.Arrival, config.Simulation.ShotCount)
	wg := sync.WaitGroup{}

	for _, shot := range source.SampleWithNormal(normal, config.Simulation.ShotCount, 180, 180) {
		wg.Add(1)
		go func(shot goroom.Shot) {
			defer wg.Done()
			theseArrivals, err := room.TraceShotUnconditional(
				shot,
				listenPos,
				goroom.TraceParams{
					Order:         config.Simulation.Order,
					GainThreshold: config.Simulation.GainThresholdDB,
					TimeThreshold: config.Simulation.TimeThresholdMS * MS,
					RFZRadius:     config.Simulation.RFZRadius,
				},
				2000)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				fmt.Printf("Shot origin: %v\n", shot.Ray.Origin)
				fmt.Printf("Shot direction: %v\n", shot.Ray.Direction)
				// Optionally: return or continue; for now, just skip bad rays
				return
			}
			arrivalsChan <- theseArrivals
		}(shot)
	}

	wg.Wait()
	close(arrivalsChan)

	arrivals := []goroom.Arrival{}
	for rays := range arrivalsChan {
		arrivals = append(arrivals, rays...)
	}

	// 2. Bin the arrivals
	binWidth := 1.0                                  // ms
	maxTime := config.Simulation.TimeThresholdMS * 2 // last arrival time
	binCount := int(maxTime/binWidth) + 1
	binnedEnergy := make([]float64, binCount)

	for _, arrival := range arrivals {
		binIdx := int(arrival.ITD() / binWidth)
		// fmt.Printf("Arrival at time %.2f ms with gain %.5f dB goes to bin %d\n", arrival.ITD(), goroom.ToDB(arrival.Gain), binIdx)
		energy := arrival.Gain / float64(config.Simulation.ShotCount)
		// energy := arrival.Gain * arrival.Gain / float64(config.Simulation.ShotCount)
		binnedEnergy[binIdx] += energy
	}
	for i := 0; i < binCount; i++ {
		// fmt.Printf("Time bin %f ms: Energy %.6f\n", float64(i)*binWidth, binnedEnergy[i])
	}

	// 3. Compute reverse cumulative energy
	cumEnergy := make([]float64, binCount)
	total := 0.0
	for i := binCount - 1; i >= 0; i-- {
		total += binnedEnergy[i]
		cumEnergy[i] = total
	}

	// for i := 0; i < binCount; i++ {
	// 	fmt.Printf("Time bin %f ms: Cumulative Energy %.6f\n", float64(i)*binWidth, cumEnergy[i])
	// }

	// 4. Get decay curve: log10(cumEnergy), carefully handle zeroes
	decayDB := make([]float64, binCount)
	for i, e := range cumEnergy {
		if e > 0 {
			decayDB[i] = 10 * math.Log10(e)
		} else {
			decayDB[i] = -1000 // or some large negative value
		}
	}

	// for i := 0; i < binCount; i++ {
	// 	fmt.Printf("Time bin %f ms: Decay dB %.2f\n", float64(i)*binWidth, decayDB[i])
	// }

	// Find dB max (start of decay)
	maxDB := decayDB[0]
	for i := 1; i < binCount; i++ {
		if decayDB[i] > maxDB {
			maxDB = decayDB[i]
		}
	}

	// Choose range for linear regression: -10dB to -50dB below peak
	startThresh := maxDB - 10
	endThresh := maxDB - 50

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

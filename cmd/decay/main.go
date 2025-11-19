package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kong"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"

	"github.com/jdginn/go-recording-studio/pt"
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

type ExperimentMetadata struct {
	ConfigFile string `json:"config_file"`
	OutputDir  string `json:"output_dir"`
	Timestamp  string `json:"timestamp"`
}
type FrequencyTNResult struct {
	FreqHz float64 `json:"freq_hz"`
	TNMS   float64 `json:"tn_ms"`
}
type PointPairResult struct {
	PointPair   string              `json:"point_pair"`
	SourcePos   [3]float64          `json:"source_pos"`
	ListenPos   [3]float64          `json:"listen_pos"`
	Frequencies []FrequencyTNResult `json:"frequencies"`
}
type ExperimentSummary struct {
	Experiment ExperimentMetadata `json:"experiment"`
	Results    []PointPairResult  `json:"results"`
}

type traceParams struct {
	ShotCount       int
	Order           int
	GainThresholdDB float64
	TimeThresholdMS float64
	RFZRadius       float64
}

func traceArrivals(room *goroom.Room, source *goroom.Speaker, listenPos pt.Vector, normal pt.Vector, config traceParams, freq float64) []goroom.Arrival {
	arrivalsChan := make(chan []goroom.Arrival, config.ShotCount)

	// Do this per frequency corner
	wg := sync.WaitGroup{}

	for _, shot := range source.SampleWithNormal(normal, config.ShotCount, 180, 180) {
		wg.Add(1)
		go func(shot goroom.Shot) {
			defer wg.Done()
			theseArrivals, err := room.TraceShotUnconditional(
				shot,
				goroom.Omni{Pos: listenPos},
				goroom.TraceParams{
					Order:         config.Order,
					GainThreshold: config.GainThresholdDB,
					TimeThreshold: config.TimeThresholdMS * MS,
					RFZRadius:     config.RFZRadius,
				}, freq)
			if err != nil {
				// fmt.Printf("Error: %v\n", err)
				// fmt.Printf("Shot origin: %v\n", shot.Ray.Origin)
				// fmt.Printf("Shot direction: %v\n", shot.Ray.Direction)
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
	return arrivals
}

func computeTNFromArrivals(arrivals []goroom.Arrival, config roomConfig.Decay) float64 {
	// 2. Bin the arrivals
	binWidth := 1.0                       // ms
	maxTime := config.TimeThresholdMS * 2 // last arrival time
	binCount := int(maxTime/binWidth) + 1
	binnedEnergy := make([]float64, binCount)

	for _, arrival := range arrivals {
		binIdx := int(arrival.ITD() / binWidth)
		energy := arrival.Gain / float64(config.ShotCount)
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
	for i := 1; i < binCount; i++ {
		if decayDB[i] > maxDB {
			maxDB = decayDB[i]
		}
	}

	// Choose range for linear regression: -5dB to -10dB above target
	startThresh := maxDB
	endThresh := maxDB + config.TRangeMS + 10

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

	// Finally, TN calculation
	TN := -config.TRangeMS / math.Abs(slope) // T60 in ms (if binWidth is ms)
	return TN
}

func computeTNFromArrivalsDirectInterval(arrivals []goroom.Arrival, config roomConfig.Decay) float64 {
	// 2. Bin the arrivals
	binWidth := 1.0                       // ms
	maxTime := config.TimeThresholdMS * 2 // last arrival time
	binCount := int(maxTime/binWidth) + 1
	binnedEnergy := make([]float64, binCount)

	for _, arrival := range arrivals {
		binIdx := int(arrival.ITD() / binWidth)
		energy := arrival.Gain / float64(config.ShotCount)
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
	maxIdx := 0
	for i := 1; i < binCount; i++ {
		if decayDB[i] > maxDB {
			maxDB = decayDB[i]
			maxIdx = i
		}
	}

	targetDB := maxDB + config.TRangeMS // e.g., TRangeMS = 30 for TN30, 50 for TN50
	// targetIdx := -1
	for i := maxIdx; i < binCount; i++ {
		if decayDB[i] <= targetDB {
			// targetIdx = i
			break
		}
	}

	// Find bins i (above) and i+1 (below threshold)
	for i := maxIdx; i < binCount-1; i++ {
		if decayDB[i] >= targetDB && decayDB[i+1] < targetDB {
			t1 := float64(i) * binWidth
			t2 := float64(i+1) * binWidth
			d1 := decayDB[i]
			d2 := decayDB[i+1]
			// Linear interpolation for exact crossing time
			decayTime := t1 + (targetDB-d1)/(d2-d1)*(t2-t1)
			return decayTime
		}
	}

	// // If we found a valid interval
	// if targetIdx != -1 && targetIdx > maxIdx {
	// 	TN := float64(targetIdx-maxIdx) * binWidth // time to decay by config.TRangeMS dB
	// 	return TN
	// }

	// Else, not enough decay in simulation window
	return -1
}

func plotBinnedEnergy(arrivals []goroom.Arrival, config roomConfig.Decay, filename string) {
	// 2. Bin the arrivals
	binWidth := 0.1                       // ms
	maxTime := config.TimeThresholdMS * 2 // last arrival time
	binCount := int(maxTime/binWidth) + 1
	binnedEnergy := make([]float64, binCount)

	for _, arrival := range arrivals {
		binIdx := int(arrival.ITD() / binWidth)
		energy := arrival.Gain
		binnedEnergy[binIdx] += energy
	}

	p := plot.New()
	p.Title.Text = "Binned Arrival Energy"
	p.X.Label.Text = "Time (ms)"
	p.Y.Label.Text = "Energy in Bin"

	pts := make(plotter.XYs, len(binnedEnergy))
	db := 0.0
	var last float64
	for i := 0; i < int(250.0/binWidth); i++ {
		e := binnedEnergy[i]
		if e > 0 {
			db = 10 * math.Log10(e)
			last = db
			pts[i].X = float64(i) * binWidth
			pts[i].Y = db
		} else {
			pts[i].X = float64(i) * binWidth
			pts[i].Y = last
		}
	}

	bar, err := plotter.NewLine(pts) // Use NewLine for energy over time; for true bar chart use plotter.NewBarChart
	if err != nil {
		log.Fatalf("error creating plot line: %v", err)
	}
	p.Add(bar)
	p.Add(plotter.NewGrid())
	if err := p.Save(8*vg.Inch, 5*vg.Inch, filename); err != nil {
		log.Fatalf("error saving plot: %v", err)
	}
}

func plotCumulativeEnergy(arrivals []goroom.Arrival, config roomConfig.Decay, filename string) {
	// 2. Bin the arrivals
	binWidth := 0.1                       // ms
	maxTime := config.TimeThresholdMS * 2 // last arrival time
	binCount := int(maxTime/binWidth) + 1
	binnedEnergy := make([]float64, binCount)

	for _, arrival := range arrivals {
		binIdx := int(arrival.ITD() / binWidth)
		energy := arrival.Gain
		binnedEnergy[binIdx] += energy
	}

	// 3. Compute reverse cumulative energy
	cumEnergy := make([]float64, binCount)
	total := 0.0
	for i := binCount - 1; i >= 0; i-- {
		total += binnedEnergy[i]
		cumEnergy[i] = total
	}

	p := plot.New()
	p.Title.Text = "Cumulative Arrival Energy"
	p.X.Label.Text = "Time (ms)"
	p.Y.Label.Text = "Cumulative Energy"

	pts := make(plotter.XYs, len(cumEnergy))
	db := -60.0

	// t20 := computeTNFromArrivals(arrivals, config)
	for i := 0; i < int(250.0/binWidth); i++ {
		e := cumEnergy[i]
		if e > 0 {
			db = 10 * math.Log10(e)
			pts[i].X = float64(i) * binWidth
			pts[i].Y = db
		} else {
			pts[i].X = float64(i) * binWidth
			pts[i].Y = -60.0
		}
	}

	bar, err := plotter.NewLine(pts) // Use NewLine for energy over time; for true bar chart use plotter.NewBarChart
	if err != nil {
		log.Fatalf("error creating plot line: %v", err)
	}
	p.Add(bar)
	p.Add(plotter.NewGrid())
	p.Y.Max = 30.0
	p.Y.Min = -60.0
	if err := p.Save(8*vg.Inch, 5*vg.Inch, filename); err != nil {
		log.Fatalf("error saving plot: %v", err)
	}
}

var CLI struct {
	Simulate SimulateCmd `cmd:"" help:"Simulate T60 for a room"`
}

type SimulateCmd struct {
	Config       string  `arg:"" name:"config" help:"config file to simulate"`
	Mesh         string  `name:"mesh" help:"override mesh file in config"`
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
		if err != nil {
			return fmt.Errorf("using existing experiment directory: %w", err)
		}
	} else {
		expDir, err = roomExperiment.CreateExperimentDirectory("experiments")
		if err != nil {
			return fmt.Errorf("creating experiment directory: %w", err)
		}
	}
	if err := expDir.CopyConfigFile(c.Config); err != nil {
		return fmt.Errorf("copying config file: %w", err)
	}

	// Default frequency corners
	testFrequencies := []float64{
		63, 125, 250, 500, 1000, 2000, 4000, 8000, 16000,
	}
	if len(config.Decay.FreqCorners) > 0 {
		testFrequencies = make([]float64, len(config.Decay.FreqCorners))
		for i, fc := range config.Decay.FreqCorners {
			testFrequencies[i] = float64(fc)
		}
	}

	if c.Mesh != "" {
		config.Input.Mesh.Path = c.Mesh
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
	if err != nil {
		return err
	}
	fmt.Printf("Room volume: %.2f m³\n\n", vol)

	schroeder, err := room.SchroederFreq()
	if err != nil {
		return err
	}
	fmt.Printf("Schroeder frequency: %.0f Hz\n\n", schroeder)

	t60Sabine := make([]float64, len(testFrequencies))
	for i, freq := range testFrequencies {
		t60, err := room.T60Sabine(freq)
		if err != nil {
			return err
		}
		t60Sabine[i] = t60
	}

	t60Eyring := make([]float64, len(testFrequencies))
	for i, freq := range testFrequencies {
		t60, err := room.T60Eyring(freq)
		if err != nil {
			return err
		}
		t60Eyring[i] = t60
	}

	fmt.Println("Sabine:")
	for i, freq := range testFrequencies {
		fmt.Printf("\t%.0fHz: %.2f ms\n", freq, t60Sabine[i]/MS)
	}
	fmt.Printf("\n")
	fmt.Println("Eyring:")
	for i, freq := range testFrequencies {
		fmt.Printf("\t%.0fHz: %.2f ms\n", freq, t60Eyring[i]/MS)
	}
	fmt.Printf("\n")

	// Ray tracing part

	params := traceParams{
		ShotCount:       config.Decay.ShotCount,
		Order:           config.Decay.Order,
		GainThresholdDB: config.Decay.GainThresholdDB,
		TimeThresholdMS: config.Decay.TimeThresholdMS,
		RFZRadius:       config.Decay.RFZRadius,
	}

	// if params.GainThresholdDB > (config.Decay.TRangeMS - 10.0) {
	// 	params.GainThresholdDB = config.Decay.TRangeMS - 10.0
	// }
	// Override the simulation threshold to a safe but efficient setting
	params.GainThresholdDB = config.Decay.TRangeMS - 10.0

	normal := pt.Vector{1, 0, 0}
	speakerSpec := config.Speaker.Create()

	summary := ExperimentSummary{
		Experiment: ExperimentMetadata{
			ConfigFile: c.Config,
			OutputDir:  expDir.Path,
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
		},
		Results: []PointPairResult{},
	}

	for _, pair := range config.Decay.PointPairs {
		fmt.Printf("%s:\n", pair.Name)
		sourcePos := pair.Source.ToVector()
		listenPos := pair.Listen.ToVector()
		source := goroom.NewSpeaker(speakerSpec, sourcePos, normal, fmt.Sprintf(pair.Name+"_source")) // Normal direction doesn't matter for omnidirectional source

		pairRes := PointPairResult{
			PointPair:   pair.Name,
			SourcePos:   [3]float64{sourcePos.X, sourcePos.Y, sourcePos.Z},
			ListenPos:   [3]float64{listenPos.X, listenPos.Y, listenPos.Z},
			Frequencies: []FrequencyTNResult{},
		}
		for _, freq := range testFrequencies {
			arrivals := traceArrivals(room, &source, listenPos, normal, params, freq)

			t60 := computeTNFromArrivals(arrivals, config.Decay)
			fmt.Printf("\t%.0fHz Lin regression: %.2f ms\n", freq, t60)
			pairRes.Frequencies = append(pairRes.Frequencies, FrequencyTNResult{
				FreqHz: freq,
				TNMS:   t60,
			})
		}
		summary.Results = append(summary.Results, pairRes)
	}

	// BEGIN: Write summary.json at end
	summaryPath := expDir.Path + "/summary.json"
	jsonBytes, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal summary: %w", err)
	}
	if err := os.WriteFile(summaryPath, jsonBytes, 0644); err != nil {
		return fmt.Errorf("failed to write summary.json: %w", err)
	}
	fmt.Printf("Summary JSON written to: %s\n", summaryPath)

	return nil
}

func main() {
	ctx := kong.Parse(&CLI)
	err := ctx.Run()
	if err != nil {
		log.Fatal(err)
	}
}

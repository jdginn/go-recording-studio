package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
)

type Point3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type RayJSON struct {
	Origin Point3 `json:"origin"`
}

type ShotJSON struct {
	Ray RayJSON `json:"ray"`
}

type NearestApproachJSON struct {
	Distance float64 `json:"distance"`
	Position Point3  `json:"position"`
}

// AcousticPathJSON is a minimal struct for our needs
type AcousticPathJSON struct {
	Gain            float64             `json:"gain"`     // dB
	Distance        float64             `json:"distance"` // meters
	Shot            ShotJSON            `json:"shot"`
	NearestApproach NearestApproachJSON `json:"nearestApproach"`
}

// AnnotationsJSONContainer mirrors WriteToJSON output
type AnnotationsJSONContainer struct {
	AcousticPaths []AcousticPathJSON `json:"acousticPaths"`
}

func euclideanDistance(a, b Point3) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	dz := a.Z - b.Z
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// Collect (time_ms, gain_db) pairs
type Peak struct {
	TimeMs float64
	GainDb float64
}

// FindLocalMaximaClusters merges peaks that are "close enough" into a single representative peak.
// - timeThreshold: max allowed time difference in ms within a cluster
// - gainThreshold: max allowed gain difference in dB within a cluster
func FindLocalMaximaClusters(peaks []Peak, timeThreshold, gainThreshold float64) []Peak {
	if len(peaks) == 0 {
		return nil
	}

	var clusters [][]Peak
	current := []Peak{peaks[0]}
	for i := 1; i < len(peaks); i++ {
		last := current[len(current)-1]
		dt := peaks[i].TimeMs - last.TimeMs
		dg := math.Abs(peaks[i].GainDb - last.GainDb)
		if dt <= timeThreshold && dg <= gainThreshold {
			current = append(current, peaks[i])
		} else {
			clusters = append(clusters, current)
			current = []Peak{peaks[i]}
		}
	}
	clusters = append(clusters, current)

	// For each cluster, select the peak with the earliest time and highest gain
	var result []Peak
	for _, cluster := range clusters {
		best := cluster[0]
		for _, p := range cluster {
			// Higher gain = closer to 0dB; if tie, use earliest time
			if p.GainDb > best.GainDb || (math.Abs(p.GainDb-best.GainDb) < 1e-6 && p.TimeMs < best.TimeMs) {
				best = p
			}
		}
		result = append(result, best)
	}
	return result
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: ir_peaks <input_annotations.json> <output.txt>")
		os.Exit(1)
	}

	inFile := os.Args[1]
	outFile := os.Args[2]

	f, err := os.Open(inFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open input file: %v\n", err)
		os.Exit(2)
	}
	defer f.Close()

	var annotations AnnotationsJSONContainer
	if err := json.NewDecoder(f).Decode(&annotations); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot decode JSON: %v\n", err)
		os.Exit(3)
	}

	var peaks []Peak
	const speedOfSound = 343.0 // m/s

	for _, path := range annotations.AcousticPaths {
		directDist := euclideanDistance(path.NearestApproach.Position, path.Shot.Ray.Origin)
		deltaDist := path.Distance - directDist
		timeMs := deltaDist / speedOfSound * 1000.0
		peaks = append(peaks, Peak{TimeMs: timeMs, GainDb: path.Gain})
	}

	// Sort by time (optional, for pretty output)
	sort.Slice(peaks, func(i, j int) bool {
		return peaks[i].TimeMs < peaks[j].TimeMs
	})

	// Write output
	out, err := os.Create(outFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create output file: %v\n", err)
		os.Exit(4)
	}
	defer out.Close()

	// fmt.Fprintln(out, "Impulse Response Peaks")
	for _, p := range FindLocalMaximaClusters(peaks, 0.05, 4) {
		// for _, p := range peaks {
		fmt.Fprintf(out, "%.6fms, %.2fdB\n", p.TimeMs, p.GainDb)
	}
}

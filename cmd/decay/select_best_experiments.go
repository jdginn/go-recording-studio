package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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

// Struct for params JSON
type ExperimentParams struct {
	LNumReflectors  float64 `json:"l_num_reflectors"`
	LReflectorAngle float64 `json:"l_reflector_angle"`
	LReflectorDepth float64 `json:"l_reflector_depth"`
	LStartOffset    float64 `json:"l_start_offset"`
	LFinishOffset   float64 `json:"l_finish_offset"`
	RNumReflectors  float64 `json:"r_num_reflectors"`
	RReflectorAngle float64 `json:"r_reflector_angle"`
	RReflectorDepth float64 `json:"r_reflector_depth"`
	RStartOffset    float64 `json:"r_start_offset"`
	RFinishOffset   float64 `json:"r_finish_offset"`
}

type FitnessMetrics struct {
	DiffLiveDead float64 // average
	DeadDecay    float64
}

// Change this fitness function as needed!
func fitness(summary ExperimentSummary, params ExperimentParams) (FitnessMetrics, float64) {
	metrics := FitnessMetrics{
		DiffLiveDead: (math.Abs(summary.Results[0].Frequencies[3].T30MS-summary.Results[1].Frequencies[3].T30MS) +
			math.Abs(summary.Results[0].Frequencies[1].T30MS-summary.Results[1].Frequencies[1].T30MS) +
			math.Abs(summary.Results[0].Frequencies[2].T30MS-summary.Results[1].Frequencies[2].T30MS)) / 3.0,
		DeadDecay: summary.Results[1].Frequencies[2].T30MS,
	}

	return metrics, metrics.DiffLiveDead
	// return metrics, metrics.DiffLiveDead*2 - metrics.DeadDecay - (params.LReflectorDepth+params.RReflectorDepth)/2 - (params.LNumReflectors+params.RNumReflectors)/5
}

func main() {
	var outputRoot string
	flag.StringVar(&outputRoot, "output-dir", "", "Root directory containing experiment outputs")
	flag.Parse()

	if outputRoot == "" {
		fmt.Println("Usage: select_best_experiments --output-dir /path/to/output")
		os.Exit(1)
	}

	// Find all *_results directories
	dirs, err := os.ReadDir(outputRoot)
	if err != nil {
		fmt.Printf("Error reading output root: %v\n", err)
		os.Exit(1)
	}

	type Candidate struct {
		Path    string
		Summary ExperimentSummary
		Params  ExperimentParams
		Metrics FitnessMetrics
		Fitness float64
	}

	var candidates []Candidate
	for _, d := range dirs {
		if !d.IsDir() || !strings.HasSuffix(d.Name(), "-results") {
			continue
		}
		subdir := filepath.Join(outputRoot, d.Name())
		summaryPath := filepath.Join(subdir, "summary.json")

		if _, err := os.Stat(summaryPath); err != nil {
			continue
		}

		// Find params json file
		base := strings.TrimSuffix(d.Name(), "-results")
		paramsPath := filepath.Join(subdir, base+".json")
		if _, err := os.Stat(paramsPath); err != nil {
			fmt.Printf("WARN: No param json for %s\n", subdir)
			continue
		}

		summaryData, err := ioutil.ReadFile(summaryPath)
		if err != nil {
			fmt.Printf("Error reading %s: %v\n", summaryPath, err)
			continue
		}
		var summary ExperimentSummary
		if err := json.Unmarshal(summaryData, &summary); err != nil {
			fmt.Printf("Bad summary.json in %s: %v\n", summaryPath, err)
			continue
		}

		paramsData, err := ioutil.ReadFile(paramsPath)
		if err != nil {
			fmt.Printf("Error reading %s: %v\n", paramsPath, err)
			continue
		}
		var params ExperimentParams
		if err := json.Unmarshal(paramsData, &params); err != nil {
			fmt.Printf("Bad params json in %s: %v\n", paramsPath, err)
			continue
		}

		metrics, fitness := fitness(summary, params)
		candidates = append(candidates, Candidate{
			Path:    subdir,
			Summary: summary,
			Params:  params,
			Metrics: metrics,
			Fitness: fitness,
		})
	}

	// Sort by fitness ascending (best first)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Fitness > candidates[j].Fitness
	})

	nCandidates := 5
	fmt.Printf("Top %d experiments (of %d found):\n", nCandidates, len(candidates))
	for i := 0; i < len(candidates) && i < nCandidates; i++ {
		c := candidates[i]
		fmt.Printf("------------------------------------------------------\n")
		fmt.Printf("%2d. Experiment: %s\n", i+1, filepath.Base(c.Path))
		fmt.Printf("    Fitness: %.6f\n", c.Fitness)

		fmt.Printf("    Fitness Metrics:\n")
		fmt.Printf("      DiffLiveDead: %.6f\n", c.Metrics.DiffLiveDead)
		fmt.Printf("      DeadDecay:    %.6f\n", c.Metrics.DeadDecay)

		fmt.Printf("    Params:\n")
		fmt.Printf("      l_num_reflectors:    %.2f\n", c.Params.LNumReflectors)
		fmt.Printf("      l_reflector_angle:   %.2f\n", c.Params.LReflectorAngle)
		fmt.Printf("      l_reflector_depth:   %.2f\n", c.Params.LReflectorDepth)
		fmt.Printf("      l_start_offset:      %.2f\n", c.Params.LStartOffset)
		fmt.Printf("      l_finish_offset:     %.2f\n", c.Params.LFinishOffset)
		fmt.Printf("      r_num_reflectors:    %.2f\n", c.Params.RNumReflectors)
		fmt.Printf("      r_reflector_angle:   %.2f\n", c.Params.RReflectorAngle)
		fmt.Printf("      r_reflector_depth:   %.2f\n", c.Params.RReflectorDepth)
		fmt.Printf("      r_start_offset:      %.2f\n", c.Params.RStartOffset)
		fmt.Printf("      r_finish_offset:     %.2f\n", c.Params.RFinishOffset)
		fmt.Printf("------------------------------------------------------\n\n")
	}
	fmt.Printf("Bottom %d experiments (of %d found):\n", nCandidates, len(candidates))
	for i := len(candidates) - 1; i >= 0 && i >= len(candidates)-nCandidates; i-- {
		c := candidates[i]
		fmt.Printf("------------------------------------------------------\n")
		fmt.Printf("%2d. Experiment: %s\n", i+1, filepath.Base(c.Path))
		fmt.Printf("    Fitness: %.6f\n", c.Fitness)

		fmt.Printf("    Fitness Metrics:\n")
		fmt.Printf("      DiffLiveDead: %.6f\n", c.Metrics.DiffLiveDead)
		fmt.Printf("      DeadDecay:    %.6f\n", c.Metrics.DeadDecay)

		fmt.Printf("    Params:\n")
		fmt.Printf("      l_num_reflectors:    %.2f\n", c.Params.LNumReflectors)
		fmt.Printf("      l_reflector_angle:   %.2f\n", c.Params.LReflectorAngle)
		fmt.Printf("      l_reflector_depth:   %.2f\n", c.Params.LReflectorDepth)
		fmt.Printf("      l_start_offset:      %.2f\n", c.Params.LStartOffset)
		fmt.Printf("      l_finish_offset:     %.2f\n", c.Params.LFinishOffset)
		fmt.Printf("      r_num_reflectors:    %.2f\n", c.Params.RNumReflectors)
		fmt.Printf("      r_reflector_angle:   %.2f\n", c.Params.RReflectorAngle)
		fmt.Printf("      r_reflector_depth:   %.2f\n", c.Params.RReflectorDepth)
		fmt.Printf("      r_start_offset:      %.2f\n", c.Params.RStartOffset)
		fmt.Printf("      r_finish_offset:     %.2f\n", c.Params.RFinishOffset)
		fmt.Printf("------------------------------------------------------\n\n")
	}
}

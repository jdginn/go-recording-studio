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
	T20MS            float64 `json:"t20_ms"`
	T30MS            float64 `json:"t30_ms"`
	EchoDensityScore float64 `json:"echo_density_score"`
	TemporalKurtosis float64 `json:"temporal_kurtosis"`
}
type PointPairResult struct {
	SourcePos [3]float64 `json:"source_pos"`
	// Microphone  room.Microphone           `json:"microphone"`
	Frequencies map[int]FrequencyTNResult `json:"frequencies"`
}
type ExperimentSummary struct {
	Experiment ExperimentMetadata         `json:"experiment"`
	Results    map[string]PointPairResult `json:"results"`
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

type PositionMetrics struct {
	Name             string
	T20MS            float64
	T30MS            float64
	EchoDensity      float64
	TemporalKurtosis float64
}

func (pm PositionMetrics) String() string {
	return fmt.Sprintf("%s: T20MS: %.2f, T30MS: %.2f, EchoDensity: %.2f, TemporalKurtosis: %.2f", pm.Name, pm.T20MS, pm.T30MS, pm.EchoDensity, pm.TemporalKurtosis)
}

type FitnessMetrics struct {
	VoxWindowToDoorT30   float64
	VoxCenterToWindowT30 float64
	VoxDoorToWindowT30   float64
	VoxDiffusion         float64
	DrumsWindowCardT30   float64
	DrumsWindowOmniT30   float64
	DrumsWindowRoomT30   float64
	DrumsDoorCardT30     float64
	DrumsDoorOmniT30     float64
	DrumsDoorRoomT30     float64
	T30                  float64
	SpacingAsymmetry     float64
	VoxFitness           float64
	DrumDeadFitness      float64
	DrumLiveFitness      float64
}

func avg[T any](keys []T, f func(T) float64) float64 {
	var sum float64
	for _, k := range keys {
		sum += f(k)
	}
	return sum / float64(len(keys))
}

func pickMin[T any](keys []T, f func(T) float64) T {
	min := f(keys[0])
	minKey := keys[0]
	for _, k := range keys[1:] {
		v := f(k)
		if v < min {
			min = v
			minKey = k
		}
	}
	return minKey
}

func pickMax[T any](keys []T, f func(T) float64) T {
	max := f(keys[0])
	maxKey := keys[0]
	for _, k := range keys[1:] {
		v := f(k)
		if v > max {
			max = v
			maxKey = k
		}
	}
	return maxKey
}

func calcPositionMetrics(summary ExperimentSummary, names []string, freqs []int) []PositionMetrics {
	var metrics []PositionMetrics
	for _, name := range names {
		t20 := avg(freqs, func(freq int) float64 {
			return summary.Results[name].Frequencies[freq].T20MS
		})
		t30 := avg(freqs, func(freq int) float64 {
			return summary.Results[name].Frequencies[freq].T30MS
		})
		echoDensity := avg(freqs, func(freq int) float64 {
			return summary.Results[name].Frequencies[freq].EchoDensityScore
		})
		temporalKurtosis := avg(freqs, func(freq int) float64 {
			return summary.Results[name].Frequencies[freq].TemporalKurtosis
		})
		metrics = append(metrics, PositionMetrics{
			Name:             name,
			T20MS:            t20,
			T30MS:            t30,
			EchoDensity:      echoDensity,
			TemporalKurtosis: temporalKurtosis,
		})
	}
	return metrics
}

// Change this fitness function as needed!
func fitness(summary ExperimentSummary, params ExperimentParams) (FitnessMetrics, float64) {
	metrics := FitnessMetrics{
		VoxWindowToDoorT30: avg([]int{500, 1000, 2000, 4000}, func(freq int) float64 {
			return summary.Results["vox_window_to_door"].Frequencies[freq].T30MS
		}),
		VoxCenterToWindowT30: avg([]int{500, 1000, 2000, 4000}, func(freq int) float64 {
			return summary.Results["vox_center_to_window"].Frequencies[freq].T30MS
		}),
		VoxDoorToWindowT30: avg([]int{500, 1000, 2000, 4000}, func(freq int) float64 {
			return summary.Results["vox_door_to_window"].Frequencies[freq].T30MS
		}),
		VoxDiffusion: avg([]int{500, 1000, 2000, 4000}, func(freq int) float64 {
			f := func(names []string) float64 {
				sum := 0.0
				for _, name := range names {
					sum += summary.Results[name].Frequencies[freq].EchoDensityScore + summary.Results[name].Frequencies[freq].TemporalKurtosis
				}
				return sum / float64(len(names))
			}
			return f([]string{"vox_window_to_door", "vox_center_to_window", "vox_door_to_window"})
		}) * 50,
		DrumsWindowCardT30: avg([]int{500, 1000, 2000, 4000}, func(freq int) float64 {
			return summary.Results["window_drums_OH_back_cardioid"].Frequencies[freq].T30MS
		}),
		DrumsWindowOmniT30: avg([]int{500, 1000, 2000, 4000}, func(freq int) float64 {
			return summary.Results["window_drums_OH_omni"].Frequencies[freq].T30MS
		}),
		DrumsWindowRoomT30: avg([]int{500, 1000, 2000, 4000}, func(freq int) float64 {
			return summary.Results["window_drums_far_omni"].Frequencies[freq].T30MS
		}),
		DrumsDoorCardT30: avg([]int{500, 1000, 2000, 4000}, func(freq int) float64 {
			return summary.Results["door_drums_OH_cardioid"].Frequencies[freq].T30MS
		}),
		DrumsDoorOmniT30: avg([]int{500, 1000, 2000, 4000}, func(freq int) float64 {
			return summary.Results["door_drums_OH_omni"].Frequencies[freq].T30MS
		}),
		DrumsDoorRoomT30: avg([]int{500, 1000, 2000, 4000}, func(freq int) float64 {
			return summary.Results["door_drums_room_omni"].Frequencies[freq].T30MS
		}),
		SpacingAsymmetry: math.Abs(params.LFinishOffset-params.RFinishOffset) + math.Abs(params.LStartOffset-params.RStartOffset),
	}
	metrics.VoxFitness = (metrics.VoxDoorToWindowT30*2 + metrics.VoxCenterToWindowT30 - metrics.VoxWindowToDoorT30*4 + metrics.VoxDiffusion) * 3
	metrics.DrumLiveFitness = (metrics.DrumsDoorCardT30*2 + metrics.DrumsDoorOmniT30 + metrics.DrumsDoorRoomT30*3)
	metrics.DrumDeadFitness = -(metrics.DrumsWindowCardT30*3 + metrics.DrumsWindowOmniT30 + metrics.DrumsWindowRoomT30/3)
	fitness := metrics.VoxFitness + metrics.DrumLiveFitness + metrics.DrumDeadFitness - metrics.SpacingAsymmetry*2
	return metrics, fitness
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
		fmt.Printf("    Fitness: %.1f\n", c.Fitness)

		fmt.Printf("    Fitness Metrics:\n")
		fmt.Printf("      VoxFitness:           %.1f\n", c.Metrics.VoxFitness)
		fmt.Printf("      DrumDeadFitness:      %.1f\n", c.Metrics.DrumDeadFitness)
		fmt.Printf("      DrumLiveFitness:      %.1f\n", c.Metrics.DrumLiveFitness)
		fmt.Printf("      VoxWindowToDoorT30:   %.1f\n", c.Metrics.VoxWindowToDoorT30)
		fmt.Printf("      VoxCenterToWindowT30: %.1f\n", c.Metrics.VoxCenterToWindowT30)
		fmt.Printf("      VoxDoorToWindowT30:   %.1f\n", c.Metrics.VoxDoorToWindowT30)
		fmt.Printf("      VoxDiffusion:         %.1f\n", c.Metrics.VoxDiffusion)
		fmt.Printf("      DrumsWindowCardT30:   %.1f\n", c.Metrics.DrumsWindowCardT30)
		fmt.Printf("      DrumsWindowOmniT30:   %.1f\n", c.Metrics.DrumsWindowOmniT30)
		fmt.Printf("      DrumsWindowRoomT30:   %.1f\n", c.Metrics.DrumsWindowRoomT30)
		fmt.Printf("      DrumsDoorCardT30:     %.1f\n", c.Metrics.DrumsDoorCardT30)
		fmt.Printf("      DrumsDoorOmniT30:     %.1f\n", c.Metrics.DrumsDoorOmniT30)
		fmt.Printf("      DrumsDoorRoomT30:     %.1f\n", c.Metrics.DrumsDoorRoomT30)
		fmt.Printf("      SpacingAsymmetry:     %.1f\n", c.Metrics.SpacingAsymmetry)

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
		fmt.Printf("    Fitness: %.1f\n", c.Fitness)

		fmt.Printf("    Fitness Metrics:\n")
		fmt.Printf("      VoxFitness:           %.1f\n", c.Metrics.VoxFitness)
		fmt.Printf("      DrumDeadFitness:      %.1f\n", c.Metrics.DrumDeadFitness)
		fmt.Printf("      DrumLiveFitness:      %.1f\n", c.Metrics.DrumLiveFitness)
		fmt.Printf("      VoxWindowToDoorT30:   %.1f\n", c.Metrics.VoxWindowToDoorT30)
		fmt.Printf("      VoxCenterToWindowT30: %.1f\n", c.Metrics.VoxCenterToWindowT30)
		fmt.Printf("      VoxDoorToWindowT30:   %.1f\n", c.Metrics.VoxDoorToWindowT30)
		fmt.Printf("      VoxDiffusion:         %.1f\n", c.Metrics.VoxDiffusion)
		fmt.Printf("      DrumsWindowCardT30:   %.1f\n", c.Metrics.DrumsWindowCardT30)
		fmt.Printf("      DrumsWindowOmniT30:   %.1f\n", c.Metrics.DrumsWindowOmniT30)
		fmt.Printf("      DrumsWindowRoomT30:   %.1f\n", c.Metrics.DrumsWindowRoomT30)
		fmt.Printf("      DrumsDoorCardT30:     %.1f\n", c.Metrics.DrumsDoorCardT30)
		fmt.Printf("      DrumsDoorOmniT30:     %.1f\n", c.Metrics.DrumsDoorOmniT30)
		fmt.Printf("      DrumsDoorRoomT30:     %.1f\n", c.Metrics.DrumsDoorRoomT30)
		fmt.Printf("      SpacingAsymmetry:     %.1f\n", c.Metrics.SpacingAsymmetry)

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

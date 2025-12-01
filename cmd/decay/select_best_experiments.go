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
	SourcePos   [3]float64                `json:"source_pos"`
	Frequencies map[int]FrequencyTNResult `json:"frequencies"`
}
type ExperimentSummary struct {
	Experiment ExperimentMetadata         `json:"experiment"`
	Results    map[string]PointPairResult `json:"results"`
}
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

type Metrics struct {
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
	SpacingAsymmetry     float64
	TotalDepth           float64
}

type RelativeScores struct {
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
	SpacingAsymmetry     float64
	TotalDepth           float64
}

type Candidate struct {
	Path     string
	Summary  ExperimentSummary
	Params   ExperimentParams
	Metrics  Metrics
	Relative RelativeScores
}

func avg(keys []int, f func(int) float64) float64 {
	var sum float64
	for _, k := range keys {
		sum += f(k)
	}
	return sum / float64(len(keys))
}

func calcMetrics(summary ExperimentSummary, params ExperimentParams) Metrics {
	metrics := Metrics{
		VoxWindowToDoorT30: avg([]int{500, 1000, 2000, 4000},
			func(freq int) float64 { return summary.Results["vox_window_to_door"].Frequencies[freq].T30MS }),
		VoxCenterToWindowT30: avg([]int{500, 1000, 2000, 4000},
			func(freq int) float64 { return summary.Results["vox_center_to_window"].Frequencies[freq].T30MS }),
		VoxDoorToWindowT30: avg([]int{500, 1000, 2000, 4000},
			func(freq int) float64 { return summary.Results["vox_door_to_window"].Frequencies[freq].T30MS }),
		VoxDiffusion: avg([]int{500, 1000, 2000, 4000}, func(freq int) float64 {
			f := func(names []string) float64 {
				sum := 0.0
				for _, name := range names {
					sum += summary.Results[name].Frequencies[freq].EchoDensityScore +
						summary.Results[name].Frequencies[freq].TemporalKurtosis
				}
				return sum / float64(len(names))
			}
			return f([]string{"vox_window_to_door", "vox_center_to_window", "vox_door_to_window"})
		}) * 50,
		DrumsWindowCardT30: avg([]int{500, 1000, 2000, 4000},
			func(freq int) float64 {
				return summary.Results["window_drums_OH_back_cardioid"].Frequencies[freq].T30MS
			}),
		DrumsWindowOmniT30: avg([]int{500, 1000, 2000, 4000},
			func(freq int) float64 { return summary.Results["window_drums_OH_omni"].Frequencies[freq].T30MS }),
		DrumsWindowRoomT30: avg([]int{500, 1000, 2000, 4000},
			func(freq int) float64 { return summary.Results["window_drums_far_omni"].Frequencies[freq].T30MS }),
		DrumsDoorCardT30: avg([]int{500, 1000, 2000, 4000},
			func(freq int) float64 { return summary.Results["door_drums_OH_cardioid"].Frequencies[freq].T30MS }),
		DrumsDoorOmniT30: avg([]int{500, 1000, 2000, 4000},
			func(freq int) float64 { return summary.Results["door_drums_OH_omni"].Frequencies[freq].T30MS }),
		DrumsDoorRoomT30: avg([]int{500, 1000, 2000, 4000},
			func(freq int) float64 { return summary.Results["door_drums_room_omni"].Frequencies[freq].T30MS }),
		SpacingAsymmetry: math.Abs(params.LFinishOffset-params.RFinishOffset) +
			math.Abs(params.LStartOffset-params.RStartOffset),
		TotalDepth: params.LReflectorDepth + params.RReflectorDepth,
	}
	return metrics
}

func main() {
	var outputRoot string
	flag.StringVar(&outputRoot, "output-dir", "", "Root directory containing experiment outputs")
	flag.Parse()

	if outputRoot == "" {
		fmt.Println("Usage: select_best_experiments --output-dir /path/to/output")
		os.Exit(1)
	}

	dirs, err := os.ReadDir(outputRoot)
	if err != nil {
		fmt.Printf("Error reading output root: %v\n", err)
		os.Exit(1)
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

		metrics := calcMetrics(summary, params)
		candidates = append(candidates, Candidate{
			Path:    subdir,
			Summary: summary,
			Params:  params,
			Metrics: metrics,
		})
	}

	if len(candidates) == 0 {
		fmt.Println("No experiment candidates found.")
		os.Exit(1)
	}

	// Find "best" observed {max (big is good) OR min (small is good)} for each metric
	best := Metrics{}
	worst := Metrics{}
	first := candidates[0].Metrics
	best = first // Init

	for _, c := range candidates[1:] {
		m := c.Metrics
		// Bigger is better
		best.VoxCenterToWindowT30 = math.Max(best.VoxCenterToWindowT30, m.VoxCenterToWindowT30)
		best.VoxDoorToWindowT30 = math.Max(best.VoxDoorToWindowT30, m.VoxDoorToWindowT30)
		best.DrumsDoorCardT30 = math.Max(best.DrumsDoorCardT30, m.DrumsDoorCardT30)
		best.DrumsDoorOmniT30 = math.Max(best.DrumsDoorOmniT30, m.DrumsDoorOmniT30)
		best.DrumsDoorRoomT30 = math.Max(best.DrumsDoorRoomT30, m.DrumsDoorRoomT30)
		best.VoxDiffusion = math.Max(best.VoxDiffusion, m.VoxDiffusion)

		worst.VoxCenterToWindowT30 = math.Min(worst.VoxCenterToWindowT30, m.VoxCenterToWindowT30)
		worst.VoxDoorToWindowT30 = math.Min(worst.VoxDoorToWindowT30, m.VoxDoorToWindowT30)
		worst.DrumsDoorCardT30 = math.Min(worst.DrumsDoorCardT30, m.DrumsDoorCardT30)
		worst.DrumsDoorOmniT30 = math.Min(worst.DrumsDoorOmniT30, m.DrumsDoorOmniT30)
		worst.DrumsDoorRoomT30 = math.Min(worst.DrumsDoorRoomT30, m.DrumsDoorRoomT30)
		worst.VoxDiffusion = math.Max(worst.VoxDiffusion, m.VoxDiffusion)
		// Smaller is better
		best.VoxWindowToDoorT30 = math.Min(best.VoxWindowToDoorT30, m.VoxWindowToDoorT30)
		best.DrumsWindowCardT30 = math.Min(best.DrumsWindowCardT30, m.DrumsWindowCardT30)
		best.DrumsWindowOmniT30 = math.Min(best.DrumsWindowOmniT30, m.DrumsWindowOmniT30)
		best.DrumsWindowRoomT30 = math.Min(best.DrumsWindowRoomT30, m.DrumsWindowRoomT30)
		best.SpacingAsymmetry = math.Min(best.SpacingAsymmetry, m.SpacingAsymmetry)
		best.TotalDepth = math.Min(best.TotalDepth, m.TotalDepth)

		worst.VoxWindowToDoorT30 = math.Max(worst.VoxWindowToDoorT30, m.VoxWindowToDoorT30)
		worst.DrumsWindowCardT30 = math.Max(worst.DrumsWindowCardT30, m.DrumsWindowCardT30)
		worst.DrumsWindowOmniT30 = math.Max(worst.DrumsWindowOmniT30, m.DrumsWindowOmniT30)
		worst.DrumsWindowRoomT30 = math.Max(worst.DrumsWindowRoomT30, m.DrumsWindowRoomT30)
		worst.SpacingAsymmetry = math.Max(worst.SpacingAsymmetry, m.SpacingAsymmetry)
		worst.TotalDepth = math.Max(worst.TotalDepth, m.TotalDepth)
	}

	// Calculate relative metrics for each candidate
	for i, c := range candidates {
		m := c.Metrics
		r := RelativeScores{}

		// Helper: avoid divide by zero
		div := func(num, denom float64) float64 {
			if denom == 0 {
				if num == 0 {
					return 1.0
				}
				return 0.0
			}
			return num / denom
		}

		// Bigger is better
		r.VoxCenterToWindowT30 = div(m.VoxCenterToWindowT30, best.VoxCenterToWindowT30)
		r.VoxDoorToWindowT30 = div(m.VoxDoorToWindowT30, best.VoxDoorToWindowT30)
		r.DrumsDoorCardT30 = div(m.DrumsDoorCardT30, best.DrumsDoorCardT30)
		r.DrumsDoorOmniT30 = div(m.DrumsDoorOmniT30, best.DrumsDoorOmniT30)
		r.DrumsDoorRoomT30 = div(m.DrumsDoorRoomT30, best.DrumsDoorRoomT30)
		r.VoxDiffusion = div(m.VoxDiffusion, best.VoxDiffusion)
		// Smaller is better: score = 1.0 - ((value - best) / best)
		srel := func(val, bestval, worstval float64) float64 {
			// if bestval == 0 {
			// 	if val == 0 {
			// 		return 1.0
			// 	}
			// 	return 0.0
			// }
			// diff := val - bestval
			// return 1.0 - (diff / bestval)
			// return (bestval - val) / (bestval - worstval)
			return (worstval - val) / (worstval - bestval)
		}
		r.VoxWindowToDoorT30 = srel(m.VoxWindowToDoorT30, best.VoxWindowToDoorT30, worst.VoxWindowToDoorT30)
		r.DrumsWindowCardT30 = srel(m.DrumsWindowCardT30, best.DrumsWindowCardT30, worst.DrumsWindowCardT30)
		r.DrumsWindowOmniT30 = srel(m.DrumsWindowOmniT30, best.DrumsWindowOmniT30, worst.DrumsWindowOmniT30)
		r.DrumsWindowRoomT30 = srel(m.DrumsWindowRoomT30, best.DrumsWindowRoomT30, worst.DrumsWindowRoomT30)
		r.SpacingAsymmetry = srel(m.SpacingAsymmetry, best.SpacingAsymmetry, worst.SpacingAsymmetry)
		r.TotalDepth = srel(m.TotalDepth, best.TotalDepth, worst.TotalDepth)

		candidates[i].Relative = r
	}

	// Optionally sort by average relative score (all metrics equal weight)
	type SortableCandidate struct {
		C                Candidate
		VoxFitness       float64
		DrumDeadFitness  float64
		DrumLiveFitness  float64
		SpacingAsymmetry float64
		TotalDepth       float64
		CombinedScore    float64
	}
	var sortable []SortableCandidate
	for _, c := range candidates {
		r := c.Relative
		voxFitness := (r.VoxCenterToWindowT30 + r.VoxDoorToWindowT30*2 + r.VoxDiffusion + r.VoxWindowToDoorT30*2) / 6.0
		drumLiveFitness := (r.DrumsDoorCardT30*3 + r.DrumsDoorOmniT30 + r.DrumsWindowRoomT30*2) / 6.0
		// drumDeadFitness := (s.DrumsWindowCardT30*5 + s.DrumsWindowOmniT30*2 + s.DrumsWindowRoomT30) / 8.0
		drumDeadFitness := (r.DrumsWindowCardT30*5 + r.DrumsWindowOmniT30*2) / 7.0
		totalDepth := r.TotalDepth
		fitness := (voxFitness + drumLiveFitness + drumDeadFitness + r.SpacingAsymmetry) / 4.0
		sortable = append(sortable, SortableCandidate{
			C:                c,
			VoxFitness:       voxFitness,
			DrumDeadFitness:  drumDeadFitness,
			DrumLiveFitness:  drumLiveFitness,
			SpacingAsymmetry: r.SpacingAsymmetry,
			TotalDepth:       totalDepth,
			CombinedScore:    fitness,
		})
	}
	sort.Slice(sortable, func(i, j int) bool {
		return sortable[i].CombinedScore > sortable[j].CombinedScore
	})

	// Find max fitness for normalization
	maxVoxFitness := sortable[0].VoxFitness
	maxDrumLiveFitness := sortable[0].DrumLiveFitness
	maxDrumDeadFitness := sortable[0].DrumDeadFitness
	maxTotalDepth := sortable[0].TotalDepth

	for _, sc := range sortable[1:] {
		if sc.VoxFitness > maxVoxFitness {
			maxVoxFitness = sc.VoxFitness
		}
		if sc.DrumLiveFitness > maxDrumLiveFitness {
			maxDrumLiveFitness = sc.DrumLiveFitness
		}
		if sc.DrumDeadFitness > maxDrumDeadFitness {
			maxDrumDeadFitness = sc.DrumDeadFitness
		}
		if sc.TotalDepth > maxTotalDepth {
			maxTotalDepth = sc.TotalDepth
		}
	}

	// Attach normalized fitness to SortableCandidate (you could add a new field if you wish)
	for i, sc := range sortable {
		norm := func(val, best float64) float64 {
			if best == 0 {
				if val == 0 {
					return 1.0
				}
				return 0.0 // or you could just return val, but 0.0 is safest
			}
			return val / best
		}
		sortable[i].VoxFitness = norm(sc.VoxFitness, maxVoxFitness)
		sortable[i].DrumLiveFitness = norm(sc.DrumLiveFitness, maxDrumLiveFitness)
		sortable[i].DrumDeadFitness = norm(sc.DrumDeadFitness, maxDrumDeadFitness)
		sortable[i].TotalDepth = norm(sc.TotalDepth, maxTotalDepth)
		sortable[i].CombinedScore = (sortable[i].VoxFitness + sortable[i].DrumLiveFitness + sortable[i].DrumDeadFitness + sortable[i].SpacingAsymmetry/8 + sortable[i].TotalDepth) / 4.125
	}

	sort.Slice(sortable, func(i, j int) bool {
		return sortable[i].CombinedScore > sortable[j].CombinedScore
	})

	nCandidates := 25
	fmt.Printf("Top %d experiments (by average relative score, of %d found):\n", nCandidates, len(sortable))
	for i := 0; i < len(sortable) && i < nCandidates; i++ {
		c := sortable[i].C
		s := sortable[i].CombinedScore
		// r := c.Relative
		// m := c.Metrics
		fmt.Printf("------------------------------------------------------\n")
		fmt.Printf("%2d. Experiment: %s\n", i+1, filepath.Base(c.Path))
		fmt.Printf("    Combined Fitness: %.3f\n", s)
		fmt.Printf("    Relative Scores:\n")
		fmt.Printf("      VoxFitness:        %.3f\n", sortable[i].VoxFitness)
		fmt.Printf("      DrumLiveFitness:   %.3f\n", sortable[i].DrumLiveFitness)
		fmt.Printf("      DrumDeadFitness:   %.3f\n", sortable[i].DrumDeadFitness)
		fmt.Printf("      SpacingAsymmetry:  %.3f\n", sortable[i].SpacingAsymmetry)
		fmt.Printf("      TotalDepth:        %.3f\n", sortable[i].TotalDepth)
		fmt.Println("")
		//
		fmt.Printf("    Parameters:\n")
		fmt.Printf("      l_num_reflectors:    %.3f\n", c.Params.LNumReflectors)
		fmt.Printf("      l_reflector_angle:   %.3f\n", c.Params.LReflectorAngle)
		fmt.Printf("      l_reflector_depth:   %.3f\n", c.Params.LReflectorDepth)
		fmt.Printf("      l_start_offset:      %.3f\n", c.Params.LStartOffset)
		fmt.Printf("      l_finish_offset:     %.3f\n", c.Params.LFinishOffset)
		fmt.Printf("      r_num_reflectors:    %.3f\n", c.Params.RNumReflectors)
		fmt.Printf("      r_reflector_angle:   %.3f\n", c.Params.RReflectorAngle)
		fmt.Printf("      r_reflector_depth:   %.3f\n", c.Params.RReflectorDepth)
		fmt.Printf("      r_start_offset:      %.3f\n", c.Params.RStartOffset)
		fmt.Printf("      r_finish_offset:     %.3f\n", c.Params.RFinishOffset)
		fmt.Println("")
		// fmt.Printf("    Relative Sub-metrics:\n")
		// fmt.Printf("      VoxCenterToWindowT30: %.3f\n", r.VoxCenterToWindowT30)
		// fmt.Printf("      VoxDoorToWindowT30:   %.3f\n", r.VoxDoorToWindowT30)
		// fmt.Printf("      DrumsDoorCardT30:     %.3f\n", r.DrumsDoorCardT30)
		// fmt.Printf("      DrumsDoorOmniT30:     %.3f\n", r.DrumsDoorOmniT30)
		// fmt.Printf("      DrumsDoorRoomT30:     %.3f\n", r.DrumsDoorRoomT30)
		// fmt.Printf("      VoxDiffusion:         %.3f\n", r.VoxDiffusion)
		// fmt.Printf("      VoxWindowToDoorT30:   %.3f\n", r.VoxWindowToDoorT30)
		// fmt.Printf("      DrumsWindowCardT30:   %.3f\n", r.DrumsWindowCardT30)
		// fmt.Printf("      DrumsWindowOmniT30:   %.3f\n", r.DrumsWindowOmniT30)
		// fmt.Printf("      DrumsWindowRoomT30:   %.3f\n", r.DrumsWindowRoomT30)
		// fmt.Printf("      SpacingAsymmetry:     %.3f\n", r.SpacingAsymmetry)
		// fmt.Println("")
		// fmt.Printf("    Raw Metrics:\n")
		// fmt.Printf("      VoxCenterToWindowT30: %.3f\n", m.VoxCenterToWindowT30)
		// fmt.Printf("      VoxDoorToWindowT30:   %.3f\n", m.VoxDoorToWindowT30)
		// fmt.Printf("      DrumsDoorCardT30:     %.3f\n", m.DrumsDoorCardT30)
		// fmt.Printf("      DrumsDoorOmniT30:     %.3f\n", m.DrumsDoorOmniT30)
		// fmt.Printf("      DrumsDoorRoomT30:     %.3f\n", m.DrumsDoorRoomT30)
		// fmt.Printf("      VoxDiffusion:         %.3f\n", m.VoxDiffusion)
		// fmt.Printf("      VoxWindowToDoorT30:   %.3f\n", m.VoxWindowToDoorT30)
		// fmt.Printf("      DrumsWindowCardT30:   %.3f\n", m.DrumsWindowCardT30)
		// fmt.Printf("      DrumsWindowOmniT30:   %.3f\n", m.DrumsWindowOmniT30)
		// fmt.Printf("      DrumsWindowRoomT30:   %.3f\n", m.DrumsWindowRoomT30)
		// fmt.Printf("      SpacingAsymmetry:     %.3f\n", m.SpacingAsymmetry)
		fmt.Printf("------------------------------------------------------\n\n")
	}
	fmt.Printf("Bottom %d experiments (by average relative score):\n", nCandidates)
	for i := len(sortable) - 1; i >= 0 && i >= len(sortable)-nCandidates; i-- {
		c := sortable[i].C
		s := sortable[i].CombinedScore
		// r := c.Relative
		// m := c.Metrics
		fmt.Printf("------------------------------------------------------\n")
		fmt.Printf("%2d. Experiment: %s\n", i+1, filepath.Base(c.Path))
		fmt.Printf("    Combined Fitness: %.3f\n", s)
		// fmt.Printf("    Raw Metrics:\n")
		// fmt.Printf("      VoxCenterToWindowT30: %.3f\n", m.VoxCenterToWindowT30)
		// fmt.Printf("      VoxDoorToWindowT30:   %.3f\n", m.VoxDoorToWindowT30)
		// fmt.Printf("      DrumsDoorCardT30:     %.3f\n", m.DrumsDoorCardT30)
		// fmt.Printf("      DrumsDoorOmniT30:     %.3f\n", m.DrumsDoorOmniT30)
		// fmt.Printf("      DrumsDoorRoomT30:     %.3f\n", m.DrumsDoorRoomT30)
		// fmt.Printf("      VoxDiffusion:         %.3f\n", m.VoxDiffusion)
		// fmt.Printf("      VoxWindowToDoorT30:   %.3f\n", m.VoxWindowToDoorT30)
		// fmt.Printf("      DrumsWindowCardT30:   %.3f\n", m.DrumsWindowCardT30)
		// fmt.Printf("      DrumsWindowOmniT30:   %.3f\n", m.DrumsWindowOmniT30)
		// fmt.Printf("      DrumsWindowRoomT30:   %.3f\n", m.DrumsWindowRoomT30)
		// fmt.Printf("      SpacingAsymmetry:     %.3f\n", m.SpacingAsymmetry)
		fmt.Printf("    Relative Scores:\n")
		fmt.Printf("      VoxFitness:        %.3f\n", sortable[i].VoxFitness)
		fmt.Printf("      DrumLiveFitness:   %.3f\n", sortable[i].DrumLiveFitness)
		fmt.Printf("      DrumDeadFitness:   %.3f\n", sortable[i].DrumDeadFitness)
		fmt.Println("")
		// fmt.Printf("      VoxCenterToWindowT30: %.3f\n", r.VoxCenterToWindowT30)
		// fmt.Printf("      VoxDoorToWindowT30:   %.3f\n", r.VoxDoorToWindowT30)
		// fmt.Printf("      DrumsDoorCardT30:     %.3f\n", r.DrumsDoorCardT30)
		// fmt.Printf("      DrumsDoorOmniT30:     %.3f\n", r.DrumsDoorOmniT30)
		// fmt.Printf("      DrumsDoorRoomT30:     %.3f\n", r.DrumsDoorRoomT30)
		// fmt.Printf("      VoxDiffusion:         %.3f\n", r.VoxDiffusion)
		// fmt.Printf("      VoxWindowToDoorT30:   %.3f\n", r.VoxWindowToDoorT30)
		// fmt.Printf("      DrumsWindowCardT30:   %.3f\n", r.DrumsWindowCardT30)
		// fmt.Printf("      DrumsWindowOmniT30:   %.3f\n", r.DrumsWindowOmniT30)
		// fmt.Printf("      DrumsWindowRoomT30:   %.3f\n", r.DrumsWindowRoomT30)
		// fmt.Printf("      SpacingAsymmetry:     %.3f\n", r.SpacingAsymmetry)
		//
		fmt.Printf("    Parameters:\n")
		fmt.Printf("      l_num_reflectors:    %.3f\n", c.Params.LNumReflectors)
		fmt.Printf("      l_reflector_angle:   %.3f\n", c.Params.LReflectorAngle)
		fmt.Printf("      l_reflector_depth:   %.3f\n", c.Params.LReflectorDepth)
		fmt.Printf("      l_start_offset:      %.3f\n", c.Params.LStartOffset)
		fmt.Printf("      l_finish_offset:     %.3f\n", c.Params.LFinishOffset)
		fmt.Printf("      r_num_reflectors:    %.3f\n", c.Params.RNumReflectors)
		fmt.Printf("      r_reflector_angle:   %.3f\n", c.Params.RReflectorAngle)
		fmt.Printf("      r_reflector_depth:   %.3f\n", c.Params.RReflectorDepth)
		fmt.Printf("      r_start_offset:      %.3f\n", c.Params.RStartOffset)
		fmt.Printf("      r_finish_offset:     %.3f\n", c.Params.RFinishOffset)
		fmt.Printf("------------------------------------------------------\n\n")
	}
}

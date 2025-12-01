package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/alecthomas/kong"

	goroom "github.com/jdginn/go-recording-studio/room"
	"github.com/jdginn/go-recording-studio/room/config"
	roomConfig "github.com/jdginn/go-recording-studio/room/config"
	roomExperiment "github.com/jdginn/go-recording-studio/room/experiment"
)

const MS float64 = 1.0 / 1000.0

const SCALE float64 = 100

type ExperimentMetadata struct {
	ConfigFile string `json:"config_file"`
	OutputDir  string `json:"output_dir"`
	Timestamp  string `json:"timestamp"`
}
type (
	Result            struct{}
	ExperimentSummary struct {
		Experiment ExperimentMetadata `json:"experiment"`
		Results    map[string]Result  `json:"results"`
	}
)

var CLI struct {
	Simulate        SimulateCmd        `cmd:"" help:"Simulate T60 for a room"`
	Batch           BatchSimulateCmd   `cmd:"" help:"Run experiments in batch mode"`
	RecursiveSabine RecursiveSabineCmd `cmd:"" help:"Recursively process experiments and write sabine.json for each"`
}

type SimulateCmd struct {
	ParamsFile string `arg:"" name:"params-file" help:"path to params json file"`
	ConfigFile string `arg:"" name:"config-file" help:"path to room config file"`
	OutputDir  string `arg:"" optional:"" name:"output-dir" help:"directory to store output in"`
}

type BatchSimulateCmd struct {
	ParamsDir  string `arg:"" name:"params-dir" help:"directory containing .json experiment param files"`
	ConfigFile string `arg:"" name:"config-file" help:"path to room config file"`
	OutputFile string `arg:"" name:"output-file" help:"where to write sabine results"`
}

type RecursiveSabineCmd struct {
	RootDir    string `arg:"" name:"root-dir" help:"root directory to recursively scan for experiments"`
	ConfigFile string `arg:"" name:"config-file" help:"path to room config file"`
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

type Absorber struct {
	Area     float64
	Material goroom.Material
}

const (
	ROOM_LENGTH = 4.74
	// REFLECTOR_HEIGHT = 1.64
	REFLECTOR_HEIGHT = 1.8
	GAP_HEIGHT       = 0.4
	// GAP_HEIGHT = 0.0
)

// Get the material with the closest depth less than or equal to the provided depth
func getMaterialByDepth(materials config.Materials, depth float64) (goroom.Material, string, error) {
	var closestDepth float64 = -1
	var closestMaterial goroom.Material
	var closestMaterialName string

	for name, mat := range materials.Inline {
		if mat.Depth < 0 {
			continue
		}
		if mat.Depth <= depth {
			if closestDepth == -1 || mat.Depth > closestDepth {
				closestDepth = mat.Depth
				closestMaterial = goroom.NewMaterial(mat.Absorption)
				closestMaterialName = name
			}
		}
	}
	if closestDepth == -1 {
		return goroom.Material{}, "", fmt.Errorf("no material found with depth less than or equal to %.2f", depth)
	}
	return closestMaterial, closestMaterialName, nil
}

// For the given side, calculate the effective absorbers based on the reflector configuration
//
// Effective absorbers represent the total area of a given material type present in the absorbers on that side
func calculateEffectiveAbsorbersSide(materials config.Materials, numReflectors, reflectorAngle, reflectorDepth, startOffset, finishOffset float64) []Absorber {
	absorberAreas := map[string]float64{}
	absorberMaterials := map[string]goroom.Material{}
	absorberKeys := []string{}

	// Convert offsets from cm to m
	startOffset = startOffset / 100
	finishOffset = finishOffset / 100
	reflectorDepth = reflectorDepth / 100

	const SAMPLE_SIZE = 0.01
	// fmt.Printf("ROOM_LENGTH: %.2f m\n", ROOM_LENGTH)
	// fmt.Printf("startOffset: %.2f m\n", startOffset)
	// fmt.Printf("finishOffset: %.2f m\n", finishOffset)
	// fmt.Printf("reflectorDepth: %.2f m\n", reflectorDepth)
	systemLength := ROOM_LENGTH - startOffset - finishOffset
	reflectorAngleRad := reflectorAngle * (math.Pi / 180.0)
	oneReflectorLength := reflectorDepth / math.Tan(reflectorAngleRad)
	allAbsorbersLength := systemLength - (oneReflectorLength * numReflectors)
	oneAbsorberLength := allAbsorbersLength / (numReflectors)
	absorberAngle := math.Atan(reflectorDepth / oneAbsorberLength)
	absorberSteps := int(oneAbsorberLength / SAMPLE_SIZE)
	// fmt.Printf("reflectorAngle: %.2f degrees\n", reflectorAngle)
	// fmt.Printf("numReflectors: %.1f\n", numReflectors)
	// fmt.Printf("systemLength: %.2f m\n", systemLength)
	// fmt.Printf("oneReflectorLength: %.2f m\n", oneReflectorLength)
	// fmt.Printf("allAbsorbersLength: %.2f m\n", allAbsorbersLength)
	// fmt.Printf("oneAbsorberLength: %.2f m\n", oneAbsorberLength)
	// fmt.Printf("absorberAngle: %.2f degrees\n", absorberAngle*(180.0/math.Pi))
	// fmt.Printf("steps per absorber: %d\n", steps)
	for i := 0; i < absorberSteps; i++ {
		depthAtStep := float64(i+1) * SAMPLE_SIZE * math.Tan(absorberAngle)
		material, name, err := getMaterialByDepth(materials, depthAtStep*100)
		if err != nil {
			panic(err)
		}
		if _, ok := absorberAreas[name]; !ok {
			absorberAreas[name] = 0
			absorberKeys = append(absorberKeys, name)
			absorberMaterials[name] = material
		}
		absorberAreas[name] += SAMPLE_SIZE * REFLECTOR_HEIGHT * numReflectors
	}
	reflectorSteps := int(oneReflectorLength / SAMPLE_SIZE)
	for i := 0; i < reflectorSteps; i++ {
		depthAtStep := float64(i+1) * SAMPLE_SIZE * math.Tan(reflectorAngleRad)
		material, name, err := getMaterialByDepth(materials, depthAtStep*100)
		if err != nil {
			panic(err)
		}
		if _, ok := absorberAreas[name]; !ok {
			absorberAreas[name] = 0
			absorberKeys = append(absorberKeys, name)
			absorberMaterials[name] = material
		}
		absorberAreas[name] += SAMPLE_SIZE * GAP_HEIGHT * numReflectors
	}
	effectiveAbsorbers := []Absorber{}
	for _, name := range absorberKeys {
		effectiveAbsorbers = append(effectiveAbsorbers, Absorber{
			Area:     absorberAreas[name],
			Material: absorberMaterials[name],
		})
	}
	return effectiveAbsorbers
}

func calculateEffectiveAbsorbers(materials config.Materials, params ExperimentParams) []Absorber {
	// Calculate left side absorbers
	leftAbsorbers := calculateEffectiveAbsorbersSide(
		materials,
		params.LNumReflectors,
		params.LReflectorAngle,
		params.LReflectorDepth,
		params.LStartOffset,
		params.LFinishOffset,
	)
	rightAbsorbers := calculateEffectiveAbsorbersSide(
		materials,
		params.RNumReflectors,
		params.RReflectorAngle,
		params.RReflectorDepth,
		params.RStartOffset,
		params.RFinishOffset,
	)
	return append(leftAbsorbers, rightAbsorbers...)
}

type sabineParameters struct {
	volume           float64
	absorbers        []Absorber
	replacedSurfaces string
}

func sabine(r *goroom.Room, absorbers []Absorber, replacedSurfaces []string, freq float64) (float64, error) {
	const SABINE = 0.161

	sabines := 0.0
	for _, tri := range r.M.Triangles {
		if slices.Contains(replacedSurfaces, tri.(*goroom.Triangle).Surface.Name) {
			continue
		}
		if r.IsInnermost(tri) {
			sabines += tri.(*goroom.Triangle).Surface.Material.Alpha(freq) * tri.T().Area()
			if tri.T().Area() < 0. {
				fmt.Printf("Negative area triangle detected: %+v\n", tri.T())
			}
			if tri.(*goroom.Triangle).Surface.Material.Alpha(freq) < 0. {
				fmt.Printf("Negative alpha detected: %+v\n", tri.T())
			}
		}
	}
	for _, absorber := range absorbers {
		sabines += absorber.Material.Alpha(freq) * absorber.Area
	}
	v, err := r.Volume()
	if err != nil {
		return 0, err
	}
	return SABINE * v / sabines, nil
}

func (c SimulateCmd) Run() (err error) {
	paramsPath := c.ParamsFile
	paramsData, err := os.ReadFile(paramsPath)
	if err != nil {
		fmt.Printf("Error reading %s: %v\n", paramsPath, err)
		return err
	}
	var params ExperimentParams
	if err := json.Unmarshal(paramsData, &params); err != nil {
		fmt.Printf("Bad params json in %s: %v\n", paramsPath, err)
		return err
	}

	config, err := roomConfig.LoadFromFile(c.ConfigFile, roomConfig.LoadOptions{
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
	if err := expDir.CopyConfigFile(c.ConfigFile); err != nil {
		return fmt.Errorf("copying config file: %w", err)
	}
	if err := expDir.CopyConfigFile(c.ParamsFile); err != nil {
		return fmt.Errorf("copying param file: %w", err)
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
		// Sanity check
		t60r, err := sabine(room, []Absorber{}, []string{}, freq)
		if math.Abs(t60-t60r) > 0.01 {
			fmt.Printf("Discrepancy in Sabine T60 calculation at %.0f Hz: %.2f vs %.2f\n", freq, t60, t60r)
		}
		if err != nil {
			return err
		}
		t60Sabine[i] = t60
	}

	t60SabineWithAbsorbers := make([]float64, len(testFrequencies))
	effectiveAbsorbers := calculateEffectiveAbsorbers(config.Materials, params)
	replacedSurfaces := []string{"Street Wall", "Hall wall"}
	for i, freq := range testFrequencies {
		t60, err := sabine(room, effectiveAbsorbers, replacedSurfaces, freq)
		if err != nil {
			return err
		}
		t60SabineWithAbsorbers[i] = t60
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
	fmt.Println("Sabine with absorbers:")
	for i, freq := range testFrequencies {
		fmt.Printf("\t%.0fHz: %.2f ms\n", freq, t60SabineWithAbsorbers[i]/MS)
	}
	fmt.Printf("\n")
	fmt.Println("Eyring:")
	for i, freq := range testFrequencies {
		fmt.Printf("\t%.0fHz: %.2f ms\n", freq, t60Eyring[i]/MS)
	}
	fmt.Printf("\n")

	return nil
}

type ExperimentResult struct {
	ExperimentName         string    `json:"experiment_name"`
	T60SabineWithAbsorbers []float64 `json:"t60_sabine_with_absorbers"`
	Frequencies            []float64 `json:"frequencies"`
}

func (c BatchSimulateCmd) Run() error {
	files, err := filepath.Glob(filepath.Join(c.ParamsDir, "*.json"))
	if err != nil {
		return fmt.Errorf("reading param dir: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no .json param files found in %s", c.ParamsDir)
	}
	results := []ExperimentResult{}

	config, err := roomConfig.LoadFromFile(c.ConfigFile, roomConfig.LoadOptions{
		ValidateImmediately: false,
		ResolvePaths:        true,
		MergeFiles:          true,
	})
	if err != nil {
		return err
	}

	for _, paramPath := range files {
		var params ExperimentParams
		data, err := os.ReadFile(paramPath)
		if err != nil {
			fmt.Printf("Error reading %s: %v\n", paramPath, err)
			continue
		}
		if err := json.Unmarshal(data, &params); err != nil {
			fmt.Printf("Bad json in %s: %v\n", paramPath, err)
			continue
		}

		room, _, err := goroom.NewFrom3MF(config.Input.Mesh.Path, config.SurfaceAssignmentMap())
		if err != nil {
			fmt.Printf("Room instance error for %s: %v\n", paramPath, err)
			continue
		}

		frequencies := []float64{63, 125, 250, 500, 1000, 2000, 4000, 8000, 16000}
		if len(config.Decay.FreqCorners) > 0 {
			frequencies = make([]float64, len(config.Decay.FreqCorners))
			for i, fc := range config.Decay.FreqCorners {
				frequencies[i] = float64(fc)
			}
		}

		effectiveAbsorbers := calculateEffectiveAbsorbers(config.Materials, params)
		replacedSurfaces := []string{"Street Wall", "Hall wall"}

		t60SabineWithAbsorbers := make([]float64, len(frequencies))
		for i, freq := range frequencies {
			t60, err := sabine(room, effectiveAbsorbers, replacedSurfaces, freq)
			if err != nil {
				fmt.Printf("Sabine calc error %s: %v\n", paramPath, err)
				t60SabineWithAbsorbers[i] = 0 // Or skip
				continue
			}
			t60SabineWithAbsorbers[i] = t60
		}

		// Get experiment name (use the filename or add a field to params)
		experimentName := filepath.Base(paramPath)

		results = append(results, ExperimentResult{
			ExperimentName:         experimentName,
			T60SabineWithAbsorbers: t60SabineWithAbsorbers,
			Frequencies:            frequencies,
		})
	}

	// Write results to output file
	out, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling batch results: %w", err)
	}
	if err := os.WriteFile(c.OutputFile, out, 0644); err != nil {
		return fmt.Errorf("writing batch results: %w", err)
	}
	fmt.Printf("Batch simulation complete. Results written to %s\n", c.OutputFile)
	return nil
}

func (c RecursiveSabineCmd) Run() error {
	config, err := roomConfig.LoadFromFile(c.ConfigFile, roomConfig.LoadOptions{
		ValidateImmediately: false,
		ResolvePaths:        true,
		MergeFiles:          true,
	})
	if err != nil {
		return err
	}

	frequencies := []float64{63, 125, 250, 500, 1000, 2000, 4000, 8000, 16000}
	if len(config.Decay.FreqCorners) > 0 {
		frequencies = make([]float64, len(config.Decay.FreqCorners))
		for i, fc := range config.Decay.FreqCorners {
			frequencies[i] = float64(fc)
		}
	}

	err = filepath.Walk(c.RootDir, func(dir string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		foundSummary := false
		var paramFiles []string
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if name == "summary.json" {
				foundSummary = true
			} else if name == "annotations.json" {
				// Ignore
				continue
			} else if strings.HasSuffix(name, ".json") {
				paramFiles = append(paramFiles, name)
			}
		}
		if foundSummary && len(paramFiles) > 0 {
			for _, paramFile := range paramFiles {
				paramPath := filepath.Join(dir, paramFile)
				var params ExperimentParams
				data, err := os.ReadFile(paramPath)
				if err != nil {
					fmt.Printf("Error reading %s: %v\n", paramPath, err)
					continue
				}
				if err := json.Unmarshal(data, &params); err != nil {
					fmt.Printf("Bad json in %s: %v\n", paramPath, err)
					continue
				}
				room, _, err := goroom.NewFrom3MF(config.Input.Mesh.Path, config.SurfaceAssignmentMap())
				if err != nil {
					fmt.Printf("Room instance error for %s: %v\n", paramPath, err)
					continue
				}
				effectiveAbsorbers := calculateEffectiveAbsorbers(config.Materials, params)
				replacedSurfaces := []string{"Street Wall", "Hall wall"}
				t60SabineWithAbsorbers := make([]float64, len(frequencies))
				for i, freq := range frequencies {
					t60, err := sabine(room, effectiveAbsorbers, replacedSurfaces, freq)
					if err != nil {
						fmt.Printf("Sabine calc error %s: %v\n", paramPath, err)
						t60SabineWithAbsorbers[i] = 0
						continue
					}
					t60SabineWithAbsorbers[i] = t60
				}
				experimentName := strings.TrimSuffix(paramFile, ".json")
				result := ExperimentResult{
					ExperimentName:         experimentName,
					T60SabineWithAbsorbers: t60SabineWithAbsorbers,
					Frequencies:            frequencies,
				}
				out, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					fmt.Printf("Error marshalling result for %s: %v\n", paramFile, err)
					continue
				}
				sabinePath := filepath.Join(dir, "sabine.json")
				if err := os.WriteFile(sabinePath, out, 0644); err != nil {
					fmt.Printf("Error writing sabine.json in %s: %v\n", dir, err)
					continue
				}
				fmt.Printf("Wrote %s\n", sabinePath)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error during recursive scan: %w", err)
	}
	fmt.Println("Recursive sabine processing complete.")
	return nil
}

func main() {
	ctx := kong.Parse(&CLI)
	err := ctx.Run()
	if err != nil {
		log.Fatal(err)
	}
}

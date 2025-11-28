package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"slices"

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
	Simulate SimulateCmd `cmd:"" help:"Simulate T60 for a room"`
}

type SimulateCmd struct {
	ParamsFile string `arg:"" name:"params-file" help:"path to params json file"`
	ConfigFile string `arg:"" name:"config-file" help:"path to room config file"`
	OutputDir  string `arg:"" optional:"" name:"output-dir" help:"directory to store output in"`
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
	ROOM_LENGTH      = 4.74
	REFLECTOR_HEIGHT = 1.64
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
	steps := int(oneAbsorberLength / SAMPLE_SIZE)
	// fmt.Printf("reflectorAngle: %.2f degrees\n", reflectorAngle)
	// fmt.Printf("numReflectors: %.1f\n", numReflectors)
	// fmt.Printf("systemLength: %.2f m\n", systemLength)
	// fmt.Printf("oneReflectorLength: %.2f m\n", oneReflectorLength)
	// fmt.Printf("allAbsorbersLength: %.2f m\n", allAbsorbersLength)
	// fmt.Printf("oneAbsorberLength: %.2f m\n", oneAbsorberLength)
	// fmt.Printf("absorberAngle: %.2f degrees\n", absorberAngle*(180.0/math.Pi))
	// fmt.Printf("steps per absorber: %d\n", steps)
	for i := 0; i < steps; i++ {
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

func main() {
	ctx := kong.Parse(&CLI)
	err := ctx.Run()
	if err != nil {
		log.Fatal(err)
	}
}

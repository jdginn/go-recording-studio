package main

import (
	"fmt"
	"log"

	"github.com/alecthomas/kong"
	goroom "github.com/jdginn/go-recording-studio/room"
	roomConfig "github.com/jdginn/go-recording-studio/room/config"
	roomExperiment "github.com/jdginn/go-recording-studio/room/experiment"
)

const MS float64 = 1.0 / 1000.0

const SCALE float64 = 100

var testFrequencies = []float64{
	63, 125, 250, 500, 1000, 2000, 4000, 8000, 16000,
}

var CLI struct {
	Simulate SimulateCmd `cmd:"" help:"Simulate T60 for a room"`
}

type SimulateCmd struct {
	Config    string `arg:"" name:"config" help:"config file to simulate"`
	OutputDir string `arg:"" optional:"" name:"output-dir" help:"directory to store output in"`
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

	vol, err := room.Volume()
	fmt.Printf("Room volume: %.2f m³\n\n", vol)

	t60s := make([]float64, len(testFrequencies))
	for i, freq := range testFrequencies {
		t60, err := room.T60Eyring(freq)
		if err != nil {
			return err
		}
		t60s[i] = t60
	}

	for i, t60 := range t60s {
		fmt.Printf("T60 at %.0f Hz: %.2f ms\n", testFrequencies[i], t60/MS)
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

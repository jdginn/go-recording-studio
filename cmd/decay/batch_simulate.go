package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	var inputDir, configPath string
	flag.StringVar(&inputDir, "input-dir", "", "Directory containing .3mf/.json pairs")
	flag.StringVar(&configPath, "config", "", "Path to config YAML file")
	flag.Parse()

	// if inputDir == "" || configPath == "" {
	// 	fmt.Println("Usage: batch_simulate --input-dir DIR --config room.yaml")
	// 	os.Exit(1)
	// }

	files, err := os.ReadDir(inputDir)
	if err != nil {
		fmt.Printf("Failed to read input dir: %v\n", err)
		os.Exit(1)
	}

	for _, f := range files {
		name := f.Name()
		if !strings.HasSuffix(name, ".3mf") {
			continue
		}

		base := strings.TrimSuffix(name, ".3mf")
		jsonPath := filepath.Join(inputDir, base+".json")
		threePath := filepath.Join(inputDir, name)

		fmt.Printf("Checking if %s exists\n", filepath.Join(inputDir, base+"-results"))
		// Check if results dir already exists. If so, skip.
		info, err := os.Stat(filepath.Join(inputDir, base+"-results"))
		if err == nil && info.IsDir() {
			fmt.Printf("Skipping %s: results already exist\n", name)
			continue
		}

		// Check that JSON exists
		if _, err := os.Stat(jsonPath); err != nil {
			fmt.Printf("Skipping %s: no matching json found\n", name)
			continue
		}

		outputDir := filepath.Join(inputDir, base+"-results")
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			fmt.Printf("Could not create output dir: %v\n", err)
			continue
		}

		// Copy the .3mf and .json into output dir
		copyFile(threePath, filepath.Join(outputDir, name))
		copyFile(jsonPath, filepath.Join(outputDir, base+".json"))

		// Run simulation
		cmd := exec.Command(
			"./main", "simulate",
			configPath,
			outputDir,
			"--mesh", filepath.Join(inputDir, name),
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Printf("Running simulation for: %s\n", base)
		if err := cmd.Run(); err != nil {
			fmt.Printf("Error running simulation on %s: %v\n", name, err)
		}
	}
}

// Copy file utility
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

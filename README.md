# go-recording-studio

An acoustic simulation tool for optimizing speaker and listening positions in recording studios using the Reflection-Free Zone (RFZ) design approach.

## What it does

This tool simulates high-frequency acoustic behavior in recording studios, helping studio designers:

- Find optimal speaker and listening positions
- Identify problematic reflections that might compromise the RFZ
- Determine which room surfaces need acoustic treatment
- Calculate Initial Time Delay (ITD) and other key acoustic parameters

## How it works

1. Import a 3D model of your room (in .3mf format)
2. Define materials and their properties for surfaces in the room
3. Run simulations to analyze reflection paths
4. Export detailed annotations for visualization with [psalm](https://github.com/jdginn/psalm).

The tool focuses on frequencies above the Schroeder frequency (typically most accurate above 2x Schroeder). Low-frequency behavior requires separate consideration and treatment.

## Background

### The RFZ Approach

The Reflection-Free Zone (RFZ) concept allows for precise acoustic optimization while preserving a room's natural characteristics. Unlike approaches that try to eliminate all reflections, RFZ focuses on creating a controlled listening environment within a specific zone around the mixing position, while relaxing and minimizing disruption to the rest of the room.

Key aspects of RFZ design:

- Only the zone around the listening position needs to meet strict acoustic criteria
- Sound from the main speakers is carefully controlled
- Other sounds (voices, instruments) can behave naturally in the room
- Speakers are typically flush-mounted in the front wall

This selective approach allows studio designers to:

1. Minimize modifications to the room's original surfaces
2. Maintain a natural, pleasant acoustic environment
3. Achieve professional acoustic standards within the listening zone
4. Preserve usable space in the room

### Why Simulation Matters

RFZ design requires high precision. Small changes in speaker placement, listening position, or surface treatment can significantly impact performance. Precision is especially important if we want a minimally-invasive solution for the acoustics. Traditional trial-and-error approaches are impractical because:

- Moving flush-mounted speakers is extremely costly
- Testing multiple surface treatment configurations is time-consuming
- Some changes (like speaker placement) must be decided before construction

### What We're Optimizing

The primary goal is to maximize the Initial Time Delay (ITD) gap - the time between when direct sound from the speakers reaches the listening position and when the first reflection arrives. This requires:

1. Tracing all possible paths sound might take from the speakers to the RFZ
2. Calculating the timing and intensity of each reflection
3. Identifying which surfaces are causing problematic reflections
4. Finding speaker and listening positions that maximize ITD

The RFZ is modeled as a hemisphere centered at the engineer's seated position (the "sweet spot"). We use a hemisphere rather than a full sphere because engineers typically work between seated and standing positions, but rarely below the seated position. The radius of this hemisphere is configurable, with an important tradeoff: larger RFZ radii tend to result in shorter ITDs, as a larger zone increases the probability of capturing reflections. This means that maximizing the size of the RFZ must be balanced against achieving the desired acoustic performance.

### Why Use 3MF Files

We use the 3MF format because:

1. It preserves surface names from CAD software, which is crucial for material mapping
2. It handles complex, irregular room geometries
3. Most CAD tools can export to 3MF
4. It maintains the connection between:
   - Surface names in your CAD software
   - Material definitions in your configuration
   - Acoustic properties in the simulation

### High-Frequency Focus

This tool uses ray tracing to model acoustic reflections, which is valid for frequencies above the Schroeder frequency (typically most accurate above 2x Schroeder). Below this frequency, room acoustics are dominated by modal behavior that requires different analysis methods.

### Visualization and Optimization

While go-recording-studio handles the acoustic simulation, visualization and optimization are handled by a companion tool called [psalm](https://github.com/jdginn/psalm). This separation allows:

- Efficient simulation in Go
- Rich visualization capabilities using Python's 3D tools
- Integration with machine learning for parameter optimization

TODO: Add example visualization showing reflection paths and RFZ

## Getting Started

### Configuration

Complete configuration details are documented in `experiment_schema.md`. The schema includes:

- Room and material definitions
- Speaker configurations (including directivity patterns)
- Listening position parameters
- Simulation settings

For room orientation, while the tool supports configurable reference positions and normals, the most tested configuration is:

```yaml
reference_position: [0, <half of front wall width>, 0.0]
reference_normal: [1, 0, 0]
```

### Performance

- Typical simulation (500,000 rays, 100ms) runs in seconds on modern hardware
- Recommended ray counts: 100,000 - 2,000,000
- Tip: Increase ray count until results stabilize
- Single-threaded, CPU-only implementation

### Targets and Interpretation

What is a good result?

- [EBU 3276](https://tech.ebu.ch/docs/tech/tech3276.pdf) stipulates: No reflections > -10dB within first 15ms
- For our studio we are targeting: No reflections > -20dB within first 30ms

See [EBU 3276](https://tech.ebu.ch/docs/tech/tech3276.pdf) for complete specifications.

### Common Usage Notes

#### Room Modifications

- Physical room changes require new 3MF exports
- Speaker and listening positions can be adjusted in the YAML config
- Surface materials can be modified in the YAML config

#### Material Mapping

- Surface names in the YAML must exactly match names in the 3MF
- Unmatched surfaces use the default material
- If no default is specified, brick properties are used

#### Technical Details

- ITD calculations use first reflection only
- Summary includes `avg_gain_5ms` for comparing early reflection energy between simulations
- Speaker directivity patterns are configurable in the YAML
- Non-watertight models usually work fine unless you see "non-terminating ray" errors

TODO: Document summary.json format and fields
TODO: Add detailed psalm workflow documentation once available

## Installation

```bash
go get github.com/jdginn/go-recording-studio
```

## Workflow

### 1. Prepare Your Room Model

1. Create your room model in CAD software (tested with Autodesk Fusion)
2. Name each surface that will have distinct acoustic properties
3. Export as .3mf, ensuring surface names are preserved

### 2. Configure Your Simulation

Create a YAML configuration file defining:

- Path to your .3mf model
- Material properties
- Surface-to-material mappings
- Simulation parameters

Example configuration:

```yaml
input:
  mesh:
    path: "room.3mf"

materials:
  inline:
    brick:
      absorption:
        { 125: 0.05, 250: 0.04, 500: 0.02, 1000: 0.04, 2000: 0.05, 4000: 0.05 }

surface_assignments:
  inline:
    default: "brick"
    "Front Wall": "absorber"
```

See `experiment_schema.md` for complete configuration documentation.

### 3. Run Simulation

```bash
go-recording-studio simulate config.yaml
```

### 4. Analyze Results

The simulation produces several output files:

- `annotations.json`: Detailed reflection paths and points
- `summary.json`: Acoustic parameters (ITD, T60, Schroeder frequency)
- `room.stl`: Final room geometry

TODO: Add example visualization showing reflection paths and RFZ

For visualization and optimization of these results, use the [psalm tool](https://github.com/jdginn/psalm).

## Command Line Options

```
simulate [flags] <config>
  --output-dir string             Directory to store output in
  --skip-speaker-in-room-check    Skip checking whether speaker is inside room
  --skip-add-speaker-wall         Skip adding walls for speaker flush mounting
  --skip-tracing                  Skip performing ray tracing steps
  --counterfactual strings        Simulate reflections with perfect reflector surfaces
```

## Development

### Project Structure

```
.
├── main.go              # Main application entry point
├── room/               # Core room simulation package
│   ├── config/         # Configuration handling
│   └── experiment/     # Experiment management
├── experiment_schema.md # Configuration documentation
└── annotations_schema.md # Output format documentation
```

## License

MIT License (see LICENSE)

## Acknowledgments

- Uses the [fogleman/pt](https://github.com/fogleman/pt) package for ray tracing
- Uses [kong](https://github.com/alecthomas/kong) for CLI argument parsing


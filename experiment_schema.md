# Acoustic Room Simulation Experiment Schema

This document describes the YAML format for configuring acoustic room simulations. The schema supports defining room geometry, material properties, speaker configurations, and simulation parameters. It is designed to capture all necessary parameters for reproducible acoustic experiments.

## Overview

The configuration consists of several main sections:

- Core room and material definitions
- Speaker and listening position configurations
- Simulation parameters and runtime flags
- Automatically generated metadata

All numeric measurements are in SI units (meters, degrees) unless otherwise specified.

## Schema Version: 1.0

## File Format

The configuration must be provided in YAML format. File paths within the configuration can be absolute or relative and support bash-style path expansion (., .., ~).

## Sections

### Metadata

System-generated information about the experiment that was run. This is included to help recreate an experiment to debug it. This section is only populated by the simulator, and is not meant to be consumed.

**Required Fields:**

- `timestamp` (string): UTC timestamp of execution in YYYY-MM-DD HH:MM:SS format
- `git_commit` (string): Git commit hash of the codebase at execution time

Example:

```yaml
metadata:
  timestamp: "2025-03-13 15:00:06"
  git_commit: "abc123..."
```

### Input Mesh

Defines the 3D geometry of the room to be simulated.

**Required Fields:**

- `path` (string): Path to 3MF file containing room geometry

Example:

```yaml
input:
  mesh:
    path: "path/to/room.3mf"
```

### Materials

Defines acoustic properties of materials used in the simulation. Materials can be defined inline or loaded from an external file.

**Required Fields:**

- Either `inline` or `from_file` must be specified:
  - `inline` (map): Dictionary of material definitions, where each material has:
    - `absorption` (map): Map of absorption coefficients at specific frequencies where the absorption coefficient is between 0.0 (perfect reflector) and 1.0 (perfect absorber).
  - `from_file` (string): Path to JSON file containing material definitions

If both are specified, the contents of `inline` and `from_file` are merged with the values in `inline` taking precedence.

Example:

```yaml
materials:
  inline:
    brick:
      absorption:
        { 125: 0.05, 250: 0.04, 500: 0.02, 1000: 0.04, 2000: 0.05, 4000: 0.05 }
    wood:
      absorption:
        { 125: 0.25, 250: 0.05, 500: 0.04, 1000: 0.03, 2000: 0.03, 4000: 0.02 }
  # AND/OR
  from_file: "path/to/materials.json"
```

### Surface Assignments

Maps room surfaces to defined materials.

**Required Fields:**

- Either `inline` or `from_file` must be specified:
  - `inline` (map): Dictionary of surface assignments, including:
    - `default` (string): Material name to use for unmapped surfaces
    - Additional key-value pairs mapping surface names to material names
  - `from_file` (string): Path to JSON file containing surface assignments

If both are specified, the contents of `inline` and `from_file` are merged with the values in `inline` taking precedence.

Example:

```yaml
surface_assignments:
  inline:
    default: "brick"
    "Floor": "wood"
    "Front A": "gypsum"
  # AND/OR
  from_file: "path/to/assignments.json"
```

### Speaker Configuration

Defines speaker physical dimensions, placement offsets, and directivity patterns.

Offset describes the distance of the acoustic center of the speaker from the bottom left corner of the front face of the speaker.
The acoustic center is always assumed to be on the front face of the speaker.

The directivity patterns assume an implicit 0dB attenuation at 0 degrees.

**Required Fields:**

- `model` (string): Speaker model identifier
- `dimensions` (object): Physical dimensions in meters
  - `x` (float): Width
  - `y` (float): Height
  - `z` (float): Depth
- `offset` (object): Placement offsets in meters
  - `y` (float): Vertical offset
  - `z` (float): Depth offset
- `directivity` (object): Directivity patterns
  - `horizontal` (map): Mapping of angles (-180 to 180 degrees) to attenuation values (dB)
  - `vertical` (map): Mapping of angles (-180 to 180 degrees) to attenuation values (dB)

Example:

```yaml
speaker:
  model: "MUM8"
  dimensions:
    x: 0.38
    y: 0.256
    z: 0.52
  offset:
    y: 0.096
    z: 0.412
  directivity:
    horizontal:
      0: 0
      30: -1
    vertical:
      0: 0
      30: 0
```

### Listening Triangle

Defines the geometric configuration of the listening position and orientation.

**Required Fields:**

- `distance_from_front` (float): Distance of acoustic center of the loudspeakers from front wall, in meters.
- `distance_from_center` (float): Distance of acoustic centers of loudspeakers from center of the room, in meters. This distance is perpendicular to distance_from_front.
- `source_height` (float): Height of acoustic centers of loudspeakers above the reference_position, in meters. By default, the reference_position is on the floor.
- `listen_height` (float): Height of listening position in meters above the reference_position, in meters. By default, the reference_position is on the floor.

**Optional Fields:**

- `reference_normal` (array of 3 floats): Unit vector defining orientation, where the vector points from the front wall toward the listener. This vector defines the front wall. If not specified, defaults to [1, 0, 0]
- `reference_position` (array of 3 floats): Coordinate of a point along the edge of the front wall and floor, halfway along the front wall. This point defines the acoustic middle of the room. The listener will be located along this axis. Defaults to [0, 2.37, 0]

Example:

```yaml
listening_triangle:
  reference_position: [0, 2.37, 0.0]
  reference_normal: [1, 0, 0]
  distance_from_front: 0.516
  distance_from_center: 1.352
  source_height: 1.7
  listen_height: 1.4
```

### Simulation Parameters

Controls the acoustic simulation behavior and accuracy.

**Required Fields:**

- `rfz_radius` (float): Reflection Free Zone radius in meters. Any reflection that arrives within this entire sphere is considered to be a problematic reflection.
  In other words, the entirety of this sphere will be ensured to be acoustically "good" to whatever standard of goodness this simulation demonstrates.
- `shot_count` (integer): Number of rays to simulate. More shots produces a better simulation, but takes longer.
- `shot_angle_range` (float): Angular range in degrees along which rays are shot. Defaults to 180 degrees.
- `order` (integer): Maximum sequence of reflections to simulate. Larger order setting may take longer but be more accurate. Similar overall effect to time_threshold_ms.
- `gain_threshold_db` (float): Threshold below which we stop simulating. The further this is from 0, the longer the simulation will take.
- `time_threshold_ms` (float): Threshold of resulting ITD after which we stop simulating. The larger this is, the longer the simulation will take.

Example:

```yaml
simulation:
  rfz_radius: 0.5
  shot_count: 100000
  shot_angle_range: 180
  order: 10
  gain_threshold_db: -15
  time_threshold_ms: 100
```

### Runtime Flags

Configuration flags that can be overridden via command line.

**Required Fields:**

- `skip_speaker_in_room_check` (boolean): Skip validation of speaker placement that verifies speaker does not collide with walls
- `skip_add_speaker_wall` (boolean): Skip addition of speaker wall

Example:

```yaml
flags:
  skip_speaker_in_room_check: false
  skip_add_speaker_wall: false
```

## Validation Rules

- All numeric values must be finite and non-negative where applicable
- Absorption coefficients must be between 0.0 and 1.0
- Reference normal must be a unit vector
- Angles in directivity maps must be between -180 and 180 degrees
- Material names in surface assignments must match defined materials
- Surface assignments must include a default material

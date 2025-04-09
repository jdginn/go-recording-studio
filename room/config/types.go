package config

import (
	"fmt"

	room "github.com/jdginn/go-recording-studio/room"
)

// ExperimentConfig represents the complete configuration for an acoustic room simulation
type ExperimentConfig struct {
	Metadata           Metadata           `yaml:"metadata"`
	Input              Input              `yaml:"input"`
	Materials          Materials          `yaml:"materials"`
	SurfaceAssignments SurfaceAssignments `yaml:"surface_assignments"`
	Speaker            Speaker            `yaml:"speaker"`
	ListeningTriangle  ListeningTriangle  `yaml:"listening_triangle"`
	Simulation         Simulation         `yaml:"simulation"`
	Flags              Flags              `yaml:"flags"`
	CeilingPanels      CeilingPanels      `yaml:"ceiling_panels"`
	WallAbsorbers      WallAbsorbers      `yaml:"wall_absorbers"`
}

func (c ExperimentConfig) SurfaceAssignmentMap() map[string]room.Material {
	assignmentMap := make(map[string]room.Material)

	for name, material := range c.SurfaceAssignments.Inline {
		if _, ok := c.Materials.Inline[material]; !ok {
			material = "default"
		}
		if _, ok := c.Materials.Inline[material]; !ok {
			panic(fmt.Sprintf("code bug: should be looking up default material but failed to find %s", material))
		}
		assignmentMap[name] = room.NewMaterial(c.Materials.Inline[material].Absorption)
	}

	return assignmentMap
}

func (c ExperimentConfig) GetSurfaceAssignment(name string) room.Material {
	mat, ok := c.Materials.Inline[name]
	if !ok {
		mat = c.Materials.Inline["default"]
	}
	return room.NewMaterial(mat.Absorption)
}

type Metadata struct {
	Timestamp string `yaml:"timestamp"` // YYYY-MM-DD HH:MM:SS in UTC
	GitCommit string `yaml:"git_commit"`
}

type Input struct {
	Mesh struct {
		Path string `yaml:"path"`
	} `yaml:"mesh"`
}

type Materials struct {
	Inline   map[string]Material `yaml:"inline,omitempty"`
	FromFile string              `yaml:"from_file,omitempty"`
}

type Material struct {
	Absorption map[float64]float64 `yaml:"absorption"`
}

type SurfaceAssignments struct {
	Inline   map[string]string `yaml:"inline,omitempty"` // surface name -> material name
	FromFile string            `yaml:"from_file,omitempty"`
}

type Speaker struct {
	Model       string            `yaml:"model"`
	Dimensions  SpeakerDimensions `yaml:"dimensions"`
	Offset      SpeakerOffset     `yaml:"offset"`
	Directivity Directivity       `yaml:"directivity"`
}

func (s Speaker) Create() room.LoudSpeakerSpec {
	return room.LoudSpeakerSpec{
		Xdim:        s.Dimensions.X,
		Ydim:        s.Dimensions.Y,
		Zdim:        s.Dimensions.Z,
		Yoff:        s.Offset.Y,
		Zoff:        s.Offset.Z,
		Directivity: room.NewDirectivity(s.Directivity.Horizontal, s.Directivity.Vertical),
	}
}

type SpeakerDimensions struct {
	X float64 `yaml:"x"` // Width in meters
	Y float64 `yaml:"y"` // Height in meters
	Z float64 `yaml:"z"` // Depth in meters
}

type SpeakerOffset struct {
	Y float64 `yaml:"y"` // Vertical offset in meters
	Z float64 `yaml:"z"` // Depth offset in meters
}

type Directivity struct {
	Horizontal map[float64]float64 `yaml:"horizontal"` // angle -> attenuation
	Vertical   map[float64]float64 `yaml:"vertical"`   // angle -> attenuation
}

type ListeningTriangle struct {
	ReferencePosition  [3]float64 `yaml:"reference_position,omitempty"`
	ReferenceNormal    [3]float64 `yaml:"reference_normal,omitempty"`
	DistanceFromFront  float64    `yaml:"distance_from_front"`
	DistanceFromCenter float64    `yaml:"distance_from_center"`
	SourceHeight       float64    `yaml:"source_height"`
	ListenHeight       float64    `yaml:"listen_height"`
}

func (lt ListeningTriangle) Create() room.ListeningTriangle {
	return room.ListeningTriangle{
		ReferencePosition: room.V(
			lt.ReferencePosition[0],
			lt.ReferencePosition[1],
			lt.ReferencePosition[2],
		),
		ReferenceNormal: room.V(
			lt.ReferenceNormal[0],
			lt.ReferenceNormal[1],
			lt.ReferenceNormal[2],
		),
		DistFromFront:  lt.DistanceFromFront,
		DistFromCenter: lt.DistanceFromCenter,
		SourceHeight:   lt.SourceHeight,
		ListenHeight:   lt.ListenHeight,
	}
}

type Simulation struct {
	RFZRadius       float64 `yaml:"rfz_radius"`
	ShotCount       int     `yaml:"shot_count"`
	ShotAngleRange  float64 `yaml:"shot_angle_range"`
	Order           int     `yaml:"order"`
	GainThresholdDB float64 `yaml:"gain_threshold_db"`
	TimeThresholdMS float64 `yaml:"time_threshold_ms"`
}

type Flags struct {
	SkipSpeakerInRoomCheck bool `yaml:"skip_speaker_in_room_check"`
	SkipAddSpeakerWall     bool `yaml:"skip_add_speaker_wall"`
}

type CeilingPanels struct {
	Center *struct {
		Thickness float64 `yaml:"thickness"`
		Height    float64 `yaml:"height"`
		Width     float64 `yaml:"width"`
		XMin      float64 `yaml:"xmin"`
		XMax      float64 `yaml:"xmax"`
	} `yaml:"center,omitempty"`
	Sides *struct {
		Thickness float64 `yaml:"thickness"`
		Height    float64 `yaml:"height"`
		Width     float64 `yaml:"width"`
		Spacing   float64 `yaml:"spacing"`
		XMin      float64 `yaml:"xmin"`
		XMax      float64 `yaml:"xmax"`
	} `yaml:"sides,omitempty"`
}

type WallAbsorbers struct {
	Thickness float64            `yaml:"thickness"`
	Heights   map[string]float64 `yaml:"heights"`
}

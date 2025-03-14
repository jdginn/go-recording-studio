package config

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
	Absorption float64 `yaml:"absorption"`
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

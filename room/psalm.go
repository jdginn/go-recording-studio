package room

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/fogleman/pt/pt"
)

const (
	// Pastel Colors
	PastelRed      = "#FF6961"
	PastelOrange   = "#FFD1A1"
	PastelYellow   = "#FDFD96"
	PastelGreen    = "#77DD77"
	PastelBlue     = "#AEC6CF"
	PastelPurple   = "#CBAACB"
	PastelPink     = "#FFB7CE"
	PastelTeal     = "#99C5B3"
	PastelLavender = "#B39EB5"
	PastelPeach    = "#FFDAC1"
	PastelMint     = "#B5EAD7"
	PastelSky      = "#C1E1C1"

	// Bright Colors
	BrightRed      = "#FF4D4D"
	BrightOrange   = "#FFA64D"
	BrightYellow   = "#FFFF4D"
	BrightGreen    = "#4DFF4D"
	BrightBlue     = "#4D9EFF"
	BrightPurple   = "#B64DB6"
	BrightPink     = "#FF4D93"
	BrightTeal     = "#4DDBC4"
	BrightLavender = "#C44DFF"
	BrightPeach    = "#FFB84D"
	BrightMint     = "#4DFFD1"
	BrightSky      = "#4DC4FF"
)

// JSON schema types
type VectorJSON struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type PointJSON struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Z     float64 `json:"z"`
	Name  string  `json:"name,omitempty"`
	Color string  `json:"color,omitempty"`
}

type MaterialJSON struct {
	Absorption map[float64]float64 `json:"absorption"`
}

// MarshalJSON implements custom JSON marshaling
func (m *MaterialJSON) MarshalJSON() ([]byte, error) {
	// Create a temporary struct with string keys
	stringMap := make(map[string]float64)
	for k, v := range m.Absorption {
		stringMap[fmt.Sprintf("%.2f", k)] = v
	}

	return json.Marshal(stringMap)
}

func MaterialToJSON(m Material) MaterialJSON {
	return MaterialJSON{
		Absorption: m.alphaMap,
	}
}

type SurfaceJSON struct {
	Material MaterialJSON `json:"material"`
	Name     string       `json:"name,omitempty"`
}

func SurfaceToJSON(s Surface) SurfaceJSON {
	return SurfaceJSON{
		Material: MaterialToJSON(s.Material),
		Name:     s.Name,
	}
}

type ReflectionJSON struct {
	Position PointJSON   `json:"position"`
	Normal   VectorJSON  `json:"normal,omitempty"`
	Surface  SurfaceJSON `json:"surface,omitempty"`
}

type RayJSON struct {
	Origin    PointJSON  `json:"origin"`
	Direction VectorJSON `json:"direction"`
}

type ShotJSON struct {
	Ray  RayJSON `json:"ray"`
	Gain float64 `json:"gain"` // stored in dB
}

type NearestApproachJSON struct {
	Position PointJSON `json:"position"`
	Distance float64   `json:"distance"`
}

type PathJSON struct {
	Points    []PointJSON `json:"points"`
	Name      string      `json:"name,omitempty"`
	Color     string      `json:"color,omitempty"`
	Thickness float64     `json:"thickness,omitempty"`
}

type AcousticPathJSON struct {
	Reflections     []ReflectionJSON    `json:"reflections"`
	Shot            ShotJSON            `json:"shot"`
	Gain            float64             `json:"gain"` // stored in dB
	Distance        float64             `json:"distance"`
	NearestApproach NearestApproachJSON `json:"nearestApproach"`
	Name            string              `json:"name,omitempty"`
	Color           string              `json:"color,omitempty"`
	Thickness       float64             `json:"thickness,omitempty"`
}

type ZoneJSON struct {
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Z            float64 `json:"z"`
	Radius       float64 `json:"radius"`
	Name         string  `json:"name,omitempty"`
	Color        string  `json:"color,omitempty"`
	Transparency float64 `json:"transparency,omitempty"`
}

// Conversion functions
func VectorToJSON(v pt.Vector) VectorJSON {
	return VectorJSON{
		X: v.X,
		Y: v.Y,
		Z: v.Z,
	}
}

func PointToJSON(p Point) PointJSON {
	return PointJSON{
		X:     p.Position.X,
		Y:     p.Position.Y,
		Z:     p.Position.Z,
		Name:  p.Name,
		Color: p.Color,
	}
}

func ReflectionToJSON(r Reflection) ReflectionJSON {
	return ReflectionJSON{
		Position: PointToJSON(Point{Position: r.Position}),
		Normal:   VectorToJSON(r.Normal),
		Surface:  SurfaceToJSON(r.Surface),
	}
}

func ArrivalToAcousticPathJSON(a Arrival) AcousticPathJSON {
	reflections := make([]ReflectionJSON, len(a.AllReflections))
	for i, refl := range a.AllReflections {
		reflections[i] = ReflectionToJSON(refl)
	}

	return AcousticPathJSON{
		Reflections: reflections,
		Shot: ShotJSON{
			Ray: RayJSON{
				Origin: PointToJSON(Point{Position: a.Shot.Ray.Origin}),
				Direction: VectorJSON{
					X: a.Shot.Ray.Direction.X,
					Y: a.Shot.Ray.Direction.Y,
					Z: a.Shot.Ray.Direction.Z,
				},
			},
			Gain: toDB(a.Shot.Gain),
		},
		Gain:     toDB(a.Gain),
		Distance: a.Distance,
		NearestApproach: NearestApproachJSON{
			Position: PointToJSON(Point{Position: a.NearestApproachPosition}),
			Distance: a.NearestApproachDistance,
		},
	}
}

type Zone struct {
	Center pt.Vector
	Radius float64
}

func ZoneToJSON(z Zone) ZoneJSON {
	return ZoneJSON{
		X:      z.Center.X,
		Y:      z.Center.Y,
		Z:      z.Center.Z,
		Radius: z.Radius,
	}
}

// Path represents a sequence of points with optional styling metadata
type PsalmPath struct {
	Points    []Point
	Name      string
	Color     string
	Thickness float64
}

func PathToJSON(path PsalmPath) PathJSON {
	points := make([]PointJSON, len(path.Points))
	for i, p := range path.Points {
		points[i] = PointToJSON(p)
	}
	return PathJSON{
		Points:    points,
		Name:      path.Name,
		Color:     path.Color,
		Thickness: path.Thickness,
	}
}

// Point represents a point in 3D space with size and name
type Point struct {
	Position pt.Vector
	Name     string
	Color    string
}

type Annotations struct {
	Points     []Point
	Paths      []PsalmPath
	Arrivals   []Arrival
	Zones      []Zone
	PathColors map[int]string // This is ugly but yolo
}

func NewAnnotations() *Annotations {
	return &Annotations{
		Points:     []Point{},
		Paths:      []PsalmPath{},
		Arrivals:   []Arrival{},
		Zones:      []Zone{},
		PathColors: map[int]string{},
	}
}

// SaveAnnotationsToJson saves points, paths, and both types of paths to a JSON file
func (a Annotations) WriteToJSON(filename string) error {
	container := struct {
		Points        []PointJSON        `json:"points,omitempty"`
		Paths         []PathJSON         `json:"paths,omitempty"`
		AcousticPaths []AcousticPathJSON `json:"acousticPaths,omitempty"`
		Zones         []ZoneJSON         `json:"zones,omitempty"`
	}{
		Points:        make([]PointJSON, len(a.Points)),
		Paths:         make([]PathJSON, len(a.Paths)),
		AcousticPaths: make([]AcousticPathJSON, 0, len(a.Arrivals)),
		Zones:         make([]ZoneJSON, 0, len(a.Zones)),
	}

	// Convert points
	for i, p := range a.Points {
		container.Points[i] = PointToJSON(p)
	}

	// Convert paths
	for i, path := range a.Paths {
		container.Paths[i] = PathToJSON(path)
	}

	// Convert arrivals to acoustic paths
	for i, arrival := range a.Arrivals {
		if arrival.Distance != INF {
			acousticPath := ArrivalToAcousticPathJSON(arrival)
			if a.PathColors != nil {
				color, ok := a.PathColors[i]
				if ok {
					acousticPath.Color = color
				}
			}
			container.AcousticPaths = append(container.AcousticPaths, acousticPath)
		}
	}

	// Convert zones
	for _, z := range a.Zones {
		container.Zones = append(container.Zones, ZoneToJSON(z))
	}

	data, err := json.MarshalIndent(container, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling points and paths: %w", err)
	}

	return os.WriteFile(filename, data, 0644)
}

type Status string

const (
	NoErr         Status = "success"
	ErrValidation Status = "validation_error"
	ErrSimulation Status = "simulation_error"
)

// Summary Results Schema
type Summary struct {
	Status  Status          `json:"status"`
	Errors  []string        `json:"errors,omitempty"`
	Results AnalysisResults `json:"results"`
}

func NewSummary() *Summary {
	return &Summary{
		Status:  "unknown",
		Errors:  []string{},
		Results: AnalysisResults{},
	}
}

func (r *Summary) AddError(status Status, err error) {
	// validation_error trumps simulation_error
	if r.Status != ErrValidation {
		r.Status = status
	}
	r.Errors = append(r.Errors, err.Error())
}

func (r Summary) WriteToJSON(filename string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling summary results: %w", err)
	}

	return os.WriteFile(filename, data, 0644)
}

type AnalysisResults struct {
	ITD              float64 `json:"ITD,omitempty"`
	EnergyOverWindow float64 `json:"avg_energy_over_window,omitempty"`
	ITD2             float64 `json:"ITD_2,omitempty"`
	AvgGain5ms       float64 `json:"avg_gain_5ms,omitempty"`
	ListenPosX       float64 `json:"listen_pos_x,omitempty"`
	T60Sabine        float64 `json:"T60_sabine,omitempty"`
	T60Eyering       float64 `json:"T60_eyering,omitempty"`
	SchroederFreq    float64 `json:"schroeder_freq,omitempty"`
}

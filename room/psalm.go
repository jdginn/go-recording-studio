package room

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/fogleman/pt/pt"
)

// JSON schema types
type VectorJSON struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type PointJSON struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Z    float64 `json:"z"`
	Size float64 `json:"size,omitempty"`
	Name string  `json:"name,omitempty"`
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
	Points          []PointJSON         `json:"points"`
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
func VectorToJSON(v pt.Vector) PointJSON {
	return PointJSON{
		X:    v.X,
		Y:    v.Y,
		Z:    v.Z,
		Size: 1.0,
	}
}

func ArrivalToAcousticPathJSON(a Arrival) AcousticPathJSON {
	points := make([]PointJSON, len(a.AllReflections))
	for i, v := range a.AllReflections {
		points[i] = VectorToJSON(v)
	}

	return AcousticPathJSON{
		Points: points,
		Shot: ShotJSON{
			Ray: RayJSON{
				Origin: VectorToJSON(a.Shot.Ray.Origin),
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
			Position: VectorToJSON(a.NearestApproachPosition),
			Distance: a.NearestApproachDistance,
		},
		Color: "#FF0000", // Default red color for acoustic paths
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

// SavePointsArrivalsZonesToJSON saves points and both types of paths to a JSON file
func SavePointsArrivalsZonesToJSON(filename string, points []pt.Vector, arrivals []Arrival, zones []Zone) error {
	container := struct {
		Points        []PointJSON        `json:"points,omitempty"`
		Paths         []PathJSON         `json:"paths,omitempty"`
		AcousticPaths []AcousticPathJSON `json:"acousticPaths,omitempty"`
		Zones         []ZoneJSON         `json:"zones, omitment"`
	}{
		Points:        make([]PointJSON, 0, len(points)),
		AcousticPaths: make([]AcousticPathJSON, 0, len(arrivals)),
		Zones:         make([]ZoneJSON, 0, len(zones)),
	}

	// Convert points
	for i, p := range points {
		container.Points[i] = VectorToJSON(p)
	}

	// Convert arrivals to acoustic paths
	for _, arrival := range arrivals {
		if arrival.Distance != INF {
			container.AcousticPaths = append(container.AcousticPaths, ArrivalToAcousticPathJSON(arrival))
		}
	}

	for _, z := range zones {
		container.Zones = append(container.Zones, ZoneToJSON(z))
	}

	data, err := json.MarshalIndent(container, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling points and paths: %w", err)
	}

	return os.WriteFile(filename, data, 0644)
}

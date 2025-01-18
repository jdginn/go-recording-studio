package room

import (
	"fmt"
	"math"

	"github.com/fogleman/pt/pt"
)

type Shot struct {
	ray pt.Ray
}

type Material struct {
	Alpha float64
}

type Wall struct {
	Name     string
	material Material
	mesh     pt.Mesh
}

type Room struct {
	walls []Wall
}

// func (r *Room) GetWallContainingPoint(pt.Vector) (Wall, bool) {
// 	for _, wall := range r.walls {
// 		for _, triangle := range wall.mesh.Triangles {
// 		}
// 	}
// 	return Wall{}, true
// }

func (r *Room) mesh() (pt.Mesh, error) {
	return pt.Mesh{}, nil
}

// TraceParams contains parameters to guide tracing
type TraceParams struct {
	// Maximum number of reflections to simulate
	Order int
	// Stop tracing after the reflection loses this many dB relative to the direct signal
	GainThreshold float64
	// Stop tracing after this many seconds
	TimeThreshold float64
	// Only reflections that pass within this distance from the listening position will be counted as hits
	//
	// Distance in meters
	RFZRadius float64
}

// Arrival defines a reflection that arrives within the RFZ
type Arrival struct {
	// Position of the last reflection
	LastPos pt.Vector
	// Slice of positions of all reflections
	AllPos []pt.Vector
	// Gain in dB relative to the direct signal
	Gain float64
	// Total distance traveled by this ray across all reflections, in meters
	Distance float64
}

const INF = 1e9

var NoHit = Arrival{Gain: 0.0, Distance: INF}

func (r *Room) traceShot(shot Shot, destination pt.Vector, params TraceParams) (Arrival, error) {
	mesh, err := r.mesh()
	if err != nil {
		return Arrival{}, err
	}
	currentRay := shot.ray
	gain := 1.0
	distance := 0.0
	hitPositions := []pt.Vector{}
	for i := 0; i < params.Order; i++ {
		hit := mesh.Intersect(shot.ray)
		if !hit.Ok() {
			return NoHit, fmt.Errorf("Nonterminating ray")
		}
		hitPositions = append(hitPositions, hit.HitInfo.Position)
		surface, ok := r.GetWallContainingPoint(hit.HitInfo.Position)
		if !ok {
			return NoHit, fmt.Errorf("Couldn't find surface")
		}
		gain = gain * surface.material.Alpha
		distance = distance + hit.T // TODO: not totally sure about this

		distFromRFZ := math.Abs(destination.Sub(hit.HitInfo.Position).Length())
		isWithinRFZ := distFromRFZ <= params.RFZRadius
		if isWithinRFZ {
			return Arrival{
				LastPos:  hit.HitInfo.Position,
				AllPos:   hitPositions,
				Gain:     toDB(gain),
				Distance: distance,
			}, nil
		}

		isMaxOrder := i >= params.Order
		isGainThreshold := gain <= params.GainThreshold
		isTimeThreshold := distance/SPEED_OF_SOUND > params.TimeThreshold
		if isMaxOrder || isGainThreshold || isTimeThreshold {
			return NoHit, nil
		}

		currentRay = currentRay.Reflect(hit.HitInfo.Ray)
	}
	panic("Code bug")
}

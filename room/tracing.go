package room

import (
	"fmt"
	"math"

	"github.com/fogleman/pt/pt"
)

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
	// The shot that created this arrival
	Shot Shot
	// Position of the last reflection
	LastReflection pt.Vector
	// Slice of positions of all reflections
	AllReflections []pt.Vector
	// Gain in dB relative to the direct signal
	Gain float64
	// Total distance traveled by this ray across all reflections, in meters
	Distance float64
	// The nearest this arrival came to the listening position
	NearestApproachDistance float64
	// The position of the nearest aproach
	NearestApproachPosition pt.Vector
}

const INF = 1e9

var NoHit = Arrival{Gain: 0.0, Distance: INF}

func nearestApproach(ray pt.Ray, point pt.Vector) float64 {
	diff := point.Sub(ray.Origin)
	if diff.Length() == 0 {
		return 0
	}
	return math.Abs(ray.Direction.Dot(diff) - diff.Length())
}

func raySphereIntersection(ray pt.Ray, center pt.Vector, radius float64) pt.Vector {
	dot := center.Sub(ray.Origin).Dot(ray.Direction)
	dist := dot / ray.Direction.Dot(ray.Direction)
	return ray.Origin.Add(ray.Direction.MulScalar(dist))
}

// TraceShot traces the path taken by a shot until it either arrives at the RFZ or satisfies the othe criteria in params.
//
// See the Params struct type.
func (r *Room) TraceShot(shot Shot, listenPos pt.Vector, params TraceParams) (Arrival, error) {
	mesh, err := r.mesh()
	if err != nil {
		return Arrival{}, err
	}
	currentRay := shot.ray
	gain := 1.0
	distance := 0.0
	hitPositions := []pt.Vector{shot.ray.Origin}
	for i := 0; i < params.Order; i++ {
		hit := mesh.Intersect(currentRay)
		if !hit.Ok() {
			return NoHit, fmt.Errorf("Nonterminating ray")
		}
		info := hit.Info(currentRay)
		hitPositions = append(hitPositions, info.Position)
		gain = gain * (info.Material.Reflectivity)
		distance = distance + hit.T

		currentRay = currentRay.Reflect(info.Ray)

		isMaxOrder := i >= params.Order-1
		isGainThreshold := toDB(gain) <= params.GainThreshold
		isTimeThreshold := distance/SPEED_OF_SOUND > params.TimeThreshold
		if isMaxOrder || isGainThreshold || isTimeThreshold {
			return NoHit, nil
		}

		distFromRFZ := nearestApproach(currentRay, listenPos)
		isWithinRFZ := distFromRFZ <= params.RFZRadius
		if isWithinRFZ {
			return Arrival{
				Shot:                    shot,
				LastReflection:          info.Position,
				AllReflections:          hitPositions,
				Gain:                    toDB(gain),
				Distance:                distance + distFromRFZ,
				NearestApproachDistance: distFromRFZ,
				NearestApproachPosition: raySphereIntersection(currentRay, listenPos, params.RFZRadius),
			}, nil
		}

	}
	panic("Code bug")
}

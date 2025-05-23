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

type Reflection struct {
	// Position of the reflection
	Position pt.Vector
	// Normal of the surface at the reflection
	Normal pt.Vector
	// Acoustic properties of the surface we reflected off of
	Surface Surface
}

// Arrival defines a reflection that arrives within the RFZ
type Arrival struct {
	// The shot that created this arrival
	Shot Shot
	// Position of the last reflection
	LastReflection pt.Vector
	// Slice of positions of all reflections
	AllReflections []Reflection
	// Gain in dB relative to the direct signal
	Gain float64
	// Total distance traveled by this ray across all reflections, in meters
	Distance float64
	// The nearest this arrival came to the listening position
	NearestApproachDistance float64
	// The position of the nearest aproach
	NearestApproachPosition pt.Vector
}

// Returns the distance traveled by the direct signal from source to position of last reflection
func (a Arrival) DirectDist() float64 {
	return a.Shot.Ray.Origin.Sub(a.NearestApproachPosition).Length()
}

// Returns time delay between direct signal and reflection, in milliseconds
func (a Arrival) ITD() float64 {
	return (a.Distance - a.DirectDist()) / SPEED_OF_SOUND * 1000
}

func (a Arrival) NullFreq() float64 {
	return SPEED_OF_SOUND / 2 / (a.Distance - a.DirectDist())
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

func rayHemisphereIntersection(ray pt.Ray, center pt.Vector, radius float64) (pt.Vector, bool) {
	// Vector from ray origin to sphere center
	oc := ray.Origin.Sub(center)

	// Quadratic equation coefficients
	a := ray.Direction.Dot(ray.Direction)
	b := 2 * oc.Dot(ray.Direction)
	c := oc.Dot(oc) - radius*radius

	// Calculate discriminant
	discriminant := b*b - 4*a*c

	// No intersection if discriminant is negative
	if discriminant < 0 {
		return pt.Vector{}, false
	}

	// Calculate the nearest intersection distance
	t := (-b - math.Sqrt(discriminant)) / (2 * a)

	// If t is negative, the intersection is behind the ray origin
	if t < 0 {
		return pt.Vector{}, false
	}

	// Calculate the intersection point
	intersectionPoint := ray.Origin.Add(ray.Direction.MulScalar(t))

	if intersectionPoint.Z < center.Z {
		return pt.Vector{}, false
	}
	if intersectionPoint.X < center.X {
		return pt.Vector{}, false
	}

	return intersectionPoint, true
}

// TraceShot traces the path taken by a shot until it either arrives at the RFZ or satisfies the othe criteria in params.
//
// See the Params struct type.
func (r *Room) TraceShot(shot Shot, listenPos pt.Vector, params TraceParams) ([]Arrival, error) {
	arrivals := []Arrival{}
	mesh, err := r.mesh()
	if err != nil {
		return arrivals, err
	}
	currentRay := shot.Ray
	gain := shot.Gain
	distance := 0.0
	hitPositions := []Reflection{{Position: shot.Ray.Origin}}
	for i := 0; i < params.Order; i++ {
		hit := mesh.Intersect(currentRay)
		if !hit.Ok() {
			return arrivals, fmt.Errorf("Nonterminating ray")
		}
		info := hit.Info(currentRay)
		hitPositions = append(hitPositions, Reflection{
			Position: info.Position, Normal: info.Normal,
			Surface: *info.Shape.(*Triangle).Surface,
		})
		// TODO: LOOK HERE: assuming alpha of 1000Hz is a very dangerous assumption!
		gain = gain * (1 - info.Shape.(*Triangle).Surface.Material.Alpha(1000))
		distance = distance + hit.T

		// nextRay := currentRay.Reflect(info.Ray)
		nextRay := info.Ray
		verifyReflectionLaw(currentRay, info.Normal, nextRay)
		currentRay = nextRay

		pastMaxOrder := i >= params.Order-1
		pastGainThresh := toDB(gain) <= params.GainThreshold
		pastTimeThresh := distance/SPEED_OF_SOUND > params.TimeThreshold
		if pastMaxOrder || pastGainThresh || pastTimeThresh {
			return arrivals, nil
		}

		pos, isWithinRFZ := rayHemisphereIntersection(currentRay, listenPos, params.RFZRadius)

		if isWithinRFZ {
			// TODO:
			// From dist , get null frequency
			// With null frequency, calculate all gains based on frequency

			distToRFZ := pos.Sub(currentRay.Origin).Length()
			// finalDist := distance + distToRFZ

			arrivals = append(arrivals, Arrival{
				Shot:                    shot,
				LastReflection:          info.Position,
				AllReflections:          hitPositions,
				Gain:                    gain,
				Distance:                distance + distToRFZ,
				NearestApproachDistance: nearestApproach(currentRay, listenPos),
				NearestApproachPosition: pos,
			})
			// fmt.Printf("Dist: %f, dist_diff: %f, null_freq: %f\n", distance+distToRFZ, arrivals[len(arrivals)-1].Distance-arrivals[len(arrivals)-1].DirectDist(), arrivals[len(arrivals)-1].NullFreq())
		}

	}
	panic("Code bug")
}

func (r *Room) GetSpeakerCone(speaker Speaker, angle float64, N int, color string) ([]PsalmPath, error) {
	paths := make([]PsalmPath, 0, N)
	rays := speaker.SampleCone(angle, N)

	for _, ray := range rays {
		hit := r.M.Intersect(ray)
		if !hit.Ok() {
			return paths, fmt.Errorf("Nonterminating ray")
		}
		paths = append(paths, PsalmPath{
			Points: []Point{
				{Position: ray.Origin, Color: color},
				{Position: hit.HitInfo.Position, Color: color},
			},
			Color: color,
		})
	}

	return paths, nil
}

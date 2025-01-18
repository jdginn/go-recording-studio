package room

import (
	"math"

	"github.com/fogleman/pt/pt"
	lin "github.com/sgreben/piecewiselinear"
)

type Shot struct {
	ray  pt.Ray
	gain float64
}

type directivity struct {
	horizFunc, vertFunc lin.Function
}

// Returns a directivity struct, which can compute the gain of a ray shot from a given direction
//
// horiz and vert are maps of angle in degrees to gain in dB. Gain should always be negative.
func NewDirectivity(horiz, vert map[float64]float64) directivity {
	d := directivity{}
	hX := make([]float64, 0, len(horiz))
	hY := make([]float64, 0, len(horiz))
	for k, v := range horiz {
		hX = append(hX, k)
		hY = append(hY, v)
	}
	d.horizFunc = lin.Function{
		X: hX,
		Y: hY,
	}
	vX := make([]float64, 0, len(vert))
	vY := make([]float64, 0, len(vert))
	for k, v := range horiz {
		vX = append(vX, k)
		vY = append(vY, v)
	}
	d.vertFunc = lin.Function{
		Y: hY,
		X: vX,
	}
	return d
}

func (d directivity) Gain(horiz, vert float64) float64 {
	return d.horizFunc.At(horiz) + d.vertFunc.At(vert)
}

type Source struct {
	Directivity     directivity
	Position        pt.Vector
	NormalDirection pt.Vector
}

func (s *Source) Sample(numSamples int, horizRange, vertRange float64) []Shot {
	shots := make([]Shot, 0, numSamples)

	var vertSteps, horizSteps int
	horizSteps = int(math.Floor(math.Sqrt(float64(numSamples))))
	vertSteps = numSamples / horizSteps
	for x := 0; x < horizSteps; x++ {
		yaw := -horizRange + 2*horizRange*(float64(x)/float64(horizSteps))
		yawRads := yaw / 180 * math.Pi
		for y := 0; y < vertSteps; y++ {
			pitch := -vertRange + 2*vertRange*(float64(y)/float64(vertSteps))
			pitchRads := pitch / 180 * math.Pi

			shots = append(shots, Shot{
				ray: pt.Ray{
					Origin: s.Position,
					Direction: s.NormalDirection.
						Add(pt.Vector{X: math.Cos(pitchRads), Y: math.Sin(pitchRads), Z: 0}).
						Add(pt.Vector{X: math.Cos(yawRads), Y: 0, Z: math.Sin(yawRads)}).
						Normalize(),
				},
				gain: s.Directivity.Gain(yaw, pitch),
			})
		}
	}
	return shots
}

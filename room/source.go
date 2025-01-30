package room

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/num/quat"
	"gonum.org/v1/gonum/spatial/r3"

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
	Position        pt.Vector
	NormalDirection pt.Vector
}

func (s *Speaker) Sample(numSamples int, horizRange, vertRange float64) []Shot {
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

type LoudSpeakerSpec struct {
	Xdim, Ydim, Zdim float64
	Yoff, Zoff       float64
	Directivity      directivity
}

func raise(v r3.Vec) quat.Number {
	return quat.Number{Imag: v.X, Jmag: v.Y, Kmag: v.Z}
}

type Speaker struct {
	LoudSpeakerSpec
	Source
}

func NewSpeaker(spec LoudSpeakerSpec, pos pt.Vector, dir pt.Vector) *Speaker {
	return &Speaker{
		LoudSpeakerSpec: spec,
		Source: Source{
			Position:        pos,
			NormalDirection: dir,
		},
	}
}

func normalizeQuat(q quat.Number) quat.Number {
	norm := math.Sqrt(q.Real*q.Real + q.Imag*q.Imag + q.Jmag*q.Jmag + q.Kmag*q.Kmag)
	fmt.Printf("norm: %f\n", norm)
	if norm == 0 {
		return quat.Number{Real: 1, Imag: 0, Jmag: 0, Kmag: 0}
	}
	return quat.Number{
		Real: q.Real / norm,
		Imag: q.Imag / norm,
		Jmag: q.Jmag / norm,
		Kmag: q.Kmag / norm,
	}
}

func orthogonal(v pt.Vector) pt.Vector {
	x := math.Abs(v.X)
	y := math.Abs(v.Y)
	z := math.Abs(v.Z)

	var other pt.Vector
	if x < y && x < z {
		other = V(1, 0, 0)
	} else if y < z {
		other = V(0, 1, 0)
	} else {
		other = V(0, 0, 1)
	}
	return v.Cross(other)
}

func rotate(point pt.Vector, originalOrientation, newOrientation pt.Vector) pt.Vector {
	if originalOrientation.Sub(newOrientation).Length() < pt.EPS {
		return point
	}

	originalOrientation = originalOrientation.Normalize()
	newOrientation = newOrientation.Normalize()

	d := newOrientation.Dot(originalOrientation)
	w := originalOrientation.Cross(newOrientation)

	var q quat.Number
	if originalOrientation.Negate().Sub(newOrientation).Length() < pt.EPS {
		orth := orthogonal(originalOrientation).Normalize()
		q = quat.Number{
			Real: 0,
			Imag: orth.X,
			Jmag: orth.Y,
			Kmag: orth.Z,
		}
	} else {
		q = quat.Number{
			Real: d + math.Sqrt(d*d+w.Dot(w)),
			Imag: w.X,
			Jmag: w.Y,
			Kmag: w.Z,
		}
	}
	q = normalizeQuat(q)

	raisedPoint := quat.Number{Imag: point.X, Jmag: point.Y, Kmag: point.Z}
	qq := quat.Mul(quat.Mul(q, raisedPoint), quat.Conj(q))

	return pt.Vector{
		X: qq.Imag,
		Y: qq.Jmag,
		Z: qq.Kmag,
	}
}

// vertices returns the vertieces of this speaker
//
// creates a box representing the speaker, rotates the box  to the speaker's orientation, translates the speaker to
// its position in the room, and then returns the vertices
func (s Speaker) vertices() []pt.Vector {
	topv := func(v r3.Vec) pt.Vector { return pt.Vector{X: v.X, Y: v.Y, Z: v.Z} }

	box := r3.NewBox(0, -s.Yoff, -s.Zoff, s.Xdim, s.Ydim-s.Yoff, s.Zdim-s.Zoff)

	defaultDir := V(1, 0, 0)
	newV := make([]pt.Vector, 0, 8)
	for i, v := range box.Vertices() {
		newV[i] = rotate(topv(v), defaultDir, s.NormalDirection).Add(s.Position)
	}
	return newV
}

// IsInsideRoom returns true if the speaker is inside the innermost set of walls of the mesh
func (s Speaker) IsInsideRoom(m pt.Mesh, listenPos pt.Vector) (bool, error) {
	for _, v := range s.vertices() {
		// Check whether a ray from the listening position is obscured by any walls
		//
		// It's a problem if anything obscures the speaker from the listening position so even if
		// this is not a strict check for being inside the mesh, it is good enough because what
		// we really want to know is whether this position is ok
		//
		// NIT: technically, our only requirement is that the acoustic center of the speaker has
		// an uninterrupted path to the listening position. Since the vertices of the speaker are
		// slightly offset from the acoustic center, it is possible they could be obscured by some
		// convex feature of the room and not strictly violate the requirement for an uninterrupted
		// path from speaker to listening position... but come on, since the dispersion of any real-world
		// speaker is AT LEAST 50deg, such a convex feature would ruin the room for plenty of other reasons
		// and should be rejected anyway!

		hit := m.Intersect(pt.Ray{Origin: listenPos, Direction: v.Sub(listenPos)})
		if hit.T != pt.INF {
			return false, nil
		}
	}
	return true, nil
}

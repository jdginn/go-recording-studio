package room

import (
	"fmt"
	"log"
	"math"
	"sort"

	"gonum.org/v1/gonum/num/quat"
	"gonum.org/v1/gonum/spatial/r3"

	"github.com/fogleman/pt/pt"
	lin "github.com/sgreben/piecewiselinear"
)

type Shot struct {
	Ray  pt.Ray
	Gain float64
}

func (s Shot) Equal(test Shot) bool {
	const epsilon = 1e-7

	return math.Abs(s.Ray.Origin.X-test.Ray.Origin.X) < epsilon &&
		math.Abs(s.Ray.Origin.Y-test.Ray.Origin.Y) < epsilon &&
		math.Abs(s.Ray.Origin.Z-test.Ray.Origin.Z) < epsilon &&
		math.Abs(s.Ray.Direction.X-test.Ray.Direction.X) < epsilon &&
		math.Abs(s.Ray.Direction.Y-test.Ray.Direction.Y) < epsilon &&
		math.Abs(s.Ray.Direction.Z-test.Ray.Direction.Z) < epsilon &&
		math.Abs(s.Gain-test.Gain) < epsilon
}

type directivity struct {
	horizFunc     lin.Function
	vertFunc      lin.Function
	maxHorizAngle float64
	minHorizGain  float64
	maxVertAngle  float64
	minVertGain   float64
}

// Returns a directivity struct, which can compute the gain of a ray shot from a given direction
//
// horiz and vert are maps of angle in degrees to gain in dB. Gain should always be negative except at 0 degrees.
// Angles must be positive and in ascending order. The gain at angle θ is equal to the gain at angle -θ.
func NewDirectivity(horiz, vert map[float64]float64) *directivity {
	d := directivity{}

	// Helper function to validate and process angle-gain maps
	processMap := func(m map[float64]float64, name string) ([]float64, []float64) {
		// Ensure 0 degree gain is present
		if _, exists := m[0]; !exists {
			m[0] = 0
		}

		// Create a slice of angle keys for sorting
		angles := make([]float64, 0, len(m))
		for angle := range m {
			if angle < 0 {
				log.Printf("Warning: ignoring negative angle %.2f in %s directivity map.", angle, name)
				continue
			}
			angles = append(angles, angle)
		}

		// Sort angles in ascending order
		sort.Float64s(angles)

		// Create corresponding gains slice maintaining the sorted order
		gains := make([]float64, len(angles))
		for i, angle := range angles {
			gains[i] = m[angle]
		}

		return angles, gains
	}

	// Process horizontal map
	hX, hY := processMap(horiz, "horizontal")
	d.horizFunc = lin.Function{
		X: hX,
		Y: hY,
	}
	d.maxHorizAngle = hX[len(hX)-1]
	d.minHorizGain = hY[len(hY)-1]

	// Process vertical map
	vX, vY := processMap(vert, "vertical")
	d.vertFunc = lin.Function{
		X: vX,
		Y: vY,
	}
	d.maxVertAngle = vX[len(vX)-1]
	d.minVertGain = vY[len(vY)-1]

	return &d
}

// GainDB returns the gain in dB for a given horizontal and vertical angle
func (d *directivity) GainDB(horizAngle, vertAngle float64) float64 {
	// Take absolute value of angles due to symmetry
	horizAngle = math.Abs(horizAngle)
	vertAngle = math.Abs(vertAngle)

	// Clamp angles and get corresponding gains
	var horizGain, vertGain float64
	if horizAngle >= d.maxHorizAngle {
		horizGain = d.minHorizGain
	} else {
		horizGain = d.horizFunc.At(horizAngle)
	}

	if vertAngle >= d.maxVertAngle {
		vertGain = d.minVertGain
	} else {
		vertGain = d.vertFunc.At(vertAngle)
	}

	// Return the sum of horizontal and vertical gains
	return horizGain + vertGain
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
				Ray: pt.Ray{
					Origin: s.Position,
					Direction: s.NormalDirection.
						Add(pt.Vector{X: math.Cos(pitchRads), Y: math.Sin(pitchRads), Z: 0}).
						Add(pt.Vector{X: math.Cos(yawRads), Y: 0, Z: math.Sin(yawRads)}).
						Normalize(),
				},
				Gain: fromDB(s.GainDB(yaw, pitch)),
			})
		}
	}
	return shots
}

// alignWithNormal rotates a vector that was generated assuming Y-up normal
// to align with the actual normal direction
func alignWithNormal(dir, normal pt.Vector) pt.Vector {
	// If normal is already Y-up, no transformation needed
	yAxis := pt.Vector{X: 0, Y: 1, Z: 0}
	if normal.Sub(yAxis).Length() < 1e-9 {
		return dir
	}

	// Find rotation axis and angle to transform Y-up to normal
	rotAxis := yAxis.Cross(normal).Normalize()
	rotAngle := math.Acos(yAxis.Dot(normal))

	// Rotate the direction vector using Rodriguez rotation formula
	sinTheta := math.Sin(rotAngle)
	cosTheta := math.Cos(rotAngle)

	// v * cos(θ) + (k × v) * sin(θ) + k * (k · v) * (1 - cos(θ))
	crossed := rotAxis.Cross(dir).MulScalar(sinTheta)
	scaled := rotAxis.MulScalar(rotAxis.Dot(dir) * (1 - cosTheta))

	return dir.MulScalar(cosTheta).
		Add(crossed).
		Add(scaled)
}

func (s *Speaker) SampleCone(angleDegrees float64, numRays int) []pt.Ray {
	rays := make([]pt.Ray, numRays)
	angleRads := angleDegrees * math.Pi / 180

	// The height of the normalized direction vector along the cone axis
	h := math.Cos(angleRads)
	// The radius of the circle formed by the cone at unit distance
	r := math.Sin(angleRads)

	for i := 0; i < numRays; i++ {
		// Calculate evenly spaced points around a circle
		theta := 2 * math.Pi * float64(i) / float64(numRays)

		// Calculate the x and z components of the direction vector
		x := r * math.Cos(theta)
		z := r * math.Sin(theta)

		// Create direction vector - this assumes the normal is along Y axis
		dir := pt.Vector{X: x, Y: h, Z: z}

		// If the normal is not along Y axis, we need to rotate our direction
		// to align with the speaker's normal direction
		dir = alignWithNormal(dir, s.NormalDirection)

		rays[i] = pt.Ray{
			Origin:    s.Position,
			Direction: dir.Normalize(),
		}
	}
	return rays
}

type LoudSpeakerSpec struct {
	Xdim, Ydim, Zdim float64
	Yoff, Zoff       float64
	HDirectivityMap  map[float64]float64
	VDirectivityMap  map[float64]float64

	directivity *directivity
}

func (spec LoudSpeakerSpec) GainDB(yaw, pitch float64) float64 {
	if spec.directivity == nil {
		spec.directivity = NewDirectivity(spec.HDirectivityMap, spec.VDirectivityMap)
	}
	return spec.directivity.GainDB(yaw, pitch)
}

func raise(v r3.Vec) quat.Number {
	return quat.Number{Imag: v.X, Jmag: v.Y, Kmag: v.Z}
}

type Speaker struct {
	LoudSpeakerSpec
	Source
}

func NewSpeaker(spec LoudSpeakerSpec, pos pt.Vector, dir pt.Vector) Speaker {
	return Speaker{
		LoudSpeakerSpec: spec,
		Source: Source{
			Position:        pos,
			NormalDirection: dir,
		},
	}
}

func normalizeQuat(q quat.Number) quat.Number {
	norm := math.Sqrt(q.Real*q.Real + q.Imag*q.Imag + q.Jmag*q.Jmag + q.Kmag*q.Kmag)
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

func (s Speaker) verticesUnrotated() []pt.Vector {
	topv := func(v r3.Vec) pt.Vector { return pt.Vector{X: v.X, Y: v.Y, Z: v.Z} }
	box := r3.NewBox(0, -s.Yoff, -s.Zoff, -s.Xdim, s.Ydim-s.Yoff, s.Zdim-s.Zoff)
	newV := make([]pt.Vector, 8)
	for i, v := range box.Vertices() {
		newV[i] = topv(v)
	}
	return newV
}

// vertices returns the vertieces of this speaker
//
// creates a box representing the speaker, rotates the box  to the speaker's orientation, translates the speaker to
// its position in the room, and then returns the vertices
func (s Speaker) vertices() []pt.Vector {
	topv := func(v r3.Vec) pt.Vector { return pt.Vector{X: v.X, Y: v.Y, Z: v.Z} }

	box := r3.NewBox(0, -s.Yoff, -s.Zoff, -s.Xdim, s.Ydim-s.Yoff, s.Zdim-s.Zoff)

	defaultDir := V(1, 0, 0)
	newV := make([]pt.Vector, 8)
	for i, v := range box.Vertices() {
		newPos := topv(v)
		newPos = rotate(topv(v), defaultDir, s.NormalDirection)
		newPos = newPos.Add(s.Position)
		newV[i] = newPos
	}
	return newV
}

// IsInsideRoom returns true if the speaker is inside the innermost set of walls of the mesh
func (s Speaker) IsInsideRoom(m *pt.Mesh, listenPos pt.Vector) (offendingVertex pt.Vector, intersectingPoint pt.Vector, ok bool) {
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

		hit := m.Intersect(pt.Ray{Origin: listenPos, Direction: v.Sub(listenPos).Normalize()})
		// We'll hit the wall eventually, so we just need to make sure the wall is on the far side of the speaker.
		if hit.T <= v.Sub(listenPos).Length() {
			fmt.Printf("v: %v\n", v)
			fmt.Printf("hit: %v\n", hit)
			fmt.Printf("hit Ok: %v\n", hit.Ok())
			fmt.Printf("hitInfo: %v\n", hit.HitInfo)
			if hit.HitInfo == nil {
				return v, pt.Vector{}, false
			}
			return v, hit.HitInfo.Position, false
		}
	}
	return pt.Vector{}, pt.Vector{}, true
}

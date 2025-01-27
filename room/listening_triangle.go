package room

import (
	"math"

	"github.com/fogleman/pt/pt"
)

// Value from Rod Gervais' book Home Recording Studio: Build It Like The Pros
const LISTEN_DIST_INTO_TRIANGLE = 0.38

type ListeningTriangle struct {
	// A point on the front wall
	ReferencePosition pt.Vector
	// The normal vector of the front wall
	//
	// TODO: this is currently ignored. Implement this properly.
	ReferenceNormal pt.Vector
	// Distance of the sources from the front wall
	DistFromFront float64
	// Distance of the sources from the horizontal center of the triangle
	DistFromCenter float64
	// Height of the sources
	SourceHeight float64
	// Height of the listen position
	ListenHeight float64
}

func (t ListeningTriangle) LeftSourcePosition() pt.Vector {
	return pt.Vector{
		X: t.ReferencePosition.X + t.DistFromFront,
		Y: t.ReferencePosition.Y - t.DistFromCenter,
		Z: t.SourceHeight,
	}
}

func (t ListeningTriangle) LeftSourceNormal() pt.Vector {
	return t.ListenPosition().Sub(t.LeftSourcePosition()).Normalize()
}

func (t ListeningTriangle) RightSourcePosition() pt.Vector {
	return pt.Vector{
		X: t.ReferencePosition.X + t.DistFromFront,
		Y: t.ReferencePosition.Y + t.DistFromCenter,
		Z: t.SourceHeight,
	}
}

func (t ListeningTriangle) RightSourceNormal() pt.Vector {
	return t.ListenPosition().Sub(t.RightSourcePosition()).Normalize()
}

func (t ListeningTriangle) ListenPosition() pt.Vector {
	return pt.Vector{
		X: t.ReferencePosition.X + t.DistFromFront + (t.DistFromCenter / 2 * math.Sqrt(3)) + LISTEN_DIST_INTO_TRIANGLE,
		Y: t.ReferencePosition.Y,
		Z: t.ListenHeight,
	}
}

func (t ListeningTriangle) ListenDistance() float64 {
	return math.Abs(t.ListenPosition().Sub(t.LeftSourcePosition()).Length())
}

func (t ListeningTriangle) Deviation(listenPos pt.Vector) float64 {
	return math.Abs(listenPos.Sub(t.ListenPosition()).Length())
}

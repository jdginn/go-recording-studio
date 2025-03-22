package room

import (
	"math"

	"github.com/fogleman/pt/pt"
)

// Value from Rod Gervais' book Home Recording Studio: Build It Like The Pros
const LISTEN_DIST_INTO_TRIANGLE = 0.38

// Value from Thomas Northward posted on GearSpae
// const LISTEN_DIST_INTO_TRIANGLE = 0.32

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
	return t.EquilateralPos().Sub(t.LeftSourcePosition()).Normalize()
}

func (t ListeningTriangle) RightSourcePosition() pt.Vector {
	return pt.Vector{
		X: t.ReferencePosition.X + t.DistFromFront,
		Y: t.ReferencePosition.Y + t.DistFromCenter,
		Z: t.SourceHeight,
	}
}

func (t ListeningTriangle) RightSourceNormal() pt.Vector {
	return t.EquilateralPos().Sub(t.RightSourcePosition()).Normalize()
}

func (t ListeningTriangle) EquilateralPos() pt.Vector {
	distFromSourceLine := t.DistFromCenter * math.Sqrt(3)

	// Calculate how much lower the equilateral point needs to be than the listening height
	// to maintain equal distances when the listening position is moved up and in

	// If we move LISTEN_DIST_INTO_TRIANGLE towards the sources and ListenHeight is higher,
	// we can calculate the required height difference using similar triangles
	heightDrop := LISTEN_DIST_INTO_TRIANGLE * math.Tan(math.Atan2(t.SourceHeight-t.ListenHeight, distFromSourceLine))

	return pt.Vector{
		X: t.ReferencePosition.X + t.DistFromFront + distFromSourceLine,
		Y: t.ReferencePosition.Y,
		Z: t.ListenHeight - heightDrop,
	}
}

func (t ListeningTriangle) ListenPosition() pt.Vector {
	equilateralPos := t.EquilateralPos()

	// Calculate the angle of the triangle's plane
	distFromSourceLine := t.DistFromCenter * math.Sqrt(3)
	planeAngle := math.Atan2(t.SourceHeight-t.ListenHeight, distFromSourceLine)

	// Move LISTEN_DIST_INTO_TRIANGLE along the plane of the triangle
	deltaX := LISTEN_DIST_INTO_TRIANGLE * math.Cos(planeAngle)
	deltaZ := LISTEN_DIST_INTO_TRIANGLE * math.Sin(planeAngle)

	return pt.Vector{
		X: equilateralPos.X - deltaX,
		Y: equilateralPos.Y,
		Z: equilateralPos.Z + deltaZ,
	}
}

func (t ListeningTriangle) ListenDistance() float64 {
	return math.Abs(t.ListenPosition().Sub(t.LeftSourcePosition()).Length())
}

func (t ListeningTriangle) Deviation(listenPos pt.Vector) float64 {
	return math.Abs(listenPos.Sub(t.ListenPosition()).Length())
}

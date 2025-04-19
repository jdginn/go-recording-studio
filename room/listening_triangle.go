package room

import (
	"math"

	"github.com/fogleman/pt/pt"
)

// Value from Rod Gervais' book Home Recording Studio: Build It Like The Pros
// const LISTEN_DIST_INTO_TRIANGLE = 0.38

// Value from Thomas Northward posted on GearSpae
const LISTEN_DIST_INTO_TRIANGLE = 0.32

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
	_, equilateralPos := t.ListenPosition()
	return equilateralPos.Sub(t.LeftSourcePosition()).Normalize()
}

func (t ListeningTriangle) RightSourcePosition() pt.Vector {
	return pt.Vector{
		X: t.ReferencePosition.X + t.DistFromFront,
		Y: t.ReferencePosition.Y + t.DistFromCenter,
		Z: t.SourceHeight,
	}
}

func (t ListeningTriangle) RightSourceNormal() pt.Vector {
	_, equilateralPos := t.ListenPosition()
	return equilateralPos.Sub(t.RightSourcePosition()).Normalize()
}

// ListenPosition returns two points: the ideal Listening Position within the room and the position of a
// hypothetical equilateral triangle with the two sources.
//
// The Listening Position is a constant distance into the triangle (TOWARDS the line connecting the two sources),
// to place the listener's ears directly on the paths from source to the third point of the equilateral triangle ("EquilateralPos").
func (t ListeningTriangle) ListenPosition() (ListenPos, EquilateralPos pt.Vector) {
	sourceToEquilateral := 2 * t.DistFromCenter

	// Use proportional triangles to find the height drop from the listen position to the equilateral position
	//
	// First, get the distance along the hypotenuse (a.k.a. the path from listen position to the equilateral point)
	// using 30-60-90 triangle properties
	alongHypotenuse := LISTEN_DIST_INTO_TRIANGLE * 2 / math.Sqrt(3)
	distAlongHypotenuseToListenPos := sourceToEquilateral - alongHypotenuse
	heightDropToListenPos := t.SourceHeight - t.ListenHeight
	// This is a proportional trianlge
	heightDropToEquilateralPos := heightDropToListenPos / distAlongHypotenuseToListenPos * alongHypotenuse

	// Solve for the X-position of the equilateral point based on the following relationship:
	// The distance from source to equilateralPosition MUST equal sourceToEquilateral
	// Use equation for distance between two points
	//     length = sqrt((a.x-b.x)^2+(a.y-b.y)^2+(a.z-b.z)^2)
	// We know everything except a.x, so we can solve the equation for equialteralPosX
	sourceX := t.ReferencePosition.X + t.DistFromFront
	sourceZ := t.SourceHeight
	equilateralPosZ := t.ListenHeight - heightDropToEquilateralPos
	equilateralPosX := math.Sqrt(math.Pow(sourceToEquilateral, 2)-math.Pow((equilateralPosZ-sourceZ), 2)-math.Pow(t.DistFromCenter, 2)) + sourceX

	// Set up the two positions
	EquilateralPos = pt.Vector{
		X: equilateralPosX,
		Y: t.ReferencePosition.Y,
		Z: equilateralPosZ,
	}

	ListenPos = pt.Vector{
		// Use the distance between two points again, this time based on the rknown relationship:
		// The distance from ListenPos to EquilateralPOs MUST equal LISTEN_DIST_INTO_TRIANGLE
		X: EquilateralPos.X - math.Sqrt(math.Pow(LISTEN_DIST_INTO_TRIANGLE, 2)-math.Pow(heightDropToEquilateralPos, 2)),
		Y: t.ReferencePosition.Y,
		Z: t.ListenHeight,
	}

	return ListenPos, EquilateralPos
}

func (t ListeningTriangle) ListenDistance() float64 {
	listenPos, _ := t.ListenPosition()
	return math.Abs(listenPos.Sub(t.LeftSourcePosition()).Length())
}

func (t ListeningTriangle) Deviation(listenPos pt.Vector) float64 {
	canonicalListenPos, _ := t.ListenPosition()
	return math.Abs(listenPos.Sub(canonicalListenPos).Length())
}

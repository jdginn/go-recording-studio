package room

import (
	"fmt"
	"math"
	"testing"

	"github.com/fogleman/pt/pt"
	"github.com/stretchr/testify/assert"
)

func buildTri(v1, v2, v3 pt.Vector) *pt.Triangle {
	return pt.NewTriangle(v1, v2, v3, pt.Vector{}, pt.Vector{}, pt.Vector{}, pt.Material{})
}

func TestIntersectSetment(t *testing.T) {
	assert := assert.New(t)
	intersects := func(plane Plane, want, v1, v2 pt.Vector) {
		v, ok := plane.intersectSegment(v1, v2)
		assert.True(ok)
		assert.Less(math.Abs(want.Sub(v).Length()), 0.01)
	}
	doesNotIntersect := func(plane Plane, v1, v2 pt.Vector) {
		_, ok := plane.intersectSegment(v1, v2)
		assert.False(ok)
	}

	p := Plane{
		Point:  V(0, 0, 0),
		Normal: V(0, 1, 0),
	}

	intersects(p, V(0, 0, 0), V(0, 2, 0), V(0, -1, 0))
	doesNotIntersect(p, V(0, 2, 0), V(1, 2, 0))
}

func TestIntersectTriangle(t *testing.T) {
	assert := assert.New(t)
	intersects := func(plane Plane, want1 pt.Vector, want2 pt.Vector, tri *pt.Triangle) {
		v1, v2, ok := plane.IntersectTriangle(tri)
		msg := fmt.Sprintf(`
			Expected vertices {%f, %f, %f}, {%f, %f, %f}
			Got vertices      {%f, %f, %f}, {%f, %f, %f}`, want1.X, want1.Y, want1.Z, want2.X, want2.Y, want2.Z, v1.X, v1.Y, v1.Z, v2.X, v2.Y, v2.Z)
		assert.True(ok)
		assert.Less(math.Abs(want1.Sub(v1).Length()), 0.01, msg)
		assert.Less(math.Abs(want2.Sub(v2).Length()), 0.01, msg)
	}
	doesNotIntersect := func(plane Plane, tri *pt.Triangle) {
		_, _, ok := plane.IntersectTriangle(tri)
		assert.False(ok)
	}

	p := Plane{
		Point:  V(0, 1, 0),
		Normal: V(0, 1, 0),
	}

	doesNotIntersect(p, buildTri(V(0, 2, 0), V(15, 2, 0), V(-10, 5, 7)))
	intersects(p, V(1, 1, 0), V(-1, 1, 0), buildTri(V(0.0, 0, 0), V(2, 2, 0), V(-2, 2, 0)))
	intersects(p, V(1, 1, 0), V(0, 1, 0), buildTri(V(0, 0, 0), V(2, 0, 0), V(0, 2, 0)))
}

func TestPlaneIntersection(t *testing.T) {
	v1 := V(0, 0, 0)
	v2 := V(2, 0, 0)
	v3 := V(0, 2, 0)
	v4 := V(0, 0, 2)

	m := pt.NewMesh([]*pt.Triangle{buildTri(v1, v2, v3), buildTri(v2, v3, v4), buildTri(v3, v4, v1), buildTri(v4, v1, v2)})

	p := MakePlane(
		V(0, 1, 0),
		V(0, 1, 0),
	)

	paths := p.SliceMesh(m)

	for _, path := range paths {
		for _, v := range path {
			pv := p.Project(v)
			fmt.Printf("{%f, %f, %f}\n", pv.X, pv.Y, pv.Z)
		}
	}

	fmt.Println(paths)

	t.Fail()
}

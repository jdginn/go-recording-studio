package room

import (
	"math"

	"github.com/fogleman/pt/pt"
)

// Most of this code is taken from https://github.com/fogleman/choppy/tree/master with some modifications

type Point2D struct {
	X, Y float64
}

// To2D converts a 3D vector to a 2D point
func To2D(v pt.Vector) Point2D {
	return Point2D{v.X, v.Y}
}

func (p Point2D) Translate(x, y float64) Point2D {
	return Point2D{p.X + x, p.Y + y}
}

func (p Point2D) Scale(s float64) Point2D {
	return Point2D{p.X * s, p.Y * s}
}

type Path2D []Point2D

func (p Path2D) Translate(x, y float64) Path2D {
	translated := make(Path2D, len(p))
	for i, p := range p {
		translated[i] = p.Translate(x, y)
	}
	return translated
}

func (p Path2D) BoundingBox() (XMin, XMax, YMin, YMax float64) {
	for _, p := range p {
		if p.X < XMin {
			XMin = p.X
		}
		if p.X > XMax {
			XMax = p.X
		}
		if p.Y < YMin {
			YMin = p.Y
		}
		if p.Y > YMax {
			YMax = p.Y
		}
	}
	return
}

func (p Path2D) rawScale(s float64) Path2D {
	translated := make(Path2D, len(p))
	for i, p := range p {
		translated[i] = p.Scale(s)
	}
	return translated
}

func (p Path2D) Scale(v View) Path2D {
	XMin, XMax, YMin, YMax := p.BoundingBox()
	XSize := XMax - XMin
	YSize := YMax - YMin

	XScale := float64(v.XSize) / XSize
	YScale := float64(v.YSize) / YSize

	return p.Translate(-XMin, -YMin).rawScale(math.Max(XScale, YScale))
}

type Plane struct {
	Point  pt.Vector
	Normal pt.Vector
	U, V   pt.Vector
}

func MakePlane(point, normal pt.Vector) Plane {
	u := perpendicular(normal).Normalize()
	v := u.Cross(normal).Normalize()
	return Plane{point, normal, u, v}
}

func (p Plane) Project(point pt.Vector) pt.Vector {
	d := point.Sub(p.Point)
	x := d.Dot(p.U)
	y := d.Dot(p.V)
	return V(x, y, 0)
}

func perpendicular(a pt.Vector) pt.Vector {
	if a.X == 0 && a.Y == 0 {
		if a.Z == 0 {
			return pt.Vector{}
		}
		return V(0, 1, 0)
	}
	return V(-a.Y, a.X, 0).Normalize()
}

type Path []pt.Vector

func joinPaths(paths []Path) []Path {
	frontLookup := make(map[pt.Vector]Path, len(paths))
	for _, path := range paths {
		frontLookup[path[0]] = path
	}
	var result []Path
	for len(frontLookup) > 0 {
		var v pt.Vector
		for v = range frontLookup {
			break
		}
		var path Path
	outer:
		for {
			path = append(path, v)
			if p, ok := frontLookup[v]; ok {
				delete(frontLookup, v)
				v = p[len(p)-1]
			} else {
				for k, thisPath := range frontLookup {
					if thisPath[len(thisPath)-1] == v {
						delete(frontLookup, k)
						v = k
						continue outer
					}
				}
				break
			}
		}
		// if path[0] != path[len(path)-1] {
		// 	continue
		// }
		// path = path[1:]
		// if len(path) < 3 {
		// 	continue
		// }
		result = append(result, path)
	}
	return result
}

func (p Plane) SliceMesh(m *pt.Mesh) []Path {
	var paths []Path
	for _, t := range m.Triangles {
		if v1, v2, ok := p.IntersectTriangle(t); ok {
			paths = append(paths, Path{v1, v2})
		}
	}
	paths = joinPaths(paths)
	return paths
}

func (p Plane) MeshToPath(m *pt.Mesh) []Path2D {
	result := []Path2D{}
	for _, path := range p.SliceMesh(m) {
		thisPath := Path2D{}
		for _, v := range path {
			proj := p.Project(v)
			thisPath = append(thisPath, To2D(proj))
		}
		result = append(result, thisPath)
	}
	return result
}

func (p Plane) pointInFront(v pt.Vector) bool {
	return v.Sub(p.Point).Dot(p.Normal) > 0
}

func (p Plane) intersectSegment(v0, v1 pt.Vector) (pt.Vector, bool) {
	// TODO: do slicing in Z, rotate mesh to plane
	u := v1.Sub(v0)
	w := v0.Sub(p.Point)
	d := p.Normal.Dot(u)
	if d > -1e-9 && d < 1e-9 {
		return pt.Vector{}, false
	}
	n := -p.Normal.Dot(w)
	t := n / d
	if t < 0 || t > 1 {
		return pt.Vector{}, false
	}
	return v0.Add(u.MulScalar(t)), true
}

func (p Plane) IntersectTriangle(t *pt.Triangle) (pt.Vector, pt.Vector, bool) {
	v1, ok1 := p.intersectSegment(t.V1, t.V2)
	v2, ok2 := p.intersectSegment(t.V2, t.V3)
	v3, ok3 := p.intersectSegment(t.V3, t.V1)
	var p1, p2 pt.Vector
	if ok1 && ok2 {
		p1, p2 = v1, v2
	} else if ok1 && ok3 {
		p1, p2 = v1, v3
	} else if ok2 && ok3 {
		p1, p2 = v2, v3
	} else {
		return pt.Vector{}, pt.Vector{}, false
	}
	if p1 == p2 {
		return pt.Vector{}, pt.Vector{}, false
	}
	n := p2.Sub(p1).Cross(p.Normal)
	if n.Dot(t.Normal()) < 0 {
		return p1, p2, true
	} else {
		return p2, p1, true
	}
}

func sutherlandHodgman(points []pt.Vector, planes []Plane) []pt.Vector {
	output := points
	for _, plane := range planes {
		input := output
		output = nil
		if len(input) == 0 {
			return nil
		}
		s := input[len(input)-1]
		for _, e := range input {
			if plane.pointInFront(e) {
				if !plane.pointInFront(s) {
					x, _ := plane.intersectSegment(s, e)
					output = append(output, x)
				}
				output = append(output, e)
			} else if plane.pointInFront(s) {
				x, _ := plane.intersectSegment(s, e)
				output = append(output, x)
			}
			s = e
		}
	}
	return output
}

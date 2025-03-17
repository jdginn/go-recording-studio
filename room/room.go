package room

import (
	"github.com/fogleman/pt/pt"
	"github.com/hpinc/go3mf"
)

type Material struct {
	Alpha float64
}

type Surface struct {
	Name     string
	Material Material
	M        *pt.Mesh
}

func (s *Surface) Normal() pt.Vector {
	// Assumes we only use this on flat surfaces
	return s.M.Triangles[0].T().Normal()
}

func (s *Surface) Absorber(thickness, height float64, material Material) *Surface {
	min := s.M.BoundingBox().Min
	max := s.M.BoundingBox().Max

	// Need to take normal direction into accout when delaing with the thickness

	if max.X-min.X == 0 {
		cube := pt.NewCube(
			pt.Vector{X: min.X - thickness, Y: min.Y, Z: min.Z},
			pt.Vector{X: min.X + thickness, Y: max.Y, Z: min.Z + height},
			pt.Material{})
		return &Surface{
			s.Name + "_absorber",
			material,
			cube.Mesh(),
		}
	}
	if max.Y-min.Y == 0 {
		cube := pt.NewCube(
			pt.Vector{X: min.X, Y: min.Y - thickness, Z: min.Z},
			pt.Vector{X: max.X, Y: min.Y + thickness, Z: min.Z + height},
			pt.Material{})
		return &Surface{
			s.Name + "_absorber",
			material,
			cube.Mesh(),
		}
	}
	panic("Surface is not on a normal we know how to work with")
}

type Triangle struct {
	pt.Triangle
	Surface *Surface
}

func (t Triangle) T() *pt.Triangle {
	return &t.Triangle
}

func (t *Triangle) Intersect(r pt.Ray) pt.Hit {
	e1x := t.V2.X - t.V1.X
	e1y := t.V2.Y - t.V1.Y
	e1z := t.V2.Z - t.V1.Z
	e2x := t.V3.X - t.V1.X
	e2y := t.V3.Y - t.V1.Y
	e2z := t.V3.Z - t.V1.Z
	px := r.Direction.Y*e2z - r.Direction.Z*e2y
	py := r.Direction.Z*e2x - r.Direction.X*e2z
	pz := r.Direction.X*e2y - r.Direction.Y*e2x
	det := e1x*px + e1y*py + e1z*pz
	if det > -pt.EPS && det < pt.EPS {
		return pt.NoHit
	}
	inv := 1 / det
	tx := r.Origin.X - t.V1.X
	ty := r.Origin.Y - t.V1.Y
	tz := r.Origin.Z - t.V1.Z
	u := (tx*px + ty*py + tz*pz) * inv
	if u < 0 || u > 1 {
		return pt.NoHit
	}
	qx := ty*e1z - tz*e1y
	qy := tz*e1x - tx*e1z
	qz := tx*e1y - ty*e1x
	v := (r.Direction.X*qx + r.Direction.Y*qy + r.Direction.Z*qz) * inv
	if v < 0 || u+v > 1 {
		return pt.NoHit
	}
	d := (e2x*qx + e2y*qy + e2z*qz) * inv
	if d < pt.EPS {
		return pt.NoHit
	}

	position := r.Position(d)
	normal := t.NormalAt(position)
	inside := false
	if normal.Dot(r.Direction) > 0 {
		normal = normal.Negate()
		inside = true
	}

	// Calculate proper reflection direction
	dot := r.Direction.Dot(normal)
	reflectDir := r.Direction.Sub(normal.MulScalar(2 * dot))

	ray := pt.Ray{position, reflectDir}
	info := pt.HitInfo{t, position, normal, ray, t.MaterialAt(position), inside}
	return pt.Hit{t, d, &info}
}

var _ pt.TriangleInt = (*Triangle)(nil)

type Wall struct {
	Name     string
	Material Material
}

type Room struct {
	M *pt.Mesh
}

const SCALE = 1000

func NewFrom3MF(filepath string, materials map[string]Material) (Room, map[string]*Surface, error) {
	surfaces := make(map[string]*Surface)

	if _, ok := materials["default"]; !ok {
		materials["default"] = Material{0.2}
	}

	var model go3mf.Model
	r, err := go3mf.OpenReader(filepath)
	if err != nil {
		return Room{}, surfaces, err
	}
	r.Decode(&model)

	room := Room{}

	triangles := []pt.TriangleInt{}
	for _, item := range model.Build.Items {
		obj, _ := model.FindObject(item.ObjectPath(), item.ObjectID)

		var material Material
		if _, ok := materials[obj.Name]; ok {
			material = materials[obj.Name]
		} else {
			material = materials["default"]
		}

		surface := &Surface{
			Name:     obj.Name,
			Material: material,
		}
		theseTriangles := make([]pt.TriangleInt, len(obj.Mesh.Triangles.Triangle))

		if obj.Mesh != nil {
			for i, t := range obj.Mesh.Triangles.Triangle {
				ptTri := pt.Triangle{Material: &pt.Material{}}
				ptTri.V1 = pt.Vector{
					X: float64(obj.Mesh.Vertices.Vertex[t.V1].X() / SCALE),
					Y: float64(obj.Mesh.Vertices.Vertex[t.V1].Y() / SCALE),
					Z: float64(obj.Mesh.Vertices.Vertex[t.V1].Z() / SCALE),
				}
				ptTri.V2 = pt.Vector{
					X: float64(obj.Mesh.Vertices.Vertex[t.V2].X() / SCALE),
					Y: float64(obj.Mesh.Vertices.Vertex[t.V2].Y() / SCALE),
					Z: float64(obj.Mesh.Vertices.Vertex[t.V2].Z() / SCALE),
				}
				ptTri.V3 = pt.Vector{
					X: float64(obj.Mesh.Vertices.Vertex[t.V3].X() / SCALE),
					Y: float64(obj.Mesh.Vertices.Vertex[t.V3].Y() / SCALE),
					Z: float64(obj.Mesh.Vertices.Vertex[t.V3].Z() / SCALE),
				}
				ptTri.FixNormals()
				triangles = append(triangles, &Triangle{Triangle: ptTri, Surface: surface})
				theseTriangles[i] = &ptTri
			}
			surface.M = pt.NewMesh(theseTriangles)
		}

		surfaces[obj.Name] = surface
	}
	room.M = pt.NewMesh(triangles)
	room.M.Compile()
	return room, surfaces, nil
}

func NewEmptyRoom() Room {
	return Room{
		M: pt.NewMesh([]pt.TriangleInt{}),
	}
}

func (r *Room) mesh() (*pt.Mesh, error) {
	return r.M, nil
	// return pt.Mesh{}, nil
}

func (r *Room) AddWall(point pt.Vector, normal pt.Vector, name string, material Material) error {

	surface := Surface{
		Name:     name,
		Material: material,
	}

	plane := MakePlane(point, normal)

	newTriangles := []pt.TriangleInt{}

	for _, tri := range r.M.Triangles {
		p1, p2, intersect := plane.IntersectTriangle(tri)
		if intersect {
			// TODO: what about intersections with an existing vertex of the room? In that case p1 == p2 and the third vertex of the new triangle must come
			// from another intersected triangle from the mesh
			newTriangles = append(newTriangles, &Triangle{
				Triangle: pt.Triangle{
					V1: point,
					V2: p1,
					V3: p2,
				},
				Surface: &surface,
			})
		}
	}

	r.M.Triangles = append(r.M.Triangles, newTriangles...)
	r.M.Compile()

	return nil
}

type Bounds struct {
	Min, Max float64
}

func (r *Room) AddPrism(XBound, YBound, ZBound Bounds, name string, material Material) error {
	cube := pt.NewCube(
		pt.Vector{X: XBound.Min, Y: YBound.Min, Z: ZBound.Min},
		pt.Vector{X: XBound.Max, Y: YBound.Max, Z: ZBound.Max},
		pt.Material{})

	for _, tri := range cube.Mesh().Triangles {
		r.M.Triangles = append(r.M.Triangles, &Triangle{
			Triangle: *tri.T(),
			Surface:  &Surface{Name: name, Material: material},
		})
	}

	r.M.Compile()

	return nil
}

func (r *Room) AddSurface(surface *Surface) error {
	for _, tri := range surface.M.Triangles {
		r.M.Triangles = append(r.M.Triangles, &Triangle{
			Triangle: *tri.T(),
			Surface:  surface,
		})
	}
	r.M.Compile()
	return nil
}

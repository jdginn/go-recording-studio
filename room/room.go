package room

import (
	"github.com/fogleman/pt/pt"
	"github.com/hpinc/go3mf"
)

type Material struct {
	Alpha float64
}

// type Wall struct {
// 	Name     string
// 	material Material
// 	mesh     pt.Mesh
// }

type Room struct {
	// walls []Wall
	M *pt.Mesh
}

const SCALE = 1000

func NewFrom3MF(filepath string, materials map[string]Material) (Room, error) {
	if _, ok := materials["default"]; !ok {
		materials["default"] = Material{0.2}
	}

	var model go3mf.Model
	r, err := go3mf.OpenReader(filepath)
	if err != nil {
		return Room{}, err
	}
	r.Decode(&model)

	room := Room{}

	ptTriangles := []*pt.Triangle{}
	for _, item := range model.Build.Items {
		obj, _ := model.FindObject(item.ObjectPath(), item.ObjectID)

		var material Material
		if _, ok := materials[obj.Name]; ok {
			material = materials[obj.Name]
		} else {
			material = materials["default"]
		}
		ptMaterial := pt.Material{Reflectivity: 1 - material.Alpha}

		if obj.Mesh != nil {
			for _, t := range obj.Mesh.Triangles.Triangle {
				ptTri := &pt.Triangle{}
				ptTri.Material = &ptMaterial
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
				ptTriangles = append(ptTriangles, ptTri)
			}
		}

		// TODO: link each triangle to the name of its shape in the 3mf file
	}
	room.M = pt.NewMesh(ptTriangles)
	room.M.Compile()
	return room, nil
}

func (r *Room) mesh() (*pt.Mesh, error) {
	return r.M, nil
	// return pt.Mesh{}, nil
}

func (r *Room) AddWall(point pt.Vector, normal pt.Vector) error {
	// Compute intersection of plane with mesh
	// TODO: for now we assume that the intersection is convex
	// Build triangles from point to each point on the path of the plane's intersection
	plane := MakePlane(point, normal)

	newTriangles := []*pt.Triangle{}

	for _, tri := range r.M.Triangles {
		p1, p2, intersect := plane.IntersectTriangle(tri)
		if intersect {
			// TODO: what about intersections with an existing vertex of the room? In that case p1 == p2 and the third vertex of the new triangle must come
			// from another intersected triangle from the mesh
			newTriangles = append(newTriangles, &pt.Triangle{
				V1: plane.Point,
				V2: p1,
				V3: p2,
			})
		}
	}

	r.M.Triangles = append(r.M.Triangles, newTriangles...)

	return nil
}

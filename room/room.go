package room

import (
	"fmt"
	"math"

	"github.com/fogleman/pt/pt"
	"github.com/hpinc/go3mf"
)

type Shot struct {
	ray pt.Ray
}

type Material struct {
	Alpha float64
}

type Wall struct {
	Name     string
	material Material
	mesh     pt.Mesh
}

type Room struct {
	// walls []Wall
	m *pt.Mesh
}

var WallMaterials = map[string]Material{
	"default": {0.9},
}

func NewRoom(filepath string) (Room, error) {
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
		fmt.Println("object:", *obj)

		var material Material
		if _, ok := WallMaterials[obj.Name]; ok {
			material = WallMaterials[obj.Name]
		} else {
			material = WallMaterials["default"]
		}
		ptMaterial := pt.Material{Reflectivity: 1.0 - material.Alpha}

		if obj.Mesh != nil {
			for _, t := range obj.Mesh.Triangles.Triangle {
				ptTri := &pt.Triangle{}
				ptTri.Material = &ptMaterial
				ptTri.V1 = pt.Vector{
					X: float64(obj.Mesh.Vertices.Vertex[t.V1].X()),
					Y: float64(obj.Mesh.Vertices.Vertex[t.V1].Y()),
					Z: float64(obj.Mesh.Vertices.Vertex[t.V1].Z()),
				}
				ptTri.V2 = pt.Vector{
					X: float64(obj.Mesh.Vertices.Vertex[t.V2].X()),
					Y: float64(obj.Mesh.Vertices.Vertex[t.V2].Y()),
					Z: float64(obj.Mesh.Vertices.Vertex[t.V2].Z()),
				}
				ptTri.V3 = pt.Vector{
					X: float64(obj.Mesh.Vertices.Vertex[t.V3].X()),
					Y: float64(obj.Mesh.Vertices.Vertex[t.V3].Y()),
					Z: float64(obj.Mesh.Vertices.Vertex[t.V3].Z()),
				}
				ptTri.FixNormals()
				ptTriangles = append(ptTriangles, ptTri)
			}
		}

		// TODO: link each triangle to the name of its shape in the 3mf file
	}
	room.m = pt.NewMesh(ptTriangles)
	return room, nil
}

func (r *Room) mesh() (*pt.Mesh, error) {
	return r.m, nil
	// return pt.Mesh{}, nil
}

// TraceParams contains parameters to guide tracing
type TraceParams struct {
	// Maximum number of reflections to simulate
	Order int
	// Stop tracing after the reflection loses this many dB relative to the direct signal
	GainThreshold float64
	// Stop tracing after this many seconds
	TimeThreshold float64
	// Only reflections that pass within this distance from the listening position will be counted as hits
	//
	// Distance in meters
	RFZRadius float64
}

// Arrival defines a reflection that arrives within the RFZ
type Arrival struct {
	// Position of the last reflection
	LastPos pt.Vector
	// Slice of positions of all reflections
	AllPos []pt.Vector
	// Gain in dB relative to the direct signal
	Gain float64
	// Total distance traveled by this ray across all reflections, in meters
	Distance float64
}

const INF = 1e9

var NoHit = Arrival{Gain: 0.0, Distance: INF}

func (r *Room) traceShot(shot Shot, destination pt.Vector, params TraceParams) (Arrival, error) {
	mesh, err := r.mesh()
	if err != nil {
		return Arrival{}, err
	}
	currentRay := shot.ray
	gain := 1.0
	distance := 0.0
	hitPositions := []pt.Vector{}
	for i := 0; i < params.Order; i++ {
		hit := mesh.Intersect(shot.ray)
		if !hit.Ok() {
			return NoHit, fmt.Errorf("Nonterminating ray")
		}
		hitPositions = append(hitPositions, hit.HitInfo.Position)
		gain = gain * (1 - hit.HitInfo.Material.Reflectivity)
		distance = distance + hit.T // TODO: not totally sure about this

		distFromRFZ := math.Abs(destination.Sub(hit.HitInfo.Position).Length())
		isWithinRFZ := distFromRFZ <= params.RFZRadius
		if isWithinRFZ {
			return Arrival{
				LastPos:  hit.HitInfo.Position,
				AllPos:   hitPositions,
				Gain:     toDB(gain),
				Distance: distance,
			}, nil
		}

		isMaxOrder := i >= params.Order
		isGainThreshold := gain <= params.GainThreshold
		isTimeThreshold := distance/SPEED_OF_SOUND > params.TimeThreshold
		if isMaxOrder || isGainThreshold || isTimeThreshold {
			return NoHit, nil
		}

		currentRay = currentRay.Reflect(hit.HitInfo.Ray)
	}
	panic("Code bug")
}

package room

import (
	"fmt"
	"math"

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

var WallMaterials = map[string]Material{
	"default": {0.3},
}

const SCALE = 1000

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
		// fmt.Println("object:", *obj)

		var material Material
		if _, ok := WallMaterials[obj.Name]; ok {
			material = WallMaterials[obj.Name]
		} else {
			material = WallMaterials["default"]
		}
		ptMaterial := pt.Material{Reflectivity: material.Alpha}

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

func nearestApproach(ray pt.Ray, point pt.Vector) float64 {
	diff := point.Sub(ray.Origin)
	if diff.Length() == 0 {
		return 0
	}
	return math.Abs(ray.Direction.Dot(diff) - diff.Length())
}

func (r *Room) TraceShot(shot Shot, listenPos pt.Vector, params TraceParams) (Arrival, error) {
	mesh, err := r.mesh()
	if err != nil {
		return Arrival{}, err
	}
	currentRay := shot.ray
	gain := 1.0
	distance := 0.0
	hitPositions := make([]pt.Vector, 0)
	for i := 0; i < params.Order; i++ {
		hit := mesh.Intersect(shot.ray)
		if !hit.Ok() {
			return NoHit, fmt.Errorf("Nonterminating ray")
		}
		info := hit.Info(shot.ray)
		hitPositions = append(hitPositions, info.Position)
		gain = gain * (1 - info.Material.Reflectivity)
		distance = distance + hit.T

		currentRay = currentRay.Reflect(info.Ray) // TODO: this might be wrong

		distFromRFZ := nearestApproach(currentRay, listenPos)
		isWithinRFZ := distFromRFZ <= params.RFZRadius
		if isWithinRFZ {
			return Arrival{
				LastPos:  info.Position,
				AllPos:   hitPositions,
				Gain:     toDB(gain),
				Distance: distance + distFromRFZ,
			}, nil
		}

		isMaxOrder := i >= params.Order-1
		isGainThreshold := toDB(gain) <= params.GainThreshold
		// fmt.Printf("Time: %f Thresh: %f\n", distance/SPEED_OF_SOUND*100, params.TimeThreshold*100)
		isTimeThreshold := distance/SPEED_OF_SOUND > params.TimeThreshold
		if isMaxOrder || isGainThreshold || isTimeThreshold {
			return NoHit, nil
		}

	}
	panic("Code bug")
}

package room

import (
	"math"
	"sort"

	"github.com/fogleman/pt/pt"
	"github.com/hpinc/go3mf"
	lin "github.com/sgreben/piecewiselinear"
)

type Material struct {
	alphaFunc lin.Function
	alphaMap  map[float64]float64 // For now, we only use this for serializing materials to json
}

func NewMaterial(alphaMap map[float64]float64) Material {
	// Create a slice of freq-alpha pairs
	pairs := make([][2]float64, 0, len(alphaMap))
	for freq, alpha := range alphaMap {
		pairs = append(pairs, [2]float64{freq, alpha})
	}

	// Sort pairs by frequency
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i][0] < pairs[j][0]
	})

	// Create sorted slices
	freqs := make([]float64, len(pairs))
	alphas := make([]float64, len(pairs))
	for i, pair := range pairs {
		freqs[i] = pair[0]  // frequency
		alphas[i] = pair[1] // alpha
	}

	return Material{
		alphaFunc: lin.Function{
			X: freqs,
			Y: alphas,
		},
		alphaMap: alphaMap,
	}
}

func PerfectReflector() Material {
	return Material{alphaFunc: lin.Function{[]float64{125}, []float64{0.0}}, alphaMap: map[float64]float64{125: 0}}
}

func PerfectAbsorber() Material {
	return Material{alphaFunc: lin.Function{[]float64{125}, []float64{1.0}}, alphaMap: map[float64]float64{125: 1}}
}

func (m Material) Alpha(freq float64) float64 {
	return m.alphaFunc.At(freq)
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

func NewFrom3MF(filepath string, materials map[string]Material) (*Room, map[string]*Surface, error) {
	surfaces := make(map[string]*Surface)

	if _, ok := materials["default"]; !ok {
		// Sane default absorption for brick
		materials["default"] = NewMaterial(map[float64]float64{125: 0.05, 250: 0.04, 500: 0.02, 1000: 0.04, 2000: 0.05, 4000: 0.05})
	}

	var model go3mf.Model
	r, err := go3mf.OpenReader(filepath)
	if err != nil {
		return &Room{}, surfaces, err
	}
	r.Decode(&model)

	room := &Room{}

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
	surface := &Surface{
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
			tri := &Triangle{
				Triangle: pt.Triangle{
					V1:       point,
					V2:       p1,
					V3:       p2,
					Material: &pt.Material{},
				},
				Surface: surface,
			}
			tri.FixNormals()
			newTriangles = append(newTriangles, tri)
		}
	}

	r.M = pt.NewMesh(append(r.M.Triangles, newTriangles...))
	// r.M.Compile()

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

	newTriangles := []pt.TriangleInt{}
	for _, tri := range cube.Mesh().Triangles {
		tri := &Triangle{
			Triangle: *tri.T(),
			Surface:  &Surface{Name: name, Material: material},
		}
		tri.FixNormals()
		newTriangles = append(newTriangles, tri)
	}

	r.M = pt.NewMesh(append(r.M.Triangles, newTriangles...))
	r.M.Compile()

	return nil
}

func (r *Room) AddSurface(surface *Surface) error {
	newTriangles := []pt.TriangleInt{}
	for _, tri := range surface.M.Triangles {
		tri := &Triangle{
			Triangle: *tri.T(),
			Surface:  surface,
		}
		tri.FixNormals()
		newTriangles = append(newTriangles, tri)
	}
	r.M = pt.NewMesh(append(r.M.Triangles, newTriangles...))
	r.M.Compile()

	return nil
}

// Function to count intersections between a ray and the mesh
func countIntersections(ray pt.Ray, mesh *pt.Mesh) int {
	count := 0
	for _, triangle := range mesh.Triangles {
		hit := triangle.Intersect(ray)
		if hit.Ok() {
			count++
		}
	}
	return count
}

// Function to calculate the centroid of a triangle
func triangleCentroid(triangle pt.TriangleInt) pt.Vector {
	tri := triangle.T()
	return pt.Vector{
		X: (tri.V1.X + tri.V2.X + tri.V3.X) / 3,
		Y: (tri.V1.Y + tri.V2.Y + tri.V3.Y) / 3,
		Z: (tri.V1.Z + tri.V2.Z + tri.V3.Z) / 3,
	}
}

// Check if a triangle is inside the mesh
func isTriangleInside(triangle pt.TriangleInt, mesh *pt.Mesh) bool {
	centroid := triangleCentroid(triangle)
	ray := pt.Ray{
		Origin:    centroid,
		Direction: pt.Vector{X: 1, Y: 0, Z: 0}, // Cast ray in +X direction
	}
	return countIntersections(ray, mesh)%2 == 1
}

// InteriorMesh returns the mesh describing the innermost set of walls in the room
func (r *Room) InteriorMesh() (*pt.Mesh, error) {
	newTriangles := []pt.TriangleInt{}

	for _, tri := range r.M.Triangles {
		if isTriangleInside(tri, r.M) {
			newTriangles = append(newTriangles, tri)
		}
	}

	return pt.NewMesh(newTriangles), nil
}

// ComputeMeshVolume calculates the volume of a closed mesh
// Note: The mesh must be closed and properly oriented (consistent winding order)
// Returns: Volume in cubic units of the mesh coordinates
func ComputeMeshVolume(mesh *pt.Mesh) float64 {
	volume := 0.0

	// For each triangle in the mesh
	for _, triangle := range mesh.Triangles {
		triangle := triangle.T()
		// Compute the signed volume of tetrahedron formed by triangle and origin
		v321 := triangle.V3.X * triangle.V2.Y * triangle.V1.Z
		v231 := triangle.V2.X * triangle.V3.Y * triangle.V1.Z
		v312 := triangle.V3.X * triangle.V1.Y * triangle.V2.Z
		v132 := triangle.V1.X * triangle.V3.Y * triangle.V2.Z
		v213 := triangle.V2.X * triangle.V1.Y * triangle.V3.Z
		v123 := triangle.V1.X * triangle.V2.Y * triangle.V3.Z

		signedVolume := (-v321 + v231 + v312 - v132 - v213 + v123) / 6.0
		volume += signedVolume
	}

	return math.Abs(volume)
}

// SurfaceArea returns the surface area of the INTERIOR of the room
func (r *Room) SurfaceArea() (float64, error) {
	area := 0.0
	interiorMesh, err := r.InteriorMesh()
	if err != nil {
		return 0, err
	}
	for _, tri := range interiorMesh.Triangles {
		tri := tri.T()
		area += tri.Area()
	}
	return area, nil
}

// Volume returns the volume of the INTERIOR of the room
func (r *Room) Volume() (float64, error) {
	// interior, err := r.InteriorMesh()
	// if err != nil {
	// 	return 0, err
	// }
	interior := r.M
	return ComputeMeshVolume(interior), nil
}

func (r *Room) NominalT60() (float64, error) {
	volume, err := r.Volume()
	if err != nil {
		return 0, err
	}
	return 0.25 * math.Pow((volume/100), 1.0/3.0), nil
}

const (
	SABINE          = 0.161
	EYERING         = 55.3
	SCHROEDER_COEFF = 2000
)

// T60Sabine returns the Sabine reverberation time of the room in seconds
func (r *Room) T60Sabine(freq float64) (float64, error) {
	sabines := 0.0
	for _, tri := range r.M.Triangles {
		sabines += tri.(*Triangle).Surface.Material.Alpha(freq) * tri.T().Area()
	}
	v, err := r.Volume()
	if err != nil {
		return 0, err
	}
	return SABINE * v / sabines, nil
}

func (r *Room) T60Eyring(freq float64) (float64, error) {
	sabines := 0.0
	for _, tri := range r.M.Triangles {
		sabines += tri.(*Triangle).Surface.Material.Alpha(freq) * tri.T().Area()
	}
	surfaceArea, err := r.SurfaceArea()
	if err != nil {
		return 0, err
	}
	volume, err := r.Volume()
	if err != nil {
		return 0, err
	}
	avgAbsorpCoeff := sabines / surfaceArea
	return EYERING * volume / (-SPEED_OF_SOUND * surfaceArea * math.Log(1-avgAbsorpCoeff)), nil
}

// SchroederFreq returns the Schroeder frequency of the room, which is the frequency at which the reverb transitions from modal to specular behavior
func (r *Room) SchroederFreq() (float64, error) {
	// 250Hz is a reasonable starting point since the Schroeder frequency is usually in that ballpark
	rt60, err := r.T60Sabine(250)
	if err != nil {
		return 0, err
	}
	volume, err := r.Volume()
	if err != nil {
		return 0, err
	}
	return SCHROEDER_COEFF * math.Sqrt(rt60/volume), nil
}

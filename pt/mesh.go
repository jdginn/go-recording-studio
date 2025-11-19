package pt

import "math"

type Mesh struct {
	Triangles []TriangleInt
	box       *Box
	tree      *Tree
}

func NewMesh(triangles []TriangleInt) *Mesh {
	return &Mesh{triangles, nil, nil}
}

func (m *Mesh) dirty() {
	m.box = nil
	m.tree = nil
}

func (m *Mesh) Copy() *Mesh {
	triangles := make([]TriangleInt, len(m.Triangles))
	for i, t := range m.Triangles {
		a := t
		triangles[i] = a
	}
	return NewMesh(triangles)
}

func (m *Mesh) Compile() {
	if m.tree == nil {
		shapes := make([]Shape, len(m.Triangles))
		for i, triangle := range m.Triangles {
			shapes[i] = triangle
		}
		m.tree = NewTree(shapes)
	}
}

func (a *Mesh) Add(b *Mesh) {
	a.Triangles = append(a.Triangles, b.Triangles...)
	a.dirty()
}

func (m *Mesh) BoundingBox() Box {
	if m.box == nil {
		min := m.Triangles[0].T().V1
		max := m.Triangles[0].T().V1
		for _, t := range m.Triangles {
			min = min.Min(t.T().V1).Min(t.T().V2).Min(t.T().V3)
			max = max.Max(t.T().V1).Max(t.T().V2).Max(t.T().V3)
		}
		m.box = &Box{min, max}
	}
	return *m.box
}

func (m *Mesh) Intersect(r Ray) Hit {
	return m.tree.Intersect(r)
}

func (m *Mesh) UV(p Vector) Vector {
	return Vector{} // not implemented
}

func (m *Mesh) MaterialAt(p Vector) Material {
	return Material{} // not implemented
}

func (m *Mesh) NormalAt(p Vector) Vector {
	return Vector{} // not implemented
}

func smoothNormalsThreshold(normal Vector, normals []Vector, threshold float64) Vector {
	result := Vector{}
	for _, x := range normals {
		if x.Dot(normal) >= threshold {
			result = result.Add(x)
		}
	}
	return result.Normalize()
}

func (m *Mesh) SmoothNormalsThreshold(radians float64) {
	threshold := math.Cos(radians)
	lookup := make(map[Vector][]Vector)
	for _, t := range m.Triangles {
		lookup[t.T().V1] = append(lookup[t.T().V1], t.T().N1)
		lookup[t.T().V2] = append(lookup[t.T().V2], t.T().N2)
		lookup[t.T().V3] = append(lookup[t.T().V3], t.T().N3)
	}
	for _, t := range m.Triangles {
		t.T().N1 = smoothNormalsThreshold(t.T().N1, lookup[t.T().V1], threshold)
		t.T().N2 = smoothNormalsThreshold(t.T().N2, lookup[t.T().V2], threshold)
		t.T().N3 = smoothNormalsThreshold(t.T().N3, lookup[t.T().V3], threshold)
	}
}

func (m *Mesh) SmoothNormals() {
	lookup := make(map[Vector]Vector)
	for _, t := range m.Triangles {
		lookup[t.T().V1] = lookup[t.T().V1].Add(t.T().N1)
		lookup[t.T().V2] = lookup[t.T().V2].Add(t.T().N2)
		lookup[t.T().V3] = lookup[t.T().V3].Add(t.T().N3)
	}
	for k, v := range lookup {
		lookup[k] = v.Normalize()
	}
	for _, t := range m.Triangles {
		t.T().N1 = lookup[t.T().V1]
		t.T().N2 = lookup[t.T().V2]
		t.T().N3 = lookup[t.T().V3]
	}
}

func (m *Mesh) UnitCube() {
	m.FitInside(Box{Vector{}, Vector{1, 1, 1}}, Vector{})
	m.MoveTo(Vector{}, Vector{0.5, 0.5, 0.5})
}

func (m *Mesh) MoveTo(position, anchor Vector) {
	matrix := Translate(position.Sub(m.BoundingBox().Anchor(anchor)))
	m.Transform(matrix)
}

func (m *Mesh) FitInside(box Box, anchor Vector) {
	scale := box.Size().Div(m.BoundingBox().Size()).MinComponent()
	extra := box.Size().Sub(m.BoundingBox().Size().MulScalar(scale))
	matrix := Identity()
	matrix = matrix.Translate(m.BoundingBox().Min.Negate())
	matrix = matrix.Scale(Vector{scale, scale, scale})
	matrix = matrix.Translate(box.Min.Add(extra.Mul(anchor)))
	m.Transform(matrix)
}

func (m *Mesh) Transform(matrix Matrix) {
	for _, t := range m.Triangles {
		t.T().V1 = matrix.MulPosition(t.T().V1)
		t.T().V2 = matrix.MulPosition(t.T().V2)
		t.T().V3 = matrix.MulPosition(t.T().V3)
		t.T().N1 = matrix.MulDirection(t.T().N1)
		t.T().N2 = matrix.MulDirection(t.T().N2)
		t.T().N3 = matrix.MulDirection(t.T().N3)
	}
	m.dirty()
}

func (m *Mesh) SetMaterial(material Material) {
	for _, t := range m.Triangles {
		t.T().Material = &material
	}
}

func (m *Mesh) SaveSTL(path string) error {
	return SaveSTL(path, m)
}

package room

import (
	"fmt"
	"math"
	"testing"

	"github.com/fogleman/pt/pt"
)

type pp struct {
	X, Y, Z float64
}

func (pp pp) String() string {
	return "{" + fmt.Sprintf("%.1f, %.1f, %.1f", pp.X, pp.Y, pp.Z) + "}"
}

func pretty(p pt.Vector) pp {
	return pp{
		X: p.X,
		Y: p.Y,
		Z: p.Z,
	}
}

func prettyArr(a []pt.Vector) []pp {
	b := make([]pp, len(a))
	for i, v := range a {
		b[i] = pretty(v)
	}
	return b
}

func TestRotate(t *testing.T) {
	type input struct {
		point       pt.Vector
		originalVec pt.Vector
		targetVec   pt.Vector
	}

	tests := []struct {
		name   string
		input  input
		expect pt.Vector
	}{
		{"origin_no_rotate", input{
			point:       V(0, 0, 0),
			originalVec: V(1, 0, 0),
			targetVec:   V(1, 0, 0),
		}, V(0, 0, 0)},
		{"origin_90deg", input{
			point:       V(0, 0, 0),
			originalVec: V(1, 0, 0),
			targetVec:   V(0, 1, 0),
		}, V(0, 0, 0)},
		{"origin_180deg", input{
			point:       V(0, 0, 0),
			originalVec: V(1, 0, 0),
			targetVec:   V(-1, 0, 0),
		}, V(0, 0, 0)},
		{"origin_arbitrary", input{
			point:       V(0, 0, 0),
			originalVec: V(1, 0, 0),
			targetVec:   V(0, 1, 7),
		}, V(0, 0, 0)},
		{"100_no_rotate", input{
			point:       V(1, 0, 0),
			originalVec: V(1, 0, 0),
			targetVec:   V(1, 0, 0),
		}, V(1, 0, 0)},
		{"100_180deg", input{
			point:       V(1, 0, 0),
			originalVec: V(1, 0, 0),
			targetVec:   V(-1, 0, 0),
		}, V(-1, 0, 0)},
		{"100_rotate_y", input{
			point:       V(1, 0, 0),
			originalVec: V(1, 0, 0),
			targetVec:   V(0, 1, 0),
		}, V(0, 1, 0)},
		{"111_rotate_y", input{
			point:       V(1, 1, 1),
			originalVec: V(1, 0, 0),
			targetVec:   V(0, 1, 0),
		}, V(-1, 1, 1)},
		{"100_rotate_yz", input{
			point:       V(1, 0, 0),
			originalVec: V(1, 0, 0),
			targetVec:   V(0, 1, 1),
		}, V(0, 1/math.Sqrt(2), 1/math.Sqrt(2))},
		{"100_rotate_xyz", input{
			point:       V(1, 0, 0),
			originalVec: V(1, 0, 0),
			targetVec:   V(1, 1, 1),
		}, V(1/math.Sqrt(3), 1/math.Sqrt(3), 1/math.Sqrt(3))},
		// {"100_rotate_xyz", input{
		// 	point:       V(2, 2, 0),
		// 	originalVec: V(1, 0, 0),
		// 	targetVec:   V(0, 1, 1),
		// }, V(2, 2/math.Sqrt(2), 2/math.Sqrt(2))},
		// {"123_no_rotate", input{
		// 	point:       V(1, 2, 3),
		// 	originalVec: V(1, 0, 0),
		// 	targetVec:   V(1, 0, 0),
		// }, V(1, 2, 3)},
		// {"123_rotate_y", input{
		// 	point:       V(1, 2, 3),
		// 	originalVec: V(1, 0, 0),
		// 	targetVec:   V(0, 1, 0),
		// }, V(-2, 1, 3)},
		// {"90deg", input{
		// 	point:       V(1, 2, 3),
		// 	originalVec: V(1, 0, 0),
		// 	targetVec:   V(0, 0, 1),
		// }, V(-3, 2, 1)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := rotate(test.input.point, test.input.originalVec, test.input.targetVec)
			if (math.Abs(test.expect.X-actual.X) > 0.000001) || (math.Abs(test.expect.Y-actual.Y) > 0.000001) || (math.Abs(test.expect.Z-actual.Z) > 0.000001) {
				t.Errorf("rotate(%v) = %v, want %v", test.input, actual, test.expect)
			}
		})
	}
}

func TestRotateSpeaker(t *testing.T) {
	type input struct {
		speaker Speaker
	}

	tests := []struct {
		name    string
		input   input
		expects []pt.Vector
	}{
		{
			"no_rotate", input{
				speaker: Speaker{
					LoudSpeakerSpec: LoudSpeakerSpec{
						Xdim: 2,
						Ydim: 2,
						Zdim: 2,
						Yoff: 1,
						Zoff: 1,
					},
					Source: Source{
						Position:        pt.Vector{},
						NormalDirection: V(1, 0, 0),
					},
				},
			}, []pt.Vector{
				V(-2, -1, -1),
				V(0, -1, -1),
			},
		},
		{
			"rotate_180deg", input{
				speaker: Speaker{
					LoudSpeakerSpec: LoudSpeakerSpec{
						Xdim: 2,
						Ydim: 2,
						Zdim: 2,
						Yoff: 1,
						Zoff: 1,
					},
					Source: Source{
						Position:        pt.Vector{},
						NormalDirection: V(-1, 0, 0),
					},
				},
			}, []pt.Vector{
				V(2, 1, -1),
				V(0, 1, -1),
			},
		},
		{
			"rotate_90deg_y", input{
				speaker: Speaker{
					LoudSpeakerSpec: LoudSpeakerSpec{
						Xdim: 2,
						Ydim: 2,
						Zdim: 2,
						Yoff: 1,
						Zoff: 1,
					},
					Source: Source{
						Position:        pt.Vector{},
						NormalDirection: V(0, 1, 0),
					},
				},
			}, []pt.Vector{
				V(-1, -2, -1),
				V(1, 0, -1),
			},
		},
		{
			"rotate_90deg_z", input{
				speaker: Speaker{
					LoudSpeakerSpec: LoudSpeakerSpec{
						Xdim: 2,
						Ydim: 2,
						Zdim: 2,
						Yoff: 1,
						Zoff: 1,
					},
					Source: Source{
						Position:        pt.Vector{},
						NormalDirection: V(0, 0, 1),
					},
				},
			}, []pt.Vector{
				V(-1, -1, -2),
				V(0, 1, 0),
			},
		},
		{
			"rotate_45deg_y", input{
				speaker: Speaker{
					LoudSpeakerSpec: LoudSpeakerSpec{
						Xdim: 2,
						Ydim: 2,
						Zdim: 2,
						Yoff: 1,
						Zoff: 1,
					},
					Source: Source{
						Position:        pt.Vector{},
						NormalDirection: V(1, 1, 0),
					},
				},
			}, []pt.Vector{
				V(math.Sqrt(2)/2, -math.Sqrt(2)/2, -1),
			},
		},
		{
			"rotate_45deg_y", input{
				speaker: Speaker{
					LoudSpeakerSpec: LoudSpeakerSpec{
						Xdim: 2,
						Ydim: 2,
						Zdim: 2,
						Yoff: 1,
						Zoff: 1,
					},
					Source: Source{
						Position:        V(10, 10, 10),
						NormalDirection: V(1, 1, 0),
					},
				},
			}, []pt.Vector{
				V(math.Sqrt(2)/2+10, -math.Sqrt(2)/2+10, 9),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := test.input.speaker.vertices()
		expectLoop:
			for _, expect := range test.expects {
				for _, actualV := range actual {
					if (math.Abs(expect.X-actualV.X) < 0.000001) && (math.Abs(expect.Y-actualV.Y) < 0.000001) && (math.Abs(expect.Z-actualV.Z) < 0.000001) {
						break expectLoop
					}
				}
				t.Errorf("%s:\ndid not find vertex %s in:\nnew:  %v\norig: %v", test.name, pretty(expect), prettyArr(actual), prettyArr(test.input.speaker.verticesUnrotated()))
			}
		})
	}
}

func TestSampleDeterminism(t *testing.T) {
	almostMatch := func(a, b float64) bool {
		return math.Abs(a-b) < 0.00001
	}

	hDirMap := map[float64]float64{0: 0, 10: -5, 30: -10, 60: -15, 90: -20}
	vDirMap := map[float64]float64{0: 0, 10: -5, 30: -10, 60: -15, 90: -20}
	pos := V(10, 10, 10)
	dir := V(1, 1, 0)

	// Build the spec from scratch each time, just in case there is something nondeterministic in its construction
	s1 := NewSpeaker(LoudSpeakerSpec{Xdim: 2, Ydim: 2, Zdim: 2, HDirectivityMap: hDirMap, VDirectivityMap: vDirMap}, pos, dir, "")
	s2 := NewSpeaker(LoudSpeakerSpec{Xdim: 2, Ydim: 2, Zdim: 2, HDirectivityMap: hDirMap, VDirectivityMap: vDirMap}, pos, dir, "")

	samples1 := s1.Sample(10_000, 90, 90)
	samples2 := s2.Sample(10_000, 90, 90)

	for i := 0; i < len(samples1); i++ {

		if !almostMatch(samples1[i].Ray.Origin.X, samples2[i].Ray.Origin.X) {
			t.Errorf("samples %v+ != %v+\n\tRay.Origin.X %f != %f", samples1[i], samples2[i], samples1[i].Ray.Origin.X, samples2[i].Ray.Origin.X)
		}
		if !almostMatch(samples1[i].Ray.Origin.Y, samples2[i].Ray.Origin.Y) {
			t.Errorf("samples %v+ != %v+\n\tRay.Origin.Y %f != %f", samples1[i], samples2[i], samples1[i].Ray.Origin.Y, samples2[i].Ray.Origin.Y)
		}
		if !almostMatch(samples1[i].Ray.Origin.Z, samples2[i].Ray.Origin.Z) {
			t.Errorf("samples %v+ != %v+\n\tRay.Origin.Z %f != %f", samples1[i], samples2[i], samples1[i].Ray.Origin.Z, samples2[i].Ray.Origin.Z)
		}
		if !almostMatch(samples1[i].Ray.Direction.X, samples2[i].Ray.Direction.X) {
			t.Errorf("samples %v+ != %v+\n\tRay.Direction.X %f != %f", samples1[i], samples2[i], samples1[i].Ray.Direction.X, samples2[i].Ray.Direction.X)
		}
		if !almostMatch(samples1[i].Ray.Direction.Y, samples2[i].Ray.Direction.Y) {
			t.Errorf("samples %v+ != %v+\n\tRay.Direction.Y %f != %f", samples1[i], samples2[i], samples1[i].Ray.Direction.Y, samples2[i].Ray.Direction.Y)
		}
		if !almostMatch(samples1[i].Ray.Direction.Z, samples2[i].Ray.Direction.Z) {
			t.Errorf("samples %v+ != %v+\n\tRay.Direction.Z %f != %f", samples1[i], samples2[i], samples1[i].Ray.Direction.Z, samples2[i].Ray.Direction.Z)
		}
		if !almostMatch(samples1[i].Gain, samples2[i].Gain) {
			t.Errorf("samples %v+ != %v+\n\tGain %f != %f", samples1[i], samples2[i], samples1[i].Gain, samples2[i].Gain)
		}
	}
}

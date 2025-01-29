package room

import (
	"math"
	"testing"

	"github.com/fogleman/pt/pt"
)

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
			// if !reflect.DeepEqual(actual, test.expect) {
			// 	assert.InDelta(test.expect.X, actual.X, 0.000001)
			// 	assert.InDelta(test.expect.Y, actual.Y, 0.000001)
			// 	assert.InDelta(test.expect.Z, actual.Z, 0.000001)
			// }
		})
	}
}

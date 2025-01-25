package room

import (
	"github.com/fogleman/pt/pt"
)

// V is a shorthand constructor for pt.Vector
func V(X, Y, Z float64) pt.Vector {
	return pt.Vector{X: X, Y: Y, Z: Z}
}

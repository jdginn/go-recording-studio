//go:build !verify_reflections
// +build !verify_reflections

package room

import "github.com/fogleman/pt/pt"

// Empty stub that will be optimized out
func verifyReflectionLaw(incident pt.Ray, normal pt.Vector, reflected pt.Ray) {
	// Empty function will be entirely optimized out in release builds
}

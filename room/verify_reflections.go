//go:build verify_reflections
// +build verify_reflections

package room

import (
	"fmt"
	"math"

	"github.com/fogleman/pt/pt"
)

// Constants for verification
const (
	lengthEpsilon      = 1e-7
	angleEpsilon       = 1e-7
	dotProductEpsilon  = 1e-7
	coplanarityEpsilon = 1e-6
)

func init() {
	fmt.Println("Angle verification enabled.")
}

func verifyReflectionLaw(incident pt.Ray, normal pt.Vector, reflected pt.Ray) {
	fmt.Println("Verifying angle...")
	// 1. Angle of incidence should equal angle of reflection
	incidentAngle := math.Acos(incident.Direction.Dot(normal))
	reflectedAngle := math.Acos(reflected.Direction.Dot(normal))
	if math.Abs(incidentAngle-reflectedAngle) < angleEpsilon {
		panic("Angle of incidence should equal angle of reflection")
	}

	// // 2. Incident, normal, and reflected vectors should be coplanar
	// cross := incident.Direction.Cross(reflected.Direction)
	// if cross.Dot(normal) < coplanarityEpsilon {
	// 	panic("Vectors should be coplanar")
	// }
	//
	i := incident.Direction.Normalize()
	n := normal.Normalize()
	r := reflected.Direction.Normalize()

	// Calculate triple product in a more stable way
	triple := math.Abs(i.Dot(n.Cross(r)))

	if triple >= coplanarityEpsilon {
		panic(fmt.Sprintf("Coplanarity check failed!\nTriple product: %v\nIncident: %v\nNormal: %v\nReflected: %v",
			triple, i, n, r))
	}
}

func verifyEnergyConservation(hit pt.HitInfo) bool {
	// Reflected direction should maintain unit length
	return math.Abs(hit.Ray.Direction.Length()-1.0) < lengthEpsilon
}

func verifyNormalOrientation(hit pt.HitInfo, incident pt.Ray) bool {
	// Normal should point against incident ray
	dotProduct := hit.Normal.Dot(incident.Direction)
	return dotProduct < 0 // Should be negative for hits from outside
}

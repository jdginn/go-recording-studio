package room

import (
	"math"
	"testing"

	"github.com/jdginn/go-recording-studio/pt"
)

func TestOmniMicrophone(t *testing.T) {
	mic := Omni{Pos: pt.V(1, 2, 3)}

	// Test position
	pos := mic.Position()
	if pos.X != 1 || pos.Y != 2 || pos.Z != 3 {
		t.Errorf("Position() = %v, want (1, 2, 3)", pos)
	}

	// Test directional gain (should be 1.0 for all directions)
	directions := []pt.Vector{
		pt.V(1, 0, 0),
		pt.V(0, 1, 0),
		pt.V(0, 0, 1),
		pt.V(-1, 0, 0),
		pt.V(1, 1, 1),
	}

	for _, dir := range directions {
		gain := mic.DirectionalGain(dir)
		if gain != 1.0 {
			t.Errorf("DirectionalGain(%v) = %f, want 1.0", dir, gain)
		}
	}
}

func TestCardioidMicrophone(t *testing.T) {
	// Mic pointing in +X direction
	mic := Cardioid{
		Pos:    pt.V(0, 0, 0),
		Normal: pt.V(1, 0, 0),
		BackDB: -24.0,
	}

	// Test position
	pos := mic.Position()
	if pos.X != 0 || pos.Y != 0 || pos.Z != 0 {
		t.Errorf("Position() = %v, want (0, 0, 0)", pos)
	}

	// Test directional gain
	// Front (parallel to normal): should be close to 1.0
	frontGain := mic.DirectionalGain(pt.V(1, 0, 0))
	if math.Abs(frontGain-1.0) > 0.01 {
		t.Errorf("Front gain = %f, want ~1.0", frontGain)
	}

	// Back (opposite to normal): should be close to -24dB = 10^(-24/20) ≈ 0.063
	backGain := mic.DirectionalGain(pt.V(-1, 0, 0))
	expectedBack := math.Pow(10, -24.0/20.0)
	if math.Abs(backGain-expectedBack) > 0.01 {
		t.Errorf("Back gain = %f, want ~%f", backGain, expectedBack)
	}

	// Side (perpendicular to normal): should be in between
	sideGain := mic.DirectionalGain(pt.V(0, 1, 0))
	if sideGain <= expectedBack || sideGain >= 1.0 {
		t.Errorf("Side gain = %f, should be between %f and 1.0", sideGain, expectedBack)
	}
}

func TestCardioidDefaultBackDB(t *testing.T) {
	// Test that BackDB defaults to -24 when set to 0
	mic := Cardioid{
		Pos:    pt.V(0, 0, 0),
		Normal: pt.V(1, 0, 0),
		BackDB: 0, // Should use default -24
	}

	backGain := mic.DirectionalGain(pt.V(-1, 0, 0))
	expectedBack := math.Pow(10, -24.0/20.0)
	if math.Abs(backGain-expectedBack) > 0.01 {
		t.Errorf("Back gain with default = %f, want ~%f", backGain, expectedBack)
	}
}

func TestCardioidSmoothnessProperty(t *testing.T) {
	// Verify that gain varies smoothly from front to back
	mic := Cardioid{
		Pos:    pt.V(0, 0, 0),
		Normal: pt.V(1, 0, 0),
		BackDB: -24.0,
	}

	// Test at several angles
	angles := []float64{0, 45, 90, 135, 180}
	prevGain := 2.0 // Start with impossible value

	for _, angleDeg := range angles {
		angleRad := angleDeg * math.Pi / 180.0
		// Ray direction at this angle from normal (in XY plane)
		dir := pt.V(math.Cos(angleRad), math.Sin(angleRad), 0)
		gain := mic.DirectionalGain(dir)

		// Gain should decrease monotonically as angle increases
		if prevGain != 2.0 && gain > prevGain {
			t.Errorf("Gain not monotonically decreasing: at %f° gain=%f > previous gain=%f", angleDeg, gain, prevGain)
		}
		prevGain = gain
	}
}

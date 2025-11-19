package room

import (
	"math"

	"github.com/jdginn/go-recording-studio/pt"
)

// Microphone represents a microphone with position and directional characteristics
type Microphone interface {
	// Position returns the position of the microphone
	Position() pt.Vector
	// DirectionalGain returns the linear amplitude gain for a ray arriving from the given direction
	DirectionalGain(rayDir pt.Vector) float64
}

// Omni is an omnidirectional microphone with uniform gain in all directions
type Omni struct {
	Pos pt.Vector
}

// Position returns the position of the omnidirectional microphone
func (o Omni) Position() pt.Vector {
	return o.Pos
}

// DirectionalGain returns 1.0 for all directions (omnidirectional response)
func (o Omni) DirectionalGain(rayDir pt.Vector) float64 {
	return 1.0
}

// Cardioid is a cardioid microphone with directional response
type Cardioid struct {
	Pos    pt.Vector
	Normal pt.Vector
	BackDB float64 // Attenuation at the back in dB (default -24 dB)
}

// Position returns the position of the cardioid microphone
func (c Cardioid) Position() pt.Vector {
	return c.Pos
}

// DirectionalGain returns the linear amplitude gain based on the angle between
// the ray direction and the microphone's normal vector.
// Uses a cardioid pattern: gain varies smoothly from 1.0 (front) to a minimum at the back.
func (c Cardioid) DirectionalGain(rayDir pt.Vector) float64 {
	// Normalize both vectors
	micNormal := c.Normal.Normalize()
	rayDirection := rayDir.Normalize()

	// Calculate cosine of angle between ray direction and microphone normal
	// Note: We want the angle between the ray direction and the mic's "listening" direction
	cosTheta := micNormal.Dot(rayDirection)

	// Use default BackDB of -24 dB if not set (or if set to 0)
	backDB := c.BackDB
	if backDB == 0 {
		backDB = -24.0
	}

	// Convert back attenuation from dB to linear amplitude
	backLinear := math.Pow(10, backDB/20.0)

	// Cardioid pattern: interpolate between 1.0 (front, cosTheta=1) and backLinear (back, cosTheta=-1)
	// Using (1 + cosTheta) / 2 to map [-1, 1] to [0, 1], then scale to [backLinear, 1]
	normalizedCos := (1 + cosTheta) / 2.0
	gain := backLinear + (1.0-backLinear)*normalizedCos

	return gain
}

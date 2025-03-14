package config

import (
	"fmt"
)

// ValidationError represents a structured validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Validate performs validation on the entire configuration
func (c *ExperimentConfig) Validate() []ValidationError {
	var errors []ValidationError
	errors = append(errors, c.Materials.Validate()...)
	errors = append(errors, c.SurfaceAssignments.Validate(c.Materials)...)
	errors = append(errors, c.Speaker.Validate()...)
	errors = append(errors, c.ListeningTriangle.Validate()...)
	errors = append(errors, c.Simulation.Validate()...)
	errors = append(errors, c.Input.Validate()...)
	return errors
}

func (m *Materials) Validate() []ValidationError {
	var errors []ValidationError

	if m.Inline == nil && m.FromFile == "" {
		errors = append(errors, ValidationError{
			Field:   "materials",
			Message: "either inline or from_file must be specified",
		})
		return errors
	}

	// Validate inline materials if present
	for name, material := range m.Inline {
		if material.Absorption < 0 || material.Absorption > 1 {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("materials.inline.%s.absorption", name),
				Message: "absorption coefficient must be between 0.0 and 1.0",
			})
		}
	}

	return errors
}

func (sa *SurfaceAssignments) Validate(materials *Materials) []ValidationError {
	var errors []ValidationError

	if sa.Inline == nil && sa.FromFile == "" {
		errors = append(errors, ValidationError{
			Field:   "surface_assignments",
			Message: "either inline or from_file must be specified",
		})
		return errors
	}

	if sa.Inline != nil {
		// Check for default material
		defaultMaterial, hasDefault := sa.Inline["default"]
		if !hasDefault {
			errors = append(errors, ValidationError{
				Field:   "surface_assignments.inline",
				Message: "must include a default material",
			})
		}

		// Validate material references
		for surface, material := range sa.Inline {
			if !materials.HasMaterial(material) {
				errors = append(errors, ValidationError{
					Field:   fmt.Sprintf("surface_assignments.inline.%s", surface),
					Message: fmt.Sprintf("references undefined material '%s'", material),
				})
			}
		}
	}

	return errors
}

func (s *Speaker) Validate() []ValidationError {
	var errors []ValidationError

	if s.Model == "" {
		errors = append(errors, ValidationError{
			Field:   "speaker.model",
			Message: "model identifier is required",
		})
	}

	// Validate dimensions
	errors = append(errors, validatePositive("speaker.dimensions.x", s.Dimensions.X)...)
	errors = append(errors, validatePositive("speaker.dimensions.y", s.Dimensions.Y)...)
	errors = append(errors, validatePositive("speaker.dimensions.z", s.Dimensions.Z)...)

	// Validate directivity angles
	for angle := range s.Directivity.Horizontal {
		errors = append(errors, validateAngleRange("speaker.directivity.horizontal", angle)...)
	}
	for angle := range s.Directivity.Vertical {
		errors = append(errors, validateAngleRange("speaker.directivity.vertical", angle)...)
	}

	return errors
}

func (lt *ListeningTriangle) Validate() []ValidationError {
	var errors []ValidationError

	errors = append(errors, validatePositive("listening_triangle.distance_from_front", lt.DistanceFromFront)...)
	errors = append(errors, validatePositive("listening_triangle.distance_from_center", lt.DistanceFromCenter)...)
	errors = append(errors, validatePositive("listening_triangle.source_height", lt.SourceHeight)...)
	errors = append(errors, validatePositive("listening_triangle.listen_height", lt.ListenHeight)...)

	if lt.ReferenceNormal != [3]float64{0, 0, 0} {
		errors = append(errors, validateUnitVector("listening_triangle.reference_normal", lt.ReferenceNormal)...)
	}

	return errors
}

func (s *Simulation) Validate() []ValidationError {
	var errors []ValidationError

	errors = append(errors, validatePositive("simulation.rfz_radius", s.RFZRadius)...)
	errors = append(errors, validatePositive("simulation.shot_count", float64(s.ShotCount))...)
	errors = append(errors, validatePositive("simulation.shot_angle_range", s.ShotAngleRange)...)
	errors = append(errors, validateNonNegative("simulation.order", float64(s.Order))...)
	errors = append(errors, validatePositive("simulation.time_threshold_ms", s.TimeThresholdMS)...)

	return errors
}

func (i *Input) Validate() []ValidationError {
	var errors []ValidationError

	if i.Mesh.Path == "" {
		errors = append(errors, ValidationError{
			Field:   "input.mesh.path",
			Message: "mesh path is required",
		})
	}

	return errors
}

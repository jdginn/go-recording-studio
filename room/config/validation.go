package config

import (
	"fmt"
	"math"
	"strings"
)

// Validation helper functions
func validatePositive(field string, value float64) []ValidationError {
	if value <= 0 {
		return []ValidationError{{
			Field:   field,
			Message: "must be positive",
		}}
	}
	return nil
}

func validateNonNegative(field string, value float64) []ValidationError {
	if value < 0 {
		return []ValidationError{{
			Field:   field,
			Message: "must be non-negative",
		}}
	}
	return nil
}

func validateInRange(field string, value, min, max float64) []ValidationError {
	if value < min || value > max {
		return []ValidationError{{
			Field:   field,
			Message: fmt.Sprintf("must be between %v and %v", min, max),
		}}
	}
	return nil
}

func validateUnitVector(field string, vec [3]float64) []ValidationError {
	length := math.Sqrt(vec[0]*vec[0] + vec[1]*vec[1] + vec[2]*vec[2])
	if math.Abs(length-1.0) > 1e-6 {
		return []ValidationError{{
			Field:   field,
			Message: "must be a unit vector",
		}}
	}
	return nil
}

func validateAngleRange(field string, angle float64) []ValidationError {
	if angle < -180 || angle > 180 {
		return []ValidationError{{
			Field:   field,
			Message: "angle must be between -180 and 180 degrees",
		}}
	}
	return nil
}

// ValidationError represents a structured validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// New function to format validation errors nicely
func FormatValidationErrors(errs []ValidationError) string {
	if len(errs) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("Validation Errors:\n")

	// Group errors by category
	categories := map[string][]ValidationError{}
	for _, err := range errs {
		category := strings.Split(err.Field, ".")[0]
		categories[category] = append(categories[category], err)
	}

	// Print errors by category
	for category, categoryErrors := range categories {
		b.WriteString(fmt.Sprintf("\n%s:\n", strings.ToUpper(category)))
		for _, err := range categoryErrors {
			// Remove category prefix from field for cleaner display
			field := strings.TrimPrefix(err.Field, category+".")
			if field == category {
				field = "general"
			}
			b.WriteString(fmt.Sprintf("  - %s: %s\n", field, err.Message))
		}
	}

	return b.String()
}

// Validate performs validation on the entire configuration
func (c *ExperimentConfig) Validate() []ValidationError {
	var errors []ValidationError
	errors = append(errors, c.Materials.Validate()...)
	errors = append(errors, c.SurfaceAssignments.Validate(&c.Materials)...)
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
		_, hasDefault := sa.Inline["default"]
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

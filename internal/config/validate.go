package config

import "fmt"

// ValidationError is a single validation problem found in a workspace config.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

// ValidationErrors is a collection of ValidationError values.
type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	if len(ve) == 1 {
		return ve[0].Error()
	}
	msg := fmt.Sprintf("%d validation errors:", len(ve))
	for _, e := range ve {
		msg += "\n  - " + e.Error()
	}
	return msg
}

// Validate checks a WorkspaceConfig for correctness per spec §6.9.
// It returns a ValidationErrors (non-nil, non-empty) when problems are found,
// or nil when the config is valid. Callers must check errors.As or cast to
// ValidationErrors to retrieve all issues at once.
func Validate(cfg WorkspaceConfig) error {
	var errs ValidationErrors

	// Track effective names seen across repos and folders for collision detection.
	repoNames := make(map[string]int) // name → first index
	folderNames := make(map[string]struct{})

	for i, r := range cfg.Repos {
		field := fmt.Sprintf("repos[%d]", i)

		// URL is required.
		if r.URL == "" {
			errs = append(errs, ValidationError{Field: field, Message: "url is required"})
		}

		// Tags must be non-empty strings if provided.
		for j, tag := range r.Tags {
			if tag == "" {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("%s.tags[%d]", field, j),
					Message: "tag must not be empty",
				})
			}
		}

		// Group must be non-empty if set (TOML zero value is "").
		// Spec says "non-empty if set"; an unset group is fine (empty string is
		// the zero/unset value here), so we only flag explicitly-empty tags, not
		// missing groups.  Groups have no "set but empty" distinction in TOML
		// (empty string == not provided), so we skip the group check here.
		// DECISION: group = "" means "no group"; not an error.

		// Collect name for collision check.
		if r.URL != "" {
			name := r.EffectiveName()
			if prev, seen := repoNames[name]; seen {
				errs = append(errs, ValidationError{
					Field: field,
					Message: fmt.Sprintf(
						"derived name %q collides with repos[%d]; provide an explicit name field to disambiguate",
						name, prev,
					),
				})
			} else {
				repoNames[name] = i
			}
		}
	}

	// Collect folder names and check repo↔folder collisions.
	for i, f := range cfg.Folders {
		field := fmt.Sprintf("folders[%d]", i)

		if f.Name == "" {
			errs = append(errs, ValidationError{Field: field, Message: "name is required"})
			continue
		}

		if _, seen := folderNames[f.Name]; seen {
			errs = append(errs, ValidationError{
				Field:   field,
				Message: fmt.Sprintf("folder name %q is duplicated", f.Name),
			})
		}
		folderNames[f.Name] = struct{}{}

		// Check repo↔folder collision.
		if _, clash := repoNames[f.Name]; clash {
			errs = append(errs, ValidationError{
				Field:   field,
				Message: fmt.Sprintf("folder name %q collides with a repo of the same name", f.Name),
			})
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

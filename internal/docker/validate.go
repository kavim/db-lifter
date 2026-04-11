package docker

import (
	"fmt"
	"regexp"
)

var identifierRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// ValidateIdentifier checks a MySQL user or database name for safe use without
// shell interpolation. Only unquoted-style identifiers (alphanumeric, _, -).
func ValidateIdentifier(name, field string) error {
	if name == "" {
		return fmt.Errorf("%s is required", field)
	}
	if len(name) > 64 {
		return fmt.Errorf("%s exceeds maximum length of 64", field)
	}
	if !identifierRe.MatchString(name) {
		return fmt.Errorf("%s must be 1–64 chars, start with a letter or digit, and contain only letters, digits, underscore, or hyphen", field)
	}
	return nil
}

// ValidateConfig validates user and database fields before any docker/mysql call.
func ValidateConfig(cfg Config) error {
	if err := ValidateIdentifier(cfg.User, "user"); err != nil {
		return err
	}
	if err := ValidateIdentifier(cfg.Database, "database"); err != nil {
		return err
	}
	return nil
}

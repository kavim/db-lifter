package docker

import (
	"strings"
	"testing"
)

func TestValidateIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		field   string
		wantErr bool
	}{
		{"ok user", "root", "user", false},
		{"ok db", "my_app", "database", false},
		{"ok hyphen", "db-1", "database", false},
		{"empty", "", "user", true},
		{"bad start hyphen", "-x", "database", true},
		{"bad space", "a b", "database", true},
		{"bad backtick", "a`b", "database", true},
		{"too long", strings.Repeat("a", 65), "database", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIdentifier(tt.value, tt.field)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	if err := ValidateConfig(Config{User: "root", Database: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateConfig(Config{User: "", Database: "test"}); err == nil {
		t.Fatal("expected error for empty user")
	}
}

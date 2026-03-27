package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateInstanceName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid simple", "vm1", true},
		{"valid with hyphen", "my-vm", true},
		{"valid with numbers", "test-vm-1", true},
		{"valid two chars", "ab", true},
		{"invalid starts with number", "123vm", false},
		{"invalid ends with hyphen", "vm-", false},
		{"invalid uppercase", "VM", false},
		{"invalid special chars", "vm@test", false},
		{"invalid underscore", "vm_test", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInstanceName(tt.input)
			if tt.expected {
				assert.NoError(t, err, "expected no error for %s", tt.input)
			} else {
				assert.Error(t, err, "expected error for %s", tt.input)
			}
		})
	}
}

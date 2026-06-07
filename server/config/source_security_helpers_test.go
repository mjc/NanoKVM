package config

import (
	"os"
	"strings"
	"testing"
)

func readSource(t *testing.T, path string) string {
	t.Helper()

	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(source)
}

func containsAll(content string, needles ...string) bool {
	for _, needle := range needles {
		if !strings.Contains(content, needle) {
			return false
		}
	}
	return true
}

func containsAny(content string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(content, needle) {
			return true
		}
	}
	return false
}

type sourceSecurityContract struct {
	name       string
	path       string
	vulnerable []string
	fixedBy    []string
	message    string
}

func runSourceSecurityContracts(t *testing.T, cases []sourceSecurityContract, minimum int, label string) {
	t.Helper()

	if len(cases) < minimum {
		t.Fatalf("expected at least %d %s auth contracts, got %d", minimum, label, len(cases))
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content := readSource(t, tc.path)
			if containsAll(content, tc.vulnerable...) && !containsAny(content, tc.fixedBy...) {
				t.Fatal(tc.message)
			}
		})
	}
}

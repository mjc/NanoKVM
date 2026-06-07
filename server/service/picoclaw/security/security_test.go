package security

import (
	"archive/tar"
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestCopyWithProgressRejectsOversizedContentLength(t *testing.T) {
	t.Parallel()

	var dst bytes.Buffer
	err := CopyWithProgress(context.Background(), &dst, strings.NewReader("abc"), 4, 3, nil)
	if err == nil {
		t.Fatal("CopyWithProgress succeeded with oversized content length")
	}
}

func TestCopyWithProgressRejectsOversizedStream(t *testing.T) {
	t.Parallel()

	var dst bytes.Buffer
	err := CopyWithProgress(context.Background(), &dst, strings.NewReader("abcd"), -1, 3, nil)
	if err == nil {
		t.Fatal("CopyWithProgress succeeded with oversized stream")
	}
}

func TestPicoclawBinaryEntryRejectsUnsafeArchiveNames(t *testing.T) {
	t.Parallel()

	rejected := []*tar.Header{
		{Name: "../picoclaw", Typeflag: tar.TypeReg},
		{Name: "/picoclaw", Typeflag: tar.TypeReg},
		{Name: "dir/../../picoclaw", Typeflag: tar.TypeReg},
		{Name: "picoclaw", Typeflag: tar.TypeSymlink},
	}
	for _, header := range rejected {
		t.Run(header.Name, func(t *testing.T) {
			if IsPicoclawBinaryEntry(header) {
				t.Fatalf("IsPicoclawBinaryEntry accepted %#v", header)
			}
		})
	}
}

func TestPicoclawScriptArgsUsesFixedArgv(t *testing.T) {
	t.Parallel()

	got, ok := PicoclawScriptArgs("/etc/init.d/S96picoclaw", "start")
	want := []string{"/etc/init.d/S96picoclaw", "start"}
	if !ok {
		t.Fatal("PicoclawScriptArgs rejected valid action")
	}
	if len(got) != len(want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args = %#v, want %#v", got, want)
		}
	}
}

func TestSanitizeRuntimeOutputRedactsCommandOutput(t *testing.T) {
	t.Parallel()

	got := SanitizeRuntimeOutput([]byte("token=secret\nstack trace"))
	if strings.Contains(got, "secret") || strings.Contains(got, "stack trace") {
		t.Fatalf("SanitizeRuntimeOutput leaked output: %q", got)
	}
}

func TestSessionIDValidationRejectsPathLikeIDs(t *testing.T) {
	t.Parallel()

	for _, sessionID := range []string{"../x", "nested/x", "agent:main:pico", "", strings.Repeat("a", 129)} {
		t.Run(sessionID, func(t *testing.T) {
			if IsValidSessionID(sessionID) {
				t.Fatalf("IsValidSessionID(%q) = true", sessionID)
			}
		})
	}
}

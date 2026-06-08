package tailscale

import "testing"

func TestParseStatusOutputAcceptsLeadingPreamble(t *testing.T) {
	status, err := parseStatusOutput([]byte("warning line\n{\"BackendState\":\"Running\",\"CurrentTailnet\":{\"Name\":\"tailnet\"}}"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.BackendState != "Running" {
		t.Fatalf("expected backend state Running, got %q", status.BackendState)
	}
	if status.CurrentTailnet.Name != "tailnet" {
		t.Fatalf("expected tailnet name tailnet, got %q", status.CurrentTailnet.Name)
	}
}

func TestParseStatusOutputRejectsMissingJSON(t *testing.T) {
	_, err := parseStatusOutput([]byte("warning line\nno json here"))
	if err == nil {
		t.Fatalf("expected error for output without json payload")
	}
}

func TestExtractLoginURLStripsWhitespace(t *testing.T) {
	got := extractLoginURL(" \thttps://login.tailscale.com/a/b \n")
	if got != "https://login.tailscale.com/a/b" {
		t.Fatalf("expected compact login URL, got %q", got)
	}
}

func TestExtractLoginURLIgnoresNonURLLines(t *testing.T) {
	if got := extractLoginURL("not a login link"); got != "" {
		t.Fatalf("expected empty URL, got %q", got)
	}
}

package download

import "testing"

func TestBlocksPrivateAddressRejectsLocalhost(t *testing.T) {
	if !blocksPrivateAddress("localhost") {
		t.Fatal("localhost should be rejected as a private download target")
	}
}

func TestBlocksPrivateAddressRejectsInvalidHosts(t *testing.T) {
	if !blocksPrivateAddress("invalid.invalid.invalid") {
		t.Fatal("unresolvable hosts should fail closed")
	}
}

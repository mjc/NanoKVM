package usb

import "testing"

func TestSetRNDISEnabledTogglesFlagAndLink(t *testing.T) {
	withFakeGadget(t)

	if err := SetRNDISEnabled(&fakeHID{}, true); err != nil {
		t.Fatal(err)
	}
	assertFile(t, RNDISFlag, "")
	if !Exists(RNDISFunction) {
		t.Fatal("RNDIS function was not created")
	}
	assertSymlink(t, RNDISLink, RNDISFunction)

	if err := SetRNDISEnabled(&fakeHID{}, false); err != nil {
		t.Fatal(err)
	}
	if Exists(RNDISFlag) {
		t.Fatal("RNDIS flag still exists after disabling RNDIS")
	}
	if Exists(RNDISLink) {
		t.Fatal("RNDIS link still exists after disabling RNDIS")
	}
}

func TestPrepareRNDISFunctionCreatesAndLinks(t *testing.T) {
	withFakeGadget(t)

	if err := prepareRNDISFunction(); err != nil {
		t.Fatal(err)
	}

	if !Exists(RNDISFunction) {
		t.Fatal("RNDIS function was not created")
	}
	assertSymlink(t, RNDISLink, RNDISFunction)
}

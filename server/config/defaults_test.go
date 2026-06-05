package config

import "testing"

func TestCheckDefaultValue(t *testing.T) {
	instance = Config{}
	checkDefaultValue()
	if instance.JWT.SecretKey == "" {
		t.Fatal("secret key was not defaulted")
	}
	if instance.JWT.RefreshTokenDuration != 2678400 {
		t.Fatalf("refresh duration = %d", instance.JWT.RefreshTokenDuration)
	}
	if instance.Stun != "stun.l.google.com:19302" {
		t.Fatalf("stun = %q", instance.Stun)
	}
	if instance.Authentication != "enable" {
		t.Fatalf("authentication = %q", instance.Authentication)
	}
	if instance.Hardware.Version != HWVersionAlpha {
		t.Fatalf("hardware = %s", instance.Hardware.Version)
	}
}

func TestRegenerateSecretKey(t *testing.T) {
	instance = Config{
		JWT: JWT{
			SecretKey:            "old",
			RevokeTokensOnLogout: true,
		},
	}
	RegenerateSecretKey()
	if instance.JWT.SecretKey == "" || instance.JWT.SecretKey == "old" {
		t.Fatalf("secret key was not regenerated: %q", instance.JWT.SecretKey)
	}

	instance = Config{
		JWT: JWT{
			SecretKey:            "old",
			RevokeTokensOnLogout: false,
		},
	}
	RegenerateSecretKey()
	if instance.JWT.SecretKey != "old" {
		t.Fatalf("secret key changed unexpectedly: %q", instance.JWT.SecretKey)
	}
}

func TestHardwareVersionStringsAndFallback(t *testing.T) {
	for version, want := range map[HWVersion]string{
		HWVersionAlpha: "Alpha",
		HWVersionBeta:  "Beta",
		HWVersionPcie:  "PCIE",
		HWVersion(99):  "Unknown",
	} {
		if got := version.String(); got != want {
			t.Fatalf("%d string = %q", version, got)
		}
	}

	if got := GetHwVersion(); got != HWVersionAlpha {
		t.Fatalf("missing hw file fallback = %s", got)
	}
	if got := getHardware(); got.Version != HWVersionAlpha || got.GPIOReset == "" {
		t.Fatalf("hardware fallback = %+v", got)
	}
}

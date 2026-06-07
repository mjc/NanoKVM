package network

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

func TestBootWifiMigrationKeepsPasswordOwnerOnly(t *testing.T) {
	content := readSource(t, "../../../kvmapp/system/init.d/S30wifi")
	if strings.Contains(content, "chmod 644 /etc/kvm/wifi.ssid /etc/kvm/wifi.pass") {
		t.Fatal("S30wifi migrates /boot/wifi.pass with world-readable permissions")
	}
	if !strings.Contains(content, "chmod 600 /etc/kvm/wifi.pass") {
		t.Fatal("S30wifi should explicitly chmod /etc/kvm/wifi.pass to 0600 after migration")
	}
}

func TestGeneratedWifiConfigsContainingPSKsAreOwnerOnly(t *testing.T) {
	content := readSource(t, "../../../kvmapp/system/init.d/S30wifi")
	if !strings.Contains(content, "chmod 600 /etc/wpa_supplicant.conf") {
		t.Fatal("S30wifi should chmod /etc/wpa_supplicant.conf to 0600 because it contains station PSK material")
	}
	if !strings.Contains(content, "chmod 600 /etc/hostapd.conf") {
		t.Fatal("S30wifi should chmod /etc/hostapd.conf to 0600 because it contains the AP passphrase")
	}
}

func TestGoWifiConnectStoresPasswordOwnerOnly(t *testing.T) {
	content := readSource(t, "wifi.go")
	if strings.Contains(content, "os.WriteFile(WiFiPasswd, []byte(password), 0o644)") {
		t.Fatal("Go Wi-Fi connect path stores /etc/kvm/wifi.pass as 0644")
	}
	if !containsAny(content, "WritePrivateFile", "0o600") {
		t.Fatal("Go Wi-Fi connect path should persist wifi.pass as an owner-only file")
	}
}

func TestAPPasswordFileReadRepairsSecretPermissions(t *testing.T) {
	content := readSource(t, "wifi.go")
	if strings.Contains(content, "os.ReadFile(WiFiApPassFile)") && !strings.Contains(content, "ReadPrivateFile") {
		t.Fatal("AP password reads should repair permissions and reject symlinks")
	}
}

func TestAPWifiPasswordVerificationHasAttemptTracking(t *testing.T) {
	if !containsAny(readSource(t, "wifi.go"), "Record", "Attempt", "lockout") {
		t.Fatal("AP Wi-Fi password verification should track failed attempts or lock out repeated guesses")
	}
}

func TestAPWifiNoAuthPathDoesNotRevealModeBeforeParsingCredentialPayload(t *testing.T) {
	content := readSource(t, "wifi.go")
	modeCheck := strings.Index(content, "if !isSupported() || !isAPMode()")
	parseRequest := strings.Index(content, "proto.ParseFormRequest(c, &req)")
	if modeCheck >= 0 && parseRequest >= 0 && modeCheck < parseRequest {
		t.Fatal("AP Wi-Fi unauthenticated path should not reveal AP-mode availability before parsing the credential payload")
	}
}

func TestAuthenticatedWifiPasswordUsesEncryptedPayloadContract(t *testing.T) {
	clientContent := readSource(t, "../../../web/src/api/network.ts")
	serverContent := readSource(t, "wifi.go")
	if containsAll(clientContent, "password", "http.post('/api/network/wifi/connect', data)") &&
		!strings.Contains(clientContent, "encrypt(") &&
		!strings.Contains(serverContent, "DecodeDecrypt(req.Password)") {
		t.Fatal("authenticated Wi-Fi password payloads should use the same encrypted client/server contract as account passwords")
	}
}

func TestAPModeWifiPasswordUsesEncryptedPayloadContract(t *testing.T) {
	clientContent := readSource(t, "../../../web/src/api/network.ts")
	serverContent := readSource(t, "wifi.go")
	if containsAll(clientContent, "connectWifiNoAuth(ssid: string, password: string", "http.post('/api/network/wifi', data") &&
		!strings.Contains(clientContent, "encrypt(") &&
		!strings.Contains(serverContent, "DecodeDecrypt(req.Password)") {
		t.Fatal("AP-mode Wi-Fi station passwords should use the encrypted client/server payload contract")
	}
}

func TestAPWifiPasswordHeaderUsesEncryptedPayloadContract(t *testing.T) {
	clientContent := readSource(t, "../../../web/src/api/network.ts")
	serverContent := readSource(t, "wifi.go")
	if strings.Contains(clientContent, "'X-AP-Key': apPassword") &&
		!strings.Contains(clientContent, "encrypt(apPassword") &&
		!strings.Contains(serverContent, "DecodeDecrypt(apKey)") {
		t.Fatal("AP Wi-Fi authentication header should not send the AP password in plaintext")
	}
}

func TestAPWifiConnectDoesNotReportSuccessAfterUnhandledClientError(t *testing.T) {
	content := readSource(t, "../../../web/src/pages/wifi/index.tsx")
	catchIndex := strings.Index(content, "catch (err)")
	if catchIndex >= 0 && strings.Contains(content[catchIndex:], "setState('success')") {
		t.Fatal("AP Wi-Fi connect flow should not report success after an unhandled client/API error")
	}
}

func TestAPWifiPasswordComparisonAvoidsLengthOracle(t *testing.T) {
	content := readSource(t, "wifi.go")
	if strings.Contains(content, "subtle.ConstantTimeCompare([]byte(apKey), []byte(expectedPass))") &&
		!strings.Contains(content, "sha256") {
		t.Fatal("AP Wi-Fi password comparison should avoid ConstantTimeCompare's length-dependent early return")
	}
}

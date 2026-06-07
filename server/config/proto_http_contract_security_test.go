package config

import (
	"strings"
	"testing"
)

func TestRequestValidationDoesNotLogAuthPayloads(t *testing.T) {
	content := readSource(t, "../proto/request.go")
	if strings.Contains(content, `log.Debugf("request: %+v`) {
		t.Fatal("request validation should not log full request structs because auth payloads contain credentials")
	}
}
func TestAuthFailuresDoNotReturnHTTPStatusOK(t *testing.T) {
	content := readSource(t, "../proto/response.go")
	if strings.Contains(content, "func (r *Response) ErrRsp") && strings.Contains(content, "http.StatusOK") {
		t.Fatal("auth/API failures should use HTTP error status codes instead of encoding every error as HTTP 200")
	}
}
func TestPostAuthPayloadsBindJSONOnly(t *testing.T) {
	content := readSource(t, "../proto/request.go")
	if strings.Contains(content, "func ParseFormRequest") &&
		strings.Contains(content, "c.ShouldBind(req)") &&
		!strings.Contains(content, "ShouldBindJSON") {
		t.Fatal("POST auth payloads should be bound as JSON only so credentials cannot drift into form/query binding")
	}
}
func TestLoginRequestFieldsHaveExplicitJSONBindingTags(t *testing.T) {
	content := readSource(t, "../proto/auth.go")
	if strings.Contains(content, `Username string `+"`validate:\"required\"`") ||
		strings.Contains(content, `Password string `+"`validate:\"required\"`") {
		t.Fatal("login request fields should use explicit JSON binding tags so client/server auth payload names cannot drift")
	}
}
func TestWifiRequestFieldsHaveExplicitJSONBindingTags(t *testing.T) {
	content := readSource(t, "../proto/network.go")
	if strings.Contains(content, `Ssid     string `+"`validate:\"required\"`") ||
		strings.Contains(content, `Password string `+"`validate:\"required\"`") {
		t.Fatal("Wi-Fi credential request fields should use explicit JSON binding tags so payload names cannot drift")
	}
}

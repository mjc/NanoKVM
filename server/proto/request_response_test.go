package proto

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type queryRequest struct {
	Value string `form:"value" validate:"required"`
}

type intQueryRequest struct {
	Value int `form:"value"`
}

func TestParseQueryRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/?value=ok", nil)

	var req queryRequest
	if err := ParseQueryRequest(c, &req); err != nil {
		t.Fatal(err)
	}
	if req.Value != "ok" {
		t.Fatalf("value = %q", req.Value)
	}

	c, _ = gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/?value=not-an-int", nil)
	var intReq intQueryRequest
	if err := ParseQueryRequest(c, &intReq); err == nil {
		t.Fatal("expected query parse error")
	}
}

func TestParseFormRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("value=ok"))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var req queryRequest
	if err := ParseFormRequest(c, &req); err != nil {
		t.Fatal(err)
	}
	if req.Value != "ok" {
		t.Fatalf("value = %q", req.Value)
	}

	c, _ = gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{"))
	c.Request.Header.Set("Content-Type", "application/json")
	if err := ParseFormRequest(c, &req); err == nil {
		t.Fatal("expected form parse error")
	}
}

func TestValidateRequestFailure(t *testing.T) {
	if err := ValidateRequest(&queryRequest{}); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestResponseOkWithData(t *testing.T) {
	var rsp Response
	data := map[string]string{"value": "ok"}

	rsp.OkWithData(data)

	if rsp.Code != 0 || rsp.Msg != "success" {
		t.Fatalf("unexpected response status: %+v", rsp)
	}
	encoded, err := json.Marshal(rsp.Data)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"value":"ok"}` {
		t.Fatalf("data = %s", encoded)
	}
}

func TestResponseJSONHelpers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	var rsp Response
	rsp.OkRsp(c)
	assertJSONResponse(t, w, 0, "success")

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	rsp.OkRspWithData(c, map[string]string{"value": "ok"})
	assertJSONResponse(t, w, 0, "success")

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	rsp.ErrRsp(c, -1, "failed")
	assertJSONResponse(t, w, -1, "failed")
}

func assertJSONResponse(t *testing.T, w *httptest.ResponseRecorder, code int, msg string) {
	t.Helper()

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	var rsp Response
	if err := json.Unmarshal(w.Body.Bytes(), &rsp); err != nil {
		t.Fatal(err)
	}
	if rsp.Code != code || rsp.Msg != msg {
		t.Fatalf("response = %+v", rsp)
	}
}

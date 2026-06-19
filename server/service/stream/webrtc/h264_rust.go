//go:build !legacy_webrtc

package webrtc

import (
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

func Connect(c *gin.Context) {
	if proxyToRustSidecar(c) {
		return
	}

	c.Status(http.StatusBadGateway)
}

func proxyToRustSidecar(c *gin.Context) bool {
	target, err := url.Parse("http://127.0.0.1:6040")
	if err != nil {
		log.Errorf("invalid Rust WebRTC sidecar URL: %s", err)
		return false
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if errors.Is(err, io.EOF) {
			return
		}
		log.Errorf("Rust WebRTC sidecar proxy failed: %s", err)
		w.WriteHeader(http.StatusBadGateway)
	}
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = c.Request.URL.Path
		req.URL.RawQuery = c.Request.URL.RawQuery
		req.Host = target.Host
	}

	proxy.ServeHTTP(c.Writer, c.Request)
	return true
}

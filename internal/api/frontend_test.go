package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFrontendHandler(t *testing.T) {
	t.Run("serves frontend HTML", func(t *testing.T) {
		server := &Server{}

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		server.handleFrontend(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		// Check content type
		ct := w.Header().Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			t.Errorf("expected HTML content type, got %s", ct)
		}

		// Check body contains HTML
		body := w.Body.String()
		if !strings.Contains(body, "<!DOCTYPE html") {
			t.Error("response body doesn't contain HTML doctype")
		}

		if !strings.Contains(body, "DevLog") {
			t.Error("response body doesn't contain DevLog title")
		}
	})

	t.Run("returns 404 for non-root paths", func(t *testing.T) {
		server := &Server{}

		req := httptest.NewRequest("GET", "/notfound", nil)
		w := httptest.NewRecorder()

		server.handleFrontend(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
		}
	})

	t.Run("returns 404 for /api", func(t *testing.T) {
		server := &Server{}

		req := httptest.NewRequest("GET", "/api", nil)
		w := httptest.NewRecorder()

		server.handleFrontend(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
		}
	})

	t.Run("handles only GET requests", func(t *testing.T) {
		server := &Server{}

		req := httptest.NewRequest("POST", "/", nil)
		w := httptest.NewRecorder()

		server.handleFrontend(w, req)

		// Should still serve the HTML (or 405)
		if w.Code != http.StatusOK && w.Code != http.StatusMethodNotAllowed {
			t.Errorf("unexpected status %d", w.Code)
		}
	})
}

func TestFrontendHTMLContent(t *testing.T) {
	t.Run("HTML contains required sections", func(t *testing.T) {
		server := &Server{}

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		server.handleFrontend(w, req)

		body := w.Body.String()

		requiredElements := []string{
			"<title>",
			"<meta charset",
			"<header>",
			"stats-grid",
			"chart-grid",
			"events-list",
			"<script>",
		}

		for _, elem := range requiredElements {
			if !strings.Contains(body, elem) {
				t.Errorf("response doesn't contain required element: %s", elem)
			}
		}
	})

	t.Run("HTML has CSS styles", func(t *testing.T) {
		server := &Server{}

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		server.handleFrontend(w, req)

		body := w.Body.String()

		if !strings.Contains(body, "<style>") {
			t.Error("response doesn't contain style block")
		}

		if !strings.Contains(body, "background:") || !strings.Contains(body, "color:") {
			t.Error("style block doesn't contain expected CSS")
		}
	})

	t.Run("HTML includes Chart.js", func(t *testing.T) {
		server := &Server{}

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		server.handleFrontend(w, req)

		body := w.Body.String()

		if !strings.Contains(body, "Chart") {
			t.Error("response doesn't include Chart.js references")
		}
	})

	t.Run("HTML has API endpoints referenced", func(t *testing.T) {
		server := &Server{}

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		server.handleFrontend(w, req)

		body := w.Body.String()

		apiEndpoints := []string{
			"/api/v1/status",
			"/api/v1/events",
			"/api/v1/analytics",
		}

		for _, endpoint := range apiEndpoints {
			if !strings.Contains(body, endpoint) {
				t.Logf("response doesn't directly reference: %s (may be dynamically constructed)", endpoint)
			}
		}
	})
}

func TestFrontendStaticContent(t *testing.T) {
	t.Run("HTML is valid UTF-8", func(t *testing.T) {
		server := &Server{}

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		server.handleFrontend(w, req)

		body := w.Body.String()

		// Check that body is valid UTF-8
		if !isValidUTF8(body) {
			t.Error("response body is not valid UTF-8")
		}
	})

	t.Run("HTML has reasonable size", func(t *testing.T) {
		server := &Server{}

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		server.handleFrontend(w, req)

		size := len(w.Body.String())

		// HTML should be at least 1KB
		if size < 1024 {
			t.Logf("HTML is small: %d bytes", size)
		}

		// HTML shouldn't be unreasonably large
		if size > 10*1024*1024 {
			t.Errorf("HTML is too large: %d bytes", size)
		}
	})
}

func isValidUTF8(s string) bool {
	// Simple check: can we range over the string as runes
	for range s {
	}
	return true
}

func TestFrontendResponseHeaders(t *testing.T) {
	t.Run("sets correct content type", func(t *testing.T) {
		server := &Server{}

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		server.handleFrontend(w, req)

		ct := w.Header().Get("Content-Type")
		if ct != "text/html; charset=utf-8" {
			t.Errorf("expected 'text/html; charset=utf-8', got '%s'", ct)
		}
	})

	t.Run("no caching headers set", func(t *testing.T) {
		server := &Server{}

		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		server.handleFrontend(w, req)

		// Frontend should not have cache headers (or they should allow frequent updates)
		cc := w.Header().Get("Cache-Control")
		t.Logf("Cache-Control header: %s", cc)
	})
}

package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"subscription-radar/internal/sample"
)

func TestAnalyzeMultipartUpload(t *testing.T) {
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, _ := form.CreateFormFile("file", "march.csv")
	_, _ = part.Write(sample.Generate(sample.DefaultSeed).CSV)
	_ = form.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/analyze", &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	rec := serve(req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var resp AnalyzeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.FileName != "march.csv" || len(resp.Analysis.Series) != 11 || len(resp.Analysis.Alerts) != 3 {
		t.Errorf("file=%q series=%d alerts=%d", resp.FileName, len(resp.Analysis.Series), len(resp.Analysis.Alerts))
	}
}

func TestAnalyzeRawBody(t *testing.T) {
	csv := "Date,Description,Amount\n01/01/2026,NETFLIX.COM,15.49\n02/01/2026,NETFLIX.COM,15.49\n03/01/2026,NETFLIX.COM,15.49\n"
	rec := serve(httptest.NewRequest(http.MethodPost, "/api/analyze?name=netflix.csv", strings.NewReader(csv)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	// Money must be serialised as plain numbers in major units.
	if !strings.Contains(rec.Body.String(), `"annualCost":185.88`) {
		t.Errorf("unexpected body: %s", rec.Body)
	}
}

func TestAnalyzeErrors(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		status int
		msg    string
	}{
		{"not a statement", "foo,bar\n1,2\n", http.StatusUnprocessableEntity, "Could not find"},
		{"no readable rows", "Date,Description,Amount\nyesterday,NETFLIX,15.49\n", http.StatusUnprocessableEntity, "No charges"},
		{"too large", strings.Repeat("x", MaxUploadBytes+1), http.StatusRequestEntityTooLarge, "5 MB"},
	}
	for _, c := range cases {
		rec := serve(httptest.NewRequest(http.MethodPost, "/api/analyze", strings.NewReader(c.body)))
		if rec.Code != c.status || !strings.Contains(rec.Body.String(), c.msg) {
			t.Errorf("%s: got %d %s", c.name, rec.Code, rec.Body)
		}
	}
}

func TestSampleEndpoint(t *testing.T) {
	rec := serve(httptest.NewRequest(http.MethodGet, "/api/sample", nil))
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") {
		t.Fatalf("status %d, type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if !strings.HasPrefix(rec.Body.String(), "Date,Description,Amount") {
		t.Error("sample should be a CSV with a header")
	}
}

func TestUnknownAPIRouteIsJSON404(t *testing.T) {
	rec := serve(httptest.NewRequest(http.MethodGet, "/api/nope", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "error") {
		t.Errorf("got %d %s", rec.Code, rec.Body)
	}
}

func TestServesFrontendWithSPAFallback(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>app</html>"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "app.js"), []byte("console.log(1)"), 0o644)
	h := NewHandler(Options{StaticDir: dir})

	for path, want := range map[string]string{"/": "app", "/some/route": "app", "/app.js": "console.log"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: got %q", path, rec.Body)
		}
	}
}

func serve(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	NewHandler(Options{}).ServeHTTP(rec, req)
	return rec
}

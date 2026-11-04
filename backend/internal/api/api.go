// Package api exposes the analyser over HTTP.
//
//	GET  /api/health   liveness check
//	GET  /api/sample   the demo statement as CSV
//	POST /api/analyze  analyse a statement (multipart "file" field or raw CSV body)
//
// The server is stateless: files are analysed in memory and never stored.
package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"subscription-radar/internal/recurring"
	"subscription-radar/internal/sample"
	"subscription-radar/internal/statement"
)

// MaxUploadBytes caps uploads. A year of card statements is well under 1 MB.
const MaxUploadBytes = 5 << 20

type Options struct {
	// StaticDir, when set, serves the built frontend from the same origin.
	StaticDir string
	Logger    *slog.Logger
}

// ParseSummary tells the user how their file was read.
type ParseSummary struct {
	TransactionCount int                 `json:"transactionCount"`
	IgnoredCredits   int                 `json:"ignoredCredits"`
	Warnings         []statement.Warning `json:"warnings"`
	Assumptions      []string            `json:"assumptions"`
}

type AnalyzeResponse struct {
	FileName string             `json:"fileName"`
	Parse    ParseSummary       `json:"parse"`
	Analysis recurring.Analysis `json:"analysis"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// NewHandler wires the routes.
func NewHandler(opts Options) http.Handler {
	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/sample", handleSample)
	mux.HandleFunc("POST /api/analyze", handleAnalyze(logger))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "Not found.")
	})
	if opts.StaticDir != "" {
		mux.Handle("/", spaHandler(opts.StaticDir))
	}
	return logRequests(logger, mux)
}

func handleSample(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="sample-statement.csv"`)
	_, _ = w.Write(sample.Generate(sample.DefaultSeed).CSV)
}

func handleAnalyze(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, MaxUploadBytes)

		body, fileName, err := readUpload(r)
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				writeError(w, http.StatusRequestEntityTooLarge, "The file is larger than 5 MB.")
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		defer body.Close()

		parsed, err := statement.Parse(body, statement.Options{})
		if err != nil {
			var formatErr *statement.FormatError
			var tooLarge *http.MaxBytesError
			switch {
			case errors.As(err, &tooLarge):
				writeError(w, http.StatusRequestEntityTooLarge, "The file is larger than 5 MB.")
			case errors.As(err, &formatErr):
				writeError(w, http.StatusUnprocessableEntity, formatErr.Error())
			default:
				logger.Error("reading upload", "err", err)
				writeError(w, http.StatusInternalServerError, "Could not read the file.")
			}
			return
		}
		if len(parsed.Transactions) == 0 {
			writeError(w, http.StatusUnprocessableEntity, "No charges were found in this file.")
			return
		}

		writeJSON(w, http.StatusOK, AnalyzeResponse{
			FileName: fileName,
			Parse: ParseSummary{
				TransactionCount: len(parsed.Transactions),
				IgnoredCredits:   parsed.IgnoredCredits,
				Warnings:         parsed.Warnings,
				Assumptions:      parsed.Assumptions,
			},
			Analysis: recurring.Analyze(parsed.Transactions),
		})
	}
}

// readUpload accepts a multipart form with a "file" field (browser upload) or
// a raw CSV body (curl --data-binary @statement.csv).
func readUpload(r *http.Request) (io.ReadCloser, string, error) {
	mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if mediaType != "multipart/form-data" {
		return r.Body, r.URL.Query().Get("name"), nil
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, "", err
		}
		return nil, "", errors.New(`Expected a CSV file in the "file" field.`)
	}
	return file, header.Filename, nil
}

// spaHandler serves the built frontend and falls back to index.html.
func spaHandler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		files.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			logger.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "duration", time.Since(start).Round(time.Microsecond))
		}
	})
}

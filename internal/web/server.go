package web

import (
	"context"
	"database/sql"
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/vbonduro/kitchinv/internal/db"
	"github.com/vbonduro/kitchinv/internal/domain"
	"github.com/vbonduro/kitchinv/internal/photostore"
	"github.com/vbonduro/kitchinv/internal/service"
	"github.com/vbonduro/kitchinv/internal/vision"
)

// kitchenService is the subset of service.AreaService that the web layer uses.
// Depending on this interface rather than the concrete type decouples the web
// layer from service implementation details and enables testing with fakes.
type kitchenService interface {
	CreateArea(ctx context.Context, name string) (*domain.Area, error)
	ListAreas(ctx context.Context) ([]*domain.Area, error)
	ListAreasWithItems(ctx context.Context) ([]*service.AreaSummary, error)
	GetArea(ctx context.Context, areaID int64) (*domain.Area, error)
	GetAreaWithItems(ctx context.Context, areaID int64) (*domain.Area, []*domain.Item, *domain.Photo, error)
	UpdateArea(ctx context.Context, areaID int64, name string) (*domain.Area, error)
	DeleteArea(ctx context.Context, areaID int64) error
	DeletePhoto(ctx context.Context, areaID int64) error
	UploadPhoto(ctx context.Context, areaID int64, imageData []byte, mimeType string) (*domain.Photo, []*domain.Item, error)
	UploadPhotoStream(ctx context.Context, areaID int64, imageData []byte, mimeType string) (*domain.Photo, <-chan vision.StreamEvent, error)
	CreateItem(ctx context.Context, areaID int64, name, quantity, notes string) (*domain.Item, error)
	UpdateItem(ctx context.Context, itemID int64, name, quantity, notes string) (*domain.Item, error)
	DeleteItem(ctx context.Context, itemID int64) error
	SearchItems(ctx context.Context, query string) ([]*domain.Item, error)
}

type Server struct {
	service    kitchenService
	templates  embed.FS
	photoStore photostore.PhotoStore
	mux        *http.ServeMux
	tmplFuncs  template.FuncMap
	logger     *slog.Logger
	testDB     *sql.DB // non-nil only in test mode
	photoPath  string  // non-empty only in test mode
}

func NewServer(svc kitchenService, tmpl embed.FS, ps photostore.PhotoStore, logger *slog.Logger) *Server {
	s := &Server{
		service:    svc,
		templates:  tmpl,
		photoStore: ps,
		mux:        http.NewServeMux(),
		logger:     logger,
		tmplFuncs: template.FuncMap{
			"inc": func(i int) int { return i + 1 },
			"sub": func(a, b int) int { return a - b },
		},
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/areas", http.StatusSeeOther)
	})
	s.mux.HandleFunc("GET /areas", s.handleListAreas)
	s.mux.HandleFunc("POST /areas", s.handleCreateArea)
	s.mux.HandleFunc("GET /areas/{id}", s.handleGetAreaDetail)
	s.mux.HandleFunc("PUT /areas/{id}", s.handleUpdateArea)
	s.mux.HandleFunc("DELETE /areas/{id}", s.handleDeleteArea)
	s.mux.HandleFunc("DELETE /areas/{id}/photo", s.handleDeletePhoto)
	s.mux.HandleFunc("POST /areas/{id}/photos", s.handleUploadPhoto)
	s.mux.HandleFunc("POST /areas/{id}/photos/stream", s.handleStreamPhoto)
	s.mux.HandleFunc("GET /areas/{id}/photo", s.handleGetPhoto)
	s.mux.HandleFunc("GET /areas/{id}/items", s.handleGetAreaItems)
	s.mux.HandleFunc("POST /areas/{id}/items", s.handleCreateItem)
	s.mux.HandleFunc("PUT /areas/{id}/items/{itemId}", s.handleUpdateItem)
	s.mux.HandleFunc("DELETE /areas/{id}/items/{itemId}", s.handleDeleteItem)
	s.mux.HandleFunc("GET /search", s.handleSearch)
}

// EnableTestMode registers the /control/reset endpoint backed by database and
// the local photo directory. Must be called before the server starts listening.
func (s *Server) EnableTestMode(database *sql.DB, photoPath string) {
	s.testDB = database
	s.photoPath = photoPath
	s.mux.HandleFunc("POST /control/reset", s.handleTestReset)
}

// handleTestReset truncates all application data and clears photo files.
// Only reachable when test mode is enabled.
func (s *Server) handleTestReset(w http.ResponseWriter, r *http.Request) {
	if err := db.Reset(s.testDB); err != nil {
		http.Error(w, "reset failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if s.photoPath != "" {
		entries, _ := os.ReadDir(s.photoPath)
		for _, e := range entries {
			_ = os.Remove(s.photoPath + "/" + e.Name())
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// securityHeaders adds defensive HTTP response headers to every response.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' https://unpkg.com; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data:; "+
				"connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

// statusRecorder wraps http.ResponseWriter to capture the written status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func requestLogger(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestLogger(s.logger, securityHeaders(s.mux)).ServeHTTP(w, r)
}

func (s *Server) ListenAndServe(addr string) error {
	s.logger.Info("starting server", "addr", addr)
	srv := &http.Server{
		Addr:         addr,
		Handler:      s,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	return srv.ListenAndServe()
}

// renderPage parses and executes a full-page template set.
func (s *Server) renderPage(w http.ResponseWriter, data any, files ...string) error {
	tmpl, err := template.New("").Funcs(s.tmplFuncs).ParseFS(s.templates, files...)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.ExecuteTemplate(w, "base", data)
}

// renderPartial parses and executes a single named partial template.
// The file must contain exactly one {{define "name"}}...{{end}} block.
func (s *Server) renderPartial(w http.ResponseWriter, file string, data any) error {
	tmpl, err := template.New("").Funcs(s.tmplFuncs).ParseFS(s.templates, file)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// ParseFS registers both the file-basename template and any {{define}} blocks.
	// Find the {{define}} template: it is the one whose name is neither "" nor
	// the file basename.
	basename := file
	if idx := strings.LastIndexByte(file, '/'); idx >= 0 {
		basename = file[idx+1:]
	}
	for _, t := range tmpl.Templates() {
		if n := t.Name(); n != "" && n != basename {
			return t.Execute(w, data)
		}
	}
	// Fallback: execute the file-basename template (no {{define}} blocks found).
	return tmpl.ExecuteTemplate(w, basename, data)
}


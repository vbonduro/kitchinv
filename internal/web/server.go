package web

import (
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/vbonduro/kitchinv/internal/photostore"
	"github.com/vbonduro/kitchinv/internal/service"
)

type Server struct {
	service    *service.AreaService
	templates  embed.FS
	photoStore photostore.PhotoStore
	mux        *http.ServeMux
	tmplFuncs  template.FuncMap
	logger     *slog.Logger
}

func NewServer(svc *service.AreaService, tmpl embed.FS, ps photostore.PhotoStore, logger *slog.Logger) *Server {
	s := &Server{
		service:    svc,
		templates:  tmpl,
		photoStore: ps,
		mux:        http.NewServeMux(),
		logger:     logger,
		tmplFuncs: template.FuncMap{
			"areaIcon": areaIcon,
			"inc":      func(i int) int { return i + 1 },
			"sub":      func(a, b int) int { return a - b },
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
	s.mux.HandleFunc("DELETE /areas/{id}", s.handleDeleteArea)
	s.mux.HandleFunc("POST /areas/{id}/photos", s.handleUploadPhoto)
	s.mux.HandleFunc("POST /areas/{id}/photos/stream", s.handleStreamPhoto)
	s.mux.HandleFunc("GET /areas/{id}/photo", s.handleGetPhoto)
	s.mux.HandleFunc("GET /search", s.handleSearch)
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
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
				"font-src https://fonts.gstatic.com; "+
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

// areaIcon returns an emoji based on keywords in the area name.
func areaIcon(name string) string {
	lower := strings.ToLower(name)
	switch {
	case contains(lower, "freezer"):
		return "🧊"
	case contains(lower, "fridge", "refrigerator"):
		return "🥶"
	case contains(lower, "pantry"):
		return "🥫"
	case contains(lower, "garage"):
		return "🏠"
	case contains(lower, "basement", "cellar"):
		return "📦"
	case contains(lower, "bar", "wine", "drink", "bever"):
		return "🍷"
	case contains(lower, "spice", "herb"):
		return "🌿"
	default:
		return "🗄️"
	}
}

func contains(s string, keywords ...string) bool {
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

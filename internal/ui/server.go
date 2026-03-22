package ui

import (
	"io/fs"
	"net/http"
	"strings"
	"sync"

	"github.com/lanesket/llm.log/internal/storage"
)

// Server serves the web UI and API endpoints.
type Server struct {
	store   storage.Store
	dataDir string
	mux     *http.ServeMux
	devMode bool

	// Dashboard cache (single entry — only the last response is cached)
	cacheMu    sync.RWMutex
	cacheMaxID int64
	cachedKey  string
	cachedBody []byte
}

// New creates a new UI server. If webFS is nil, only API routes are registered.
func New(store storage.Store, dataDir string, webFS fs.FS, devMode bool) *Server {
	s := &Server{
		store:   store,
		dataDir: dataDir,
		mux:     http.NewServeMux(),
		devMode: devMode,
	}
	s.routes(webFS)
	return s
}

func (s *Server) routes(webFS fs.FS) {
	s.mux.HandleFunc("GET /api/status", s.handleStatus)
	s.mux.HandleFunc("GET /api/dashboard", s.handleDashboard)
	s.mux.HandleFunc("GET /api/requests/{id}", s.handleRequestDetail)
	s.mux.HandleFunc("GET /api/requests", s.handleRequests)
	s.mux.HandleFunc("GET /api/analytics", s.handleAnalytics)
	s.mux.HandleFunc("GET /api/filters", s.handleFilters)
	s.mux.HandleFunc("POST /api/proxy/start", s.handleProxyStart)
	s.mux.HandleFunc("POST /api/proxy/stop", s.handleProxyStop)

	if webFS != nil {
		s.mux.Handle("/", spaHandler(webFS))
	}
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers for dev mode (Vite dev server on different port)
	if s.devMode {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	s.mux.ServeHTTP(w, r)
}

// spaHandler serves static files from webFS with SPA fallback to index.html.
func spaHandler(webFS fs.FS) http.Handler {
	fileServer := http.FileServerFS(webFS)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path != "/" {
			if _, err := fs.Stat(webFS, strings.TrimPrefix(path, "/")); err != nil {
				r.URL.Path = "/"
			}
		}
		fileServer.ServeHTTP(w, r)
	})
}

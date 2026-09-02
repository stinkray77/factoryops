package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/raychua/factoryops/backend/internal/extraction"
	"github.com/raychua/factoryops/backend/internal/model"
	"github.com/raychua/factoryops/backend/internal/store"
)

type Server struct {
	store     store.Store
	extractor *extraction.Service
	logger    *slog.Logger
	requests  *prometheus.CounterVec
	duration  *prometheus.HistogramVec
	extracts  *prometheus.CounterVec
	registry  *prometheus.Registry
}

func New(data store.Store, extractor *extraction.Service, logger *slog.Logger) *Server {
	if logger == nil { logger = slog.Default() }
	registry := prometheus.NewRegistry()
	s := &Server{
		store: data,
		extractor: extractor,
		logger: logger,
		registry: registry,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "factoryops_http_requests_total", Help: "HTTP requests by method, path, and status."}, []string{"method", "path", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "factoryops_http_request_duration_seconds", Help: "HTTP request latency.", Buckets: prometheus.DefBuckets}, []string{"method", "path"}),
		extracts: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "factoryops_document_extractions_total", Help: "Document extraction attempts by result."}, []string{"result"}),
	}
	registry.MustRegister(s.requests, s.duration, s.extracts)
	return s
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, map[string]string{"status": "ok"}) })
	mux.Handle("GET /metrics", promhttp.HandlerFor(s.registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("GET /api/work-orders", s.listWorkOrders)
	mux.HandleFunc("POST /api/work-orders", s.createWorkOrder)
	mux.HandleFunc("PATCH /api/work-orders/{id}/status", s.updateStatus)
	mux.HandleFunc("GET /api/inventory", s.listInventory)
	mux.HandleFunc("POST /api/documents/extract", s.extractDocument)
	return s.cors(s.observe(mux))
}

func (s *Server) listWorkOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := s.store.ListWorkOrders(r.Context())
	if err != nil { s.serverError(w, r, err); return }
	writeJSON(w, http.StatusOK, orders)
}

func (s *Server) createWorkOrder(w http.ResponseWriter, r *http.Request) {
	var order model.WorkOrder
	if err := decodeJSON(r, &order); err != nil { writeError(w, http.StatusBadRequest, err.Error()); return }
	if strings.TrimSpace(order.Product) == "" || strings.TrimSpace(order.SKU) == "" || order.Quantity < 1 {
		writeError(w, http.StatusUnprocessableEntity, "product, sku, and a positive quantity are required")
		return
	}
	order.Status, order.Progress = "PLANNED", 0
	created, err := s.store.CreateWorkOrder(r.Context(), order)
	if err != nil { s.serverError(w, r, err); return }
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) updateStatus(w http.ResponseWriter, r *http.Request) {
	var input struct { Status string `json:"status"`; Progress int `json:"progress"` }
	if err := decodeJSON(r, &input); err != nil { writeError(w, http.StatusBadRequest, err.Error()); return }
	allowed := map[string]bool{"PLANNED": true, "IN_PROGRESS": true, "BLOCKED": true, "COMPLETE": true}
	if !allowed[input.Status] || input.Progress < 0 || input.Progress > 100 {
		writeError(w, http.StatusUnprocessableEntity, "invalid status or progress")
		return
	}
	order, err := s.store.UpdateWorkOrderStatus(r.Context(), r.PathValue("id"), input.Status, input.Progress)
	if errors.Is(err, store.ErrNotFound) { writeError(w, http.StatusNotFound, err.Error()); return }
	if err != nil { s.serverError(w, r, err); return }
	writeJSON(w, http.StatusOK, order)
}

func (s *Server) listInventory(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListInventory(r.Context())
	if err != nil { s.serverError(w, r, err); return }
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) extractDocument(w http.ResponseWriter, r *http.Request) {
	var input struct { Text string `json:"text"` }
	if err := decodeJSON(r, &input); err != nil { s.extracts.WithLabelValues("invalid").Inc(); writeError(w, http.StatusBadRequest, err.Error()); return }
	result, err := s.extractor.Extract(r.Context(), input.Text)
	if err != nil { s.extracts.WithLabelValues("failed").Inc(); writeError(w, http.StatusUnprocessableEntity, err.Error()); return }
	s.extracts.WithLabelValues("succeeded").Inc()
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) observe(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		path := r.Pattern
		if path == "" { path = "unmatched" }
		s.requests.WithLabelValues(r.Method, path, strconv.Itoa(recorder.status)).Inc()
		s.duration.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
	})
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,OPTIONS")
		if r.Method == http.MethodOptions { w.WriteHeader(http.StatusNoContent); return }
		next.ServeHTTP(w, r)
	})
}

func (s *Server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}

type statusRecorder struct { http.ResponseWriter; status int }
func (r *statusRecorder) WriteHeader(status int) { r.status = status; r.ResponseWriter.WriteHeader(status) }

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

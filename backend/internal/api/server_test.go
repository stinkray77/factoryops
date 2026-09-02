package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/raychua/factoryops/backend/internal/api"
	"github.com/raychua/factoryops/backend/internal/extraction"
	"github.com/raychua/factoryops/backend/internal/store"
)

func testServer() http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return api.New(store.NewMemory(), extraction.New("", "", "", nil), logger).Handler()
}

func TestCreateAndListWorkOrder(t *testing.T) {
	handler := testServer()
	body := []byte(`{"product":"Control Rack","sku":"RACK-01","quantity":4,"due":"2026-09-20","station":"Assembly 01"}`)
	create := httptest.NewRequest(http.MethodPost, "/api/work-orders", bytes.NewReader(body))
	create.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, create)
	if response.Code != http.StatusCreated { t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String()) }

	list := httptest.NewRequest(http.MethodGet, "/api/work-orders", nil)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, list)
	if listResponse.Code != http.StatusOK { t.Fatalf("expected 200, got %d", listResponse.Code) }
	var orders []map[string]any
	if err := json.Unmarshal(listResponse.Body.Bytes(), &orders); err != nil { t.Fatal(err) }
	if len(orders) != 4 { t.Fatalf("expected 4 orders, got %d", len(orders)) }
}

func TestExtractionRequiresReview(t *testing.T) {
	handler := testServer()
	body := []byte(`{"text":"PO-2026-0193\nSupplier: Mouser\nPart: MOSFET-650V\nQuantity: 250\nUnit Price: $8.42\nDelivery Date: 12 Sep 2026"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/documents/extract", bytes.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK { t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String()) }
	var extraction map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &extraction); err != nil { t.Fatal(err) }
	if extraction["needsReview"] != true { t.Fatalf("expected human review requirement") }
	if extraction["poNumber"] != "PO-2026-0193" { t.Fatalf("unexpected PO number: %v", extraction["poNumber"]) }
}

func TestMetricsAreExposed(t *testing.T) {
	handler := testServer()
	health := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	handler.ServeHTTP(httptest.NewRecorder(), health)
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK { t.Fatalf("expected 200, got %d", response.Code) }
	if !bytes.Contains(response.Body.Bytes(), []byte("factoryops_http_requests_total")) { t.Fatal("expected FactoryOps metrics") }
}

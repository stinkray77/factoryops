package extraction

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/raychua/factoryops/backend/internal/model"
)

type Service struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

func New(apiKey, baseURL, llmModel string, client *http.Client) *Service {
	if baseURL == "" { baseURL = "https://api.openai.com/v1" }
	if llmModel == "" { llmModel = "gpt-4.1-mini" }
	if client == nil { client = http.DefaultClient }
	return &Service{apiKey: apiKey, baseURL: strings.TrimRight(baseURL, "/"), model: llmModel, client: client}
}

func (s *Service) Extract(ctx context.Context, text string) (model.PurchaseOrderExtraction, error) {
	if strings.TrimSpace(text) == "" {
		return model.PurchaseOrderExtraction{}, errors.New("document text is required")
	}
	if s.apiKey == "" {
		result := heuristic(text)
		result.Provider = "heuristic-fallback"
		result.NeedsReview = true
		return result, nil
	}
	return s.extractWithLLM(ctx, text)
}

func (s *Service) extractWithLLM(ctx context.Context, document string) (model.PurchaseOrderExtraction, error) {
	schema := map[string]any{
		"type": "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"supplier": map[string]string{"type": "string"},
			"poNumber": map[string]string{"type": "string"},
			"part": map[string]string{"type": "string"},
			"quantity": map[string]string{"type": "string"},
			"unitPrice": map[string]string{"type": "string"},
			"deliveryDate": map[string]string{"type": "string"},
		},
		"required": []string{"supplier", "poNumber", "part", "quantity", "unitPrice", "deliveryDate"},
	}
	payload := map[string]any{
		"model": s.model,
		"messages": []map[string]string{
			{"role": "system", "content": "Extract purchase-order fields. Use an empty string when a value is absent. Never infer a value not present in the document."},
			{"role": "user", "content": document},
		},
		"response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "purchase_order", "strict": true, "schema": schema}},
	}
	body, _ := json.Marshal(payload)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil { return model.PurchaseOrderExtraction{}, err }
	request.Header.Set("Authorization", "Bearer "+s.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := s.client.Do(request)
	if err != nil { return model.PurchaseOrderExtraction{}, fmt.Errorf("call LLM provider: %w", err) }
	defer response.Body.Close()
	if response.StatusCode >= 300 { return model.PurchaseOrderExtraction{}, fmt.Errorf("LLM provider returned %s", response.Status) }

	var envelope struct { Choices []struct { Message struct { Content string `json:"content"` } `json:"message"` } `json:"choices"` }
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil { return model.PurchaseOrderExtraction{}, fmt.Errorf("decode LLM response: %w", err) }
	if len(envelope.Choices) == 0 { return model.PurchaseOrderExtraction{}, errors.New("LLM provider returned no choices") }
	var result model.PurchaseOrderExtraction
	if err := json.Unmarshal([]byte(envelope.Choices[0].Message.Content), &result); err != nil { return result, fmt.Errorf("decode structured extraction: %w", err) }
	result.Provider = s.model
	result.NeedsReview = true
	result.Confidence = map[string]float64{}
	return result, nil
}

func heuristic(text string) model.PurchaseOrderExtraction {
	return model.PurchaseOrderExtraction{
		Supplier: find(text, `(?im)^supplier\s*[:\-]\s*(.+)$`),
		PONumber: find(text, `(?i)(PO[-\s]?\d{4}[-\s]?\d+)`),
		Part: find(text, `(?im)^(?:part|sku)\s*[:\-]\s*(.+)$`),
		Quantity: find(text, `(?im)^(?:quantity|qty)\s*[:\-]\s*(\d+)`),
		UnitPrice: find(text, `(?im)^unit price\s*[:\-]\s*\$?([\d,.]+)`),
		DeliveryDate: find(text, `(?im)^(?:delivery date|deliver by)\s*[:\-]\s*(.+)$`),
		Confidence: map[string]float64{"supplier": .95, "poNumber": .98, "part": .95, "quantity": .95, "unitPrice": .82, "deliveryDate": .9},
	}
}

func find(text, pattern string) string {
	match := regexp.MustCompile(pattern).FindStringSubmatch(text)
	if len(match) > 1 { return strings.TrimSpace(match[1]) }
	return ""
}

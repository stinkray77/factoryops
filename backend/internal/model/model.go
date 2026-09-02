package model

import "time"

type WorkOrder struct {
	ID        string    `json:"id"`
	Product   string    `json:"product"`
	SKU       string    `json:"sku"`
	Quantity  int       `json:"quantity"`
	Status    string    `json:"status"`
	Due       string    `json:"due"`
	Progress  int       `json:"progress"`
	Station   string    `json:"station"`
	CreatedAt time.Time `json:"createdAt"`
}

type InventoryItem struct {
	SKU      string `json:"sku"`
	Name     string `json:"name"`
	OnHand   int    `json:"onHand"`
	Reserved int    `json:"reserved"`
	Supplier string `json:"supplier"`
}

type PurchaseOrderExtraction struct {
	Supplier     string             `json:"supplier"`
	PONumber     string             `json:"poNumber"`
	Part         string             `json:"part"`
	Quantity     string             `json:"quantity"`
	UnitPrice    string             `json:"unitPrice"`
	DeliveryDate string             `json:"deliveryDate"`
	Confidence   map[string]float64 `json:"confidence"`
	Provider     string             `json:"provider"`
	NeedsReview  bool               `json:"needsReview"`
}

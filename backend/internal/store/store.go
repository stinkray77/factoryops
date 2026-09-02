package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/raychua/factoryops/backend/internal/model"
)

var ErrNotFound = errors.New("work order not found")

type Store interface {
	ListWorkOrders(context.Context) ([]model.WorkOrder, error)
	CreateWorkOrder(context.Context, model.WorkOrder) (model.WorkOrder, error)
	UpdateWorkOrderStatus(context.Context, string, string, int) (model.WorkOrder, error)
	ListInventory(context.Context) ([]model.InventoryItem, error)
}

type Memory struct {
	mu        sync.RWMutex
	orders    []model.WorkOrder
	inventory []model.InventoryItem
}

func NewMemory() *Memory {
	now := time.Now().UTC()
	return &Memory{
		orders: []model.WorkOrder{
			{ID: "WO-1042", Product: "SST Control Module", SKU: "CTRL-SST-04", Quantity: 20, Status: "IN_PROGRESS", Due: "2026-09-08", Progress: 64, Station: "Assembly 02", CreatedAt: now},
			{ID: "WO-1043", Product: "650V Power Stage", SKU: "PWR-650-12", Quantity: 48, Status: "BLOCKED", Due: "2026-09-10", Progress: 35, Station: "Power Lab", CreatedAt: now},
			{ID: "WO-1044", Product: "Gate Driver Board", SKU: "GDB-09-A", Quantity: 80, Status: "PLANNED", Due: "2026-09-14", Progress: 0, Station: "SMT Line 01", CreatedAt: now},
		},
		inventory: []model.InventoryItem{
			{SKU: "MOSFET-650V", Name: "SiC MOSFET 650V", OnHand: 14, Reserved: 48, Supplier: "Mouser"},
			{SKU: "CTRL-DSP-02", Name: "Control DSP", OnHand: 92, Reserved: 20, Supplier: "DigiKey"},
			{SKU: "CAP-FILM-18", Name: "Film Capacitor 18μF", OnHand: 186, Reserved: 96, Supplier: "TDK"},
		},
	}
}

func (m *Memory) ListWorkOrders(context.Context) ([]model.WorkOrder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]model.WorkOrder(nil), m.orders...), nil
}

func (m *Memory) CreateWorkOrder(_ context.Context, order model.WorkOrder) (model.WorkOrder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	order.ID = fmt.Sprintf("WO-%04d", 1042+len(m.orders)+1)
	order.CreatedAt = time.Now().UTC()
	m.orders = append([]model.WorkOrder{order}, m.orders...)
	return order, nil
}

func (m *Memory) UpdateWorkOrderStatus(_ context.Context, id, status string, progress int) (model.WorkOrder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.orders {
		if m.orders[i].ID == id {
			m.orders[i].Status = status
			m.orders[i].Progress = progress
			return m.orders[i], nil
		}
	}
	return model.WorkOrder{}, ErrNotFound
}

func (m *Memory) ListInventory(context.Context) ([]model.InventoryItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]model.InventoryItem(nil), m.inventory...), nil
}

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(ctx context.Context, databaseURL string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("configure database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Close() { p.pool.Close() }

func (p *Postgres) ListWorkOrders(ctx context.Context) ([]model.WorkOrder, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, product, sku, quantity, status, due_date::text, progress, station, created_at FROM work_orders ORDER BY created_at DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	var orders []model.WorkOrder
	for rows.Next() {
		var order model.WorkOrder
		if err := rows.Scan(&order.ID, &order.Product, &order.SKU, &order.Quantity, &order.Status, &order.Due, &order.Progress, &order.Station, &order.CreatedAt); err != nil { return nil, err }
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (p *Postgres) CreateWorkOrder(ctx context.Context, order model.WorkOrder) (model.WorkOrder, error) {
	err := p.pool.QueryRow(ctx, `
		INSERT INTO work_orders (product, sku, quantity, status, due_date, progress, station)
		VALUES ($1, $2, $3, $4, $5::date, $6, $7)
		RETURNING id, created_at
	`, order.Product, order.SKU, order.Quantity, order.Status, order.Due, order.Progress, order.Station).Scan(&order.ID, &order.CreatedAt)
	return order, err
}

func (p *Postgres) UpdateWorkOrderStatus(ctx context.Context, id, status string, progress int) (model.WorkOrder, error) {
	var order model.WorkOrder
	err := p.pool.QueryRow(ctx, `
		UPDATE work_orders SET status = $2, progress = $3, updated_at = now()
		WHERE id = $1
		RETURNING id, product, sku, quantity, status, due_date::text, progress, station, created_at
	`, id, status, progress).Scan(&order.ID, &order.Product, &order.SKU, &order.Quantity, &order.Status, &order.Due, &order.Progress, &order.Station, &order.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) { return order, ErrNotFound }
	return order, err
}

func (p *Postgres) ListInventory(ctx context.Context) ([]model.InventoryItem, error) {
	rows, err := p.pool.Query(ctx, `SELECT sku, name, on_hand, reserved, supplier FROM inventory ORDER BY sku`)
	if err != nil { return nil, err }
	defer rows.Close()
	var items []model.InventoryItem
	for rows.Next() {
		var item model.InventoryItem
		if err := rows.Scan(&item.SKU, &item.Name, &item.OnHand, &item.Reserved, &item.Supplier); err != nil { return nil, err }
		items = append(items, item)
	}
	return items, rows.Err()
}

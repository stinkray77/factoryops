CREATE SEQUENCE IF NOT EXISTS work_order_number START 1045;

CREATE TABLE IF NOT EXISTS work_orders (
  id text PRIMARY KEY DEFAULT ('WO-' || lpad(nextval('work_order_number')::text, 4, '0')),
  product text NOT NULL,
  sku text NOT NULL,
  quantity integer NOT NULL CHECK (quantity > 0),
  status text NOT NULL CHECK (status IN ('PLANNED', 'IN_PROGRESS', 'BLOCKED', 'COMPLETE')),
  due_date date NOT NULL,
  progress integer NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
  station text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS inventory (
  sku text PRIMARY KEY,
  name text NOT NULL,
  on_hand integer NOT NULL DEFAULT 0 CHECK (on_hand >= 0),
  reserved integer NOT NULL DEFAULT 0 CHECK (reserved >= 0),
  reorder_level integer NOT NULL DEFAULT 0 CHECK (reorder_level >= 0),
  supplier text NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO work_orders (id, product, sku, quantity, status, due_date, progress, station) VALUES
  ('WO-1042', 'SST Control Module', 'CTRL-SST-04', 20, 'IN_PROGRESS', '2026-09-08', 64, 'Assembly 02'),
  ('WO-1043', '650V Power Stage', 'PWR-650-12', 48, 'BLOCKED', '2026-09-10', 35, 'Power Lab'),
  ('WO-1044', 'Gate Driver Board', 'GDB-09-A', 80, 'PLANNED', '2026-09-14', 0, 'SMT Line 01')
ON CONFLICT (id) DO NOTHING;

INSERT INTO inventory (sku, name, on_hand, reserved, reorder_level, supplier) VALUES
  ('MOSFET-650V', 'SiC MOSFET 650V', 14, 48, 60, 'Mouser'),
  ('CTRL-DSP-02', 'Control DSP', 92, 20, 40, 'DigiKey'),
  ('CAP-FILM-18', 'Film Capacitor 18μF', 186, 96, 80, 'TDK'),
  ('PCB-GDB-09', 'Gate Driver PCB', 84, 80, 30, 'JLCPCB')
ON CONFLICT (sku) DO NOTHING;

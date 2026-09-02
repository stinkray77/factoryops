"use client";

import { useEffect, useMemo, useState } from "react";
import {
  Activity,
  AlertTriangle,
  ArrowRight,
  Boxes,
  Check,
  ClipboardList,
  FileScan,
  Gauge,
  Layers3,
  PackageCheck,
  Plus,
  Search,
  Sparkles,
  TimerReset,
  TrendingUp,
  X,
} from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Progress } from "@/components/ui/progress";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Toaster } from "@/components/ui/sonner";

type Status = "PLANNED" | "IN_PROGRESS" | "BLOCKED" | "COMPLETE";

type WorkOrder = {
  id: string;
  product: string;
  sku: string;
  quantity: number;
  status: Status;
  due: string;
  progress: number;
  station: string;
};

type ExtractedPO = {
  supplier: string;
  poNumber: string;
  part: string;
  quantity: string;
  unitPrice: string;
  deliveryDate: string;
};

const initialOrders: WorkOrder[] = [
  { id: "WO-1042", product: "SST Control Module", sku: "CTRL-SST-04", quantity: 20, status: "IN_PROGRESS", due: "Sep 08", progress: 64, station: "Assembly 02" },
  { id: "WO-1043", product: "650V Power Stage", sku: "PWR-650-12", quantity: 48, status: "BLOCKED", due: "Sep 10", progress: 35, station: "Power Lab" },
  { id: "WO-1044", product: "Gate Driver Board", sku: "GDB-09-A", quantity: 80, status: "PLANNED", due: "Sep 14", progress: 0, station: "SMT Line 01" },
  { id: "WO-1040", product: "Thermal Interface Kit", sku: "THM-KIT-22", quantity: 32, status: "COMPLETE", due: "Sep 02", progress: 100, station: "Final QA" },
];

const inventory = [
  { sku: "MOSFET-650V", name: "SiC MOSFET 650V", onHand: 14, reserved: 48, supplier: "Mouser" },
  { sku: "CTRL-DSP-02", name: "Control DSP", onHand: 92, reserved: 20, supplier: "DigiKey" },
  { sku: "CAP-FILM-18", name: "Film Capacitor 18μF", onHand: 186, reserved: 96, supplier: "TDK" },
  { sku: "PCB-GDB-09", name: "Gate Driver PCB", onHand: 84, reserved: 80, supplier: "JLCPCB" },
];

const emptyPO: ExtractedPO = { supplier: "", poNumber: "", part: "", quantity: "", unitPrice: "", deliveryDate: "" };
const apiBase = process.env.NEXT_PUBLIC_API_URL?.replace(/\/$/, "");

const statusStyle: Record<Status, string> = {
  PLANNED: "status planned",
  IN_PROGRESS: "status active",
  BLOCKED: "status blocked",
  COMPLETE: "status complete",
};

const statusLabel: Record<Status, string> = {
  PLANNED: "Planned",
  IN_PROGRESS: "In progress",
  BLOCKED: "Blocked",
  COMPLETE: "Complete",
};

function nextStatus(status: Status): Status {
  if (status === "PLANNED") return "IN_PROGRESS";
  if (status === "IN_PROGRESS" || status === "BLOCKED") return "COMPLETE";
  return "COMPLETE";
}

function extractField(text: string, patterns: RegExp[]) {
  for (const pattern of patterns) {
    const match = text.match(pattern);
    if (match?.[1]) return match[1].trim();
  }
  return "";
}

export default function Home() {
  const [orders, setOrders] = useState(initialOrders);
  const [query, setQuery] = useState("");
  const [dialogOpen, setDialogOpen] = useState(false);
  const [documentText, setDocumentText] = useState(
    "PURCHASE ORDER PO-2026-0193\nSupplier: Mouser Electronics\nPart: MOSFET-650V\nQuantity: 250\nUnit Price: $8.42\nDelivery Date: 12 Sep 2026",
  );
  const [extracted, setExtracted] = useState<ExtractedPO>(emptyPO);
  const [extracting, setExtracting] = useState(false);
  const [reviewed, setReviewed] = useState(false);

  const filteredOrders = useMemo(() => {
    const normalized = query.toLowerCase();
    return orders.filter((order) =>
      `${order.id} ${order.product} ${order.sku} ${order.station}`.toLowerCase().includes(normalized),
    );
  }, [orders, query]);

  const completed = orders.filter((order) => order.status === "COMPLETE").length;
  const blocked = orders.filter((order) => order.status === "BLOCKED").length;

  useEffect(() => {
    if (!apiBase) return;
    fetch(`${apiBase}/api/work-orders`)
      .then((response) => response.ok ? response.json() : Promise.reject(new Error("API unavailable")))
      .then((data: WorkOrder[]) => setOrders(data))
      .catch(() => toast.warning("Using local demo data"));
  }, []);

  async function createOrder(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const product = String(form.get("product") ?? "").trim();
    const quantity = Number(form.get("quantity"));
    if (!product || quantity < 1) return;
    const draft: WorkOrder = {
      id: `WO-${1040 + orders.length + 1}`,
      product,
      sku: String(form.get("sku") ?? "NEW-PART"),
      quantity,
      status: "PLANNED",
      due: String(form.get("due") ?? "Sep 20"),
      progress: 0,
      station: String(form.get("station") ?? "Unassigned"),
    };
    let created = draft;
    if (apiBase) {
      try {
        const response = await fetch(`${apiBase}/api/work-orders`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(draft),
        });
        if (!response.ok) throw new Error("Create failed");
        created = await response.json() as WorkOrder;
      } catch {
        toast.warning("API unavailable — created in demo mode");
      }
    }
    setOrders((current) => [created, ...current]);
    setDialogOpen(false);
    toast.success(`${created.id} created`);
  }

  function advanceOrder(id: string) {
    setOrders((current) =>
      current.map((order) => {
        if (order.id !== id || order.status === "COMPLETE") return order;
        const status = nextStatus(order.status);
        return { ...order, status, progress: status === "COMPLETE" ? 100 : Math.max(order.progress, 12) };
      }),
    );
    toast.success(`${id} status updated`);
  }

  async function runExtraction() {
    if (!documentText.trim()) {
      toast.error("Paste a purchase order first");
      return;
    }
    setExtracting(true);
    setReviewed(false);
    if (apiBase) {
      try {
        const response = await fetch(`${apiBase}/api/documents/extract`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ text: documentText }),
        });
        if (!response.ok) throw new Error("Extraction failed");
        const result = await response.json() as ExtractedPO;
        setExtracted(result);
        setExtracting(false);
        toast.success("Fields extracted — review before saving");
        return;
      } catch {
        toast.warning("AI service unavailable — using demo parser");
      }
    }
    window.setTimeout(() => {
      setExtracted({
        supplier: extractField(documentText, [/supplier\s*[:\-]\s*(.+)/i]),
        poNumber: extractField(documentText, [/(PO[-\s]?\d{4}[-\s]?\d+)/i, /purchase order\s*[:\-]?\s*(\S+)/i]),
        part: extractField(documentText, [/(?:part|sku)\s*[:\-]\s*(.+)/i]),
        quantity: extractField(documentText, [/quantity\s*[:\-]\s*(\d+)/i, /qty\s*[:\-]\s*(\d+)/i]),
        unitPrice: extractField(documentText, [/unit price\s*[:\-]\s*\$?([\d,.]+)/i]),
        deliveryDate: extractField(documentText, [/(?:delivery date|deliver by)\s*[:\-]\s*(.+)/i]),
      });
      setExtracting(false);
      toast.success("Fields extracted — review before saving");
    }, 650);
  }

  function updateExtracted(key: keyof ExtractedPO, value: string) {
    setExtracted((current) => ({ ...current, [key]: value }));
  }

  function confirmExtraction() {
    setReviewed(true);
    toast.success("Purchase order verified and queued for import");
  }

  return (
    <main className="app-shell">
      <Toaster position="top-right" />
      <aside className="sidebar">
        <div className="brand-mark" aria-label="FactoryOps">
          <span className="brand-icon"><Layers3 size={18} /></span>
          <span>Factory<span>Ops</span></span>
        </div>
        <nav aria-label="Primary navigation">
          <a className="nav-item selected" href="#operations"><Gauge size={17} /> Operations</a>
          <a className="nav-item" href="#work-orders"><ClipboardList size={17} /> Work orders <b>{orders.length}</b></a>
          <a className="nav-item" href="#inventory"><Boxes size={17} /> Inventory</a>
          <a className="nav-item" href="#documents"><FileScan size={17} /> Documents</a>
          <a className="nav-item" href="#observability"><Activity size={17} /> Observability</a>
        </nav>
        <div className="sidebar-footer">
          <div className="environment-dot"><span /> Production</div>
          <p>Singapore plant · Shift A</p>
        </div>
      </aside>

      <section className="workspace" id="operations">
        <header className="topbar">
          <div>
            <p className="eyebrow">Manufacturing control</p>
            <h1>Operations overview</h1>
          </div>
          <div className="top-actions">
            <div className="search-box">
              <Search size={16} />
              <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search work orders" aria-label="Search work orders" />
              <kbd>⌘ K</kbd>
            </div>
            <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
              <DialogTrigger asChild>
                <Button className="primary-action"><Plus size={16} /> New work order</Button>
              </DialogTrigger>
              <DialogContent className="order-dialog">
                <DialogHeader>
                  <DialogTitle>Create work order</DialogTitle>
                  <DialogDescription>Schedule a production run and assign its first station.</DialogDescription>
                </DialogHeader>
                <form id="new-order-form" onSubmit={createOrder} className="form-grid">
                  <label>Product name<input name="product" placeholder="SST Control Module" required /></label>
                  <label>SKU<input name="sku" placeholder="CTRL-SST-04" required /></label>
                  <label>Quantity<input name="quantity" type="number" min="1" defaultValue="20" required /></label>
                  <label>Due date<input name="due" placeholder="Sep 20" required /></label>
                  <label className="wide">Station<input name="station" placeholder="Assembly 02" required /></label>
                </form>
                <DialogFooter>
                  <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
                  <Button type="submit" form="new-order-form">Create order</Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
          </div>
        </header>

        <div className="content">
          <section className="alert-strip" aria-label="Production alert">
            <div className="alert-icon"><AlertTriangle size={18} /></div>
            <div><strong>WO-1043 is blocked by a component shortage.</strong><span>34 × MOSFET-650V required · incoming PO-2026-0193 expected Sep 12</span></div>
            <button onClick={() => document.getElementById("inventory")?.scrollIntoView({ behavior: "smooth" })}>Review shortage <ArrowRight size={14} /></button>
          </section>

          <section className="metric-grid" aria-label="Key production metrics">
            <article className="metric-card"><div className="metric-heading"><span>Open work orders</span><ClipboardList size={17} /></div><strong>{orders.length - completed}</strong><small><TrendingUp size={13} /> 3 due this week</small></article>
            <article className="metric-card"><div className="metric-heading"><span>Units in production</span><PackageCheck size={17} /></div><strong>148</strong><small><span className="good-dot" /> 92% plan attainment</small></article>
            <article className="metric-card"><div className="metric-heading"><span>Blocked orders</span><AlertTriangle size={17} /></div><strong>{blocked}</strong><small className="warn-copy">34 components short</small></article>
            <article className="metric-card"><div className="metric-heading"><span>On-time completion</span><TimerReset size={17} /></div><strong>96.4%</strong><small><TrendingUp size={13} /> +2.1% vs last week</small></article>
          </section>

          <Tabs defaultValue="orders" className="data-tabs">
            <TabsList variant="line" className="data-tabs-list">
              <TabsTrigger value="orders">Work orders</TabsTrigger>
              <TabsTrigger value="inventory">Inventory</TabsTrigger>
              <TabsTrigger value="documents">AI documents</TabsTrigger>
              <TabsTrigger value="metrics">Service health</TabsTrigger>
            </TabsList>

            <TabsContent value="orders" id="work-orders">
              <section className="panel">
                <div className="panel-heading"><div><h2>Production schedule</h2><p>Active and upcoming manufacturing work</p></div><span className="sync"><span /> Live data</span></div>
                <div className="table-wrap">
                  <table>
                    <thead><tr><th>Work order</th><th>Product</th><th>Quantity</th><th>Status</th><th>Progress</th><th>Due</th><th><span className="sr-only">Action</span></th></tr></thead>
                    <tbody>
                      {filteredOrders.map((order) => (
                        <tr key={order.id}>
                          <td><strong>{order.id}</strong><small>{order.station}</small></td>
                          <td><strong>{order.product}</strong><small>{order.sku}</small></td>
                          <td>{order.quantity}</td>
                          <td><span className={statusStyle[order.status]}>{statusLabel[order.status]}</span></td>
                          <td><div className="progress-cell"><Progress value={order.progress} /><span>{order.progress}%</span></div></td>
                          <td>{order.due}</td>
                          <td><button className="row-action" disabled={order.status === "COMPLETE"} onClick={() => advanceOrder(order.id)}>{order.status === "COMPLETE" ? <Check size={15} /> : <ArrowRight size={15} />}<span className="sr-only">Advance {order.id}</span></button></td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </section>
            </TabsContent>

            <TabsContent value="inventory" id="inventory">
              <section className="panel">
                <div className="panel-heading"><div><h2>Inventory position</h2><p>Current stock against committed demand</p></div><span className="sync"><span /> Updated now</span></div>
                <div className="inventory-grid">
                  {inventory.map((item) => {
                    const available = item.onHand - item.reserved;
                    const shortage = available < 0;
                    return <article className={`inventory-card ${shortage ? "shortage" : ""}`} key={item.sku}>
                      <div className="inventory-top"><span className="part-icon"><Boxes size={17} /></span>{shortage && <span className="attention">Shortage</span>}</div>
                      <h3>{item.name}</h3><p>{item.sku} · {item.supplier}</p>
                      <div className="stock-row"><div><small>On hand</small><strong>{item.onHand}</strong></div><div><small>Reserved</small><strong>{item.reserved}</strong></div><div><small>Available</small><strong className={shortage ? "negative" : ""}>{available}</strong></div></div>
                    </article>;
                  })}
                </div>
              </section>
            </TabsContent>

            <TabsContent value="documents" id="documents">
              <section className="document-layout">
                <div className="panel document-input">
                  <div className="panel-heading"><div><h2>Purchase-order extraction</h2><p>Paste document text to create structured ERP fields</p></div><span className="ai-badge"><Sparkles size={14} /> AI assisted</span></div>
                  <label className="document-label">Document text<textarea value={documentText} onChange={(event) => setDocumentText(event.target.value)} spellCheck={false} /></label>
                  <div className="document-foot"><small>AI output requires human review before import.</small><Button onClick={runExtraction} disabled={extracting}>{extracting ? <><span className="spinner" /> Extracting</> : <><FileScan size={16} /> Extract fields</>}</Button></div>
                </div>
                <div className="panel extraction-review">
                  <div className="panel-heading"><div><h2>Review extracted data</h2><p>Correct uncertain values before saving</p></div>{reviewed ? <span className="verified"><Check size={14} /> Verified</span> : <span className="review-state">Review required</span>}</div>
                  {extracted.poNumber ? <div className="extracted-grid">
                    {([
                      ["supplier", "Supplier"], ["poNumber", "PO number"], ["part", "Part / SKU"], ["quantity", "Quantity"], ["unitPrice", "Unit price (USD)"], ["deliveryDate", "Delivery date"],
                    ] as [keyof ExtractedPO, string][]).map(([key, label], index) => <label key={key}>{label}<span><input value={extracted[key]} onChange={(event) => updateExtracted(key, event.target.value)} /><i className={index === 4 ? "low" : "high"}>{index === 4 ? "82%" : "98%"}</i></span></label>)}
                    <Button className="confirm-button" onClick={confirmExtraction} disabled={reviewed}><Check size={16} /> Confirm and queue import</Button>
                  </div> : <div className="empty-review"><FileScan size={28} /><strong>No extraction yet</strong><p>Extract a document to review its structured fields here.</p></div>}
                </div>
              </section>
            </TabsContent>

            <TabsContent value="metrics" id="observability">
              <section className="panel">
                <div className="panel-heading"><div><h2>Service health</h2><p>Operational signals emitted by the Go API</p></div><span className="sync"><span /> All systems operational</span></div>
                <div className="health-grid">
                  <article><small>API availability</small><strong>99.98%</strong><div className="spark-bars">{[32,38,35,44,42,48,46,51,49,52,50,55].map((height, i) => <i key={i} style={{height}} />)}</div></article>
                  <article><small>p95 API latency</small><strong>42 <em>ms</em></strong><div className="threshold"><span style={{width:"28%"}} /> <small>Target &lt; 150 ms</small></div></article>
                  <article><small>p95 extraction latency</small><strong>1.8 <em>s</em></strong><div className="threshold blue"><span style={{width:"60%"}} /> <small>Target &lt; 3 s</small></div></article>
                  <article><small>Extraction error rate</small><strong>1.3%</strong><div className="threshold orange"><span style={{width:"13%"}} /> <small>Alert at 5%</small></div></article>
                </div>
                <div className="metric-events"><div><span className="event-success"><Check size={14} /></span><p><strong>POST /api/work-orders</strong><small>200 · 31 ms · 2 min ago</small></p></div><div><span className="event-success"><Check size={14} /></span><p><strong>POST /api/documents/extract</strong><small>200 · 1.42 s · 8 min ago</small></p></div><div><span className="event-fail"><X size={14} /></span><p><strong>POST /api/documents/extract</strong><small>422 · missing document text · 29 min ago</small></p></div></div>
              </section>
            </TabsContent>
          </Tabs>
        </div>
      </section>
    </main>
  );
}

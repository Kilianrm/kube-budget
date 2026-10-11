import { ChevronLeft, ChevronRight, Clock, Cloud, Download, Eye, FileCode2, Network, RotateCcw, Search, Trash2, X } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { useEstimationHistory, SavedEstimation } from "./EstimationHistory";
import { formatMoney } from "./costFormat";
import { ClusterImpact } from "./ClusterImpact";
import type { jsPDF as JsPDFDocument } from "jspdf";

interface HistoryPanelProps {
  onRestoreEstimation: (estimation: SavedEstimation) => void;
}

export function HistoryPanel({ onRestoreEstimation }: HistoryPanelProps) {
  const { estimations, deleteEstimation, clearAll } = useEstimationHistory();
  const [isOpen, setIsOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<"all" | "manual" | "cluster">("all");

  useEffect(() => {
    function closeOnEscape(event: KeyboardEvent) {
      if (event.key === "Escape") setIsOpen(false);
    }

    if (isOpen) {
      document.addEventListener("keydown", closeOnEscape);
      return () => document.removeEventListener("keydown", closeOnEscape);
    }
  }, [isOpen]);

  const normalizedQuery = query.trim().toLowerCase();
  const filteredEstimations = estimations.filter((estimation) => {
    const matchesFilter = filter === "all" || estimation.estimationMode === filter;
    const searchableText = [estimation.name, estimation.provider, estimation.region, estimation.clusterName]
      .filter(Boolean)
      .join(" ")
      .toLowerCase();
    return matchesFilter && (!normalizedQuery || searchableText.includes(normalizedQuery));
  });

  function handleClearAll() {
    if (window.confirm("Remove all saved estimations? This cannot be undone.")) clearAll();
  }

  return (
    <>
      <button type="button" className={`history-trigger ${isOpen ? "active" : ""}`} onClick={() => setIsOpen(true)} aria-expanded={isOpen} aria-controls="saved-estimates-drawer">
        <Clock size={16} /><span>Saved estimates</span><span className="history-count">{estimations.length}</span>
      </button>

      {isOpen && <>
        <button type="button" className="history-backdrop" onClick={() => setIsOpen(false)} aria-label="Close saved estimates" />
        <aside id="saved-estimates-drawer" className="history-drawer" aria-label="Saved estimates">
          <div className="history-drawer-header">
            <div className="history-header-title"><Clock size={18} /><span><strong>Saved estimates</strong><small>{estimations.length} recent run{estimations.length === 1 ? "" : "s"}</small></span></div>
            <button type="button" className="history-close-button" onClick={() => setIsOpen(false)} aria-label="Close saved estimates"><X size={17} /></button>
          </div>
          {estimations.length === 0 ? <p className="history-empty history-empty-drawer">Your saved estimates will appear here after the first run.</p> : <div className="history-body">
          <div className="history-toolbar">
            <label className="history-search"><Search size={14} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search saved estimates" aria-label="Search saved estimates" />{query && <button type="button" onClick={() => setQuery("")} aria-label="Clear search"><X size={13} /></button>}</label>
            <button type="button" className="history-clear-button" onClick={handleClearAll}><Trash2 size={13} /> Clear all</button>
          </div>
          <div className="history-filters" role="tablist" aria-label="Filter saved estimates">
            {(["all", "manual", "cluster"] as const).map((value) => <button key={value} type="button" role="tab" aria-selected={filter === value} className={filter === value ? "active" : ""} onClick={() => setFilter(value)}>{value === "all" ? "All" : value === "manual" ? "Manual" : "Cluster"}</button>)}
          </div>
          <div className="history-list">
          {filteredEstimations.map((estimation) => (
            <div key={estimation.id} className="history-item">
              <div className="history-item-content">
                <div className="history-item-main">
                  <strong className="history-item-name" title={estimation.name}>{estimation.name}</strong>
                  <span className="history-cost">{formatCost(estimation.monthlyCost, estimation.currency)}/mo</span>
                </div>
                <div className="history-meta">
                  <span className={`history-mode ${estimation.estimationMode}`}>{estimation.estimationMode === "manual" ? `${estimation.provider?.toUpperCase() ?? "Manual"} · ${estimation.region ?? ""}` : estimation.clusterName || "Cluster"}</span>
                  <span className="history-time">{formatTime(estimation.timestamp)}</span>
                </div>
              </div>
              <div className="history-item-actions"><button type="button" className="history-restore-button" onClick={() => onRestoreEstimation(estimation)}><RotateCcw size={13} /> Restore</button><button type="button" className="history-delete-button" onClick={() => { if (window.confirm(`Delete estimation "${estimation.name}"? This cannot be undone.`)) deleteEstimation(estimation.id); }} title="Delete this estimation" aria-label={`Delete ${estimation.name}`}><Trash2 size={14} /></button></div>
            </div>
          ))}
          {filteredEstimations.length === 0 && <p className="history-empty">No saved estimates match this search.</p>}
          </div>
          </div>}
        </aside>
      </>}
    </>
  );
}

export function EstimationRegistry() {
  const { estimations, deleteEstimation, clearAll } = useEstimationHistory();
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<"all" | "manual" | "cluster">("all");
  const [selectedEstimation, setSelectedEstimation] = useState<SavedEstimation | null>(null);
  const [sort, setSort] = useState("recent");
  const [page, setPage] = useState(1);
  const normalizedQuery = query.trim().toLowerCase();
  const filteredEstimations = estimations.filter((estimation) => {
    const matchesFilter = filter === "all" || estimation.estimationMode === filter;
    const searchableText = [estimation.name, estimation.provider, estimation.region, estimation.clusterName].filter(Boolean).join(" ").toLowerCase();
    return matchesFilter && (!normalizedQuery || searchableText.includes(normalizedQuery));
  }).sort((first, second) => sort === "oldest" ? first.timestamp - second.timestamp : second.timestamp - first.timestamp);
  const pageSize = 10;
  const pageCount = Math.max(1, Math.ceil(filteredEstimations.length / pageSize));
  const currentPage = Math.min(page, pageCount);
  const startIndex = (currentPage - 1) * pageSize;
  const visibleEstimations = filteredEstimations.slice(startIndex, startIndex + pageSize);

  function handleClearAll() {
    if (window.confirm("Remove all saved estimations? This cannot be undone.")) clearAll();
  }

  return (
    <section className="history-page">
      <div className="history-page-content">
      <header className="management-header history-page-header">
        <div><span className="step-label">MANIFEST / HISTORIAL</span><h1>Historial <span className="history-record-count">{estimations.length}</span></h1></div>
        <div className="history-header-actions">
          <button type="button" className="secondary-button history-download-all" onClick={() => downloadFullHistoryPdf(estimations)} disabled={estimations.length === 0} title="Download all estimations as one PDF"><Download size={15} /> Download history</button>
          <button type="button" className="secondary-button history-clear-all" onClick={handleClearAll} disabled={estimations.length === 0}><Trash2 size={15} /> Clear history</button>
        </div>
      </header>
      <div className="history-page-toolbar">
        <label className="history-search"><Search size={16} /><input value={query} onChange={(event) => { setQuery(event.target.value); setPage(1); }} placeholder="Search manifest, provider, region, or cluster" aria-label="Search historial" />{query && <button type="button" onClick={() => { setQuery(""); setPage(1); }} aria-label="Clear search"><X size={14} /></button>}</label>
        <div className="history-filters" role="group" aria-label="Estimation source">
          {(["all", "manual", "cluster"] as const).map((value) => <button key={value} type="button" aria-pressed={filter === value} className={filter === value ? "active" : ""} onClick={() => { setFilter(value); setPage(1); }}>{value === "all" ? "All" : value === "manual" ? "Catalog" : "Cluster"}</button>)}
        </div>
        <select className="history-sort" value={sort} onChange={(event) => { setSort(event.target.value); setPage(1); }} aria-label="Sort history"><option value="recent">Newest first</option><option value="oldest">Oldest first</option></select>
      </div>
      <div className="history-table-wrap">
        <table className="history-table">
          <thead><tr><th scope="col">Manifest</th><th scope="col">Estimation source</th><th scope="col">Saved</th><th scope="col" className="history-amount">Monthly estimate</th><th scope="col" className="history-actions-heading">Actions</th></tr></thead>
          <tbody>{visibleEstimations.map((estimation) => <tr key={estimation.id}>
            <td><div className="history-manifest-cell"><FileCode2 size={18} /><div><button type="button" className="history-manifest-link" onClick={() => setSelectedEstimation(estimation)} title={estimation.name}>{estimation.name}</button><span>{estimation.fullResult.workload.namespace || "default"} / {estimation.fullResult.workload.replicas} replicas</span></div></div></td>
            <td><div className={`history-source-cell ${estimation.estimationMode}`}><span>{estimation.estimationMode === "cluster" ? <Network size={14} /> : <Cloud size={14} />}{estimation.estimationMode === "cluster" ? "Cluster" : "Catalog"}</span><small title={estimation.clusterName || estimation.region}>{estimation.estimationMode === "cluster" ? estimation.clusterName || "Cluster" : `${estimation.provider?.toUpperCase() ?? "Catalog"} / ${estimation.region || "-"}`}</small></div></td>
            <td><div className="history-saved-cell"><time dateTime={new Date(estimation.timestamp).toISOString()}>{formatDateTime(estimation.timestamp)}</time><small>{formatTime(estimation.timestamp)}</small></div></td>
            <td className="history-amount"><strong>{new Intl.NumberFormat("en-US", { style: "currency", currency: estimation.currency || "USD", minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(estimation.monthlyCost)}</strong><small>/ month</small></td>
            <td><div className="history-row-actions"><button type="button" className="icon-button" onClick={() => setSelectedEstimation(estimation)} title="View estimation" aria-label={`View details for ${estimation.name}`}><Eye size={16} /></button><button type="button" className="icon-button history-download" onClick={() => downloadEstimationPdf(estimation)} title="Download estimation as PDF" aria-label={`Download ${estimation.name} as PDF`}><Download size={15} /></button><button type="button" className="icon-button history-remove" onClick={() => { if (window.confirm(`Delete estimation "${estimation.name}"? This cannot be undone.`)) deleteEstimation(estimation.id); }} title="Delete estimation" aria-label={`Delete ${estimation.name}`}><Trash2 size={15} /></button></div></td>
          </tr>)}</tbody>
        </table>
        {filteredEstimations.length === 0 && <div className="history-page-empty"><Clock size={28} /><strong>{estimations.length === 0 ? "No estimations yet" : "No matching estimations"}</strong></div>}
      </div>
      <footer className="history-page-footer"><span>{filteredEstimations.length > 0 ? `${startIndex + 1}-${Math.min(startIndex + pageSize, filteredEstimations.length)} of ${filteredEstimations.length}` : "0 results"}{filteredEstimations.length !== estimations.length && ` / ${estimations.length} saved`}</span><nav className="history-pagination" aria-label="History pages"><button type="button" className="icon-button" onClick={() => setPage(currentPage - 1)} disabled={currentPage === 1} title="Previous page" aria-label="Previous page"><ChevronLeft size={16} /></button><span>Page {currentPage} of {pageCount}</span><button type="button" className="icon-button" onClick={() => setPage(currentPage + 1)} disabled={currentPage === pageCount} title="Next page" aria-label="Next page"><ChevronRight size={16} /></button></nav></footer>
      </div>
      {selectedEstimation && <SavedEstimationDetails estimation={selectedEstimation} onClose={() => setSelectedEstimation(null)} />}
    </section>
  );
}

function SavedEstimationDetails({ estimation, onClose }: { estimation: SavedEstimation; onClose: () => void }) {
  const dialog = useRef<HTMLElement>(null);
  const { workload, pricing, hourlyTotal, dailyTotal } = estimation.fullResult;
  const currency = estimation.currency || "USD";
  const cpuTotal = workload.resources.reduce((total, resource) => total + resource.cpuCores, 0) * workload.replicas;
  const memoryTotal = workload.resources.reduce((total, resource) => total + resource.memoryGB, 0) * workload.replicas;
  const hasStorage = workload.resources.some(resource => resource.storageGB > 0);
  const hasGPU = workload.resources.some(resource => resource.gpuUnits > 0);

  useEffect(() => {
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    dialog.current?.querySelector<HTMLButtonElement>("button")?.focus();
    return () => {
      document.body.style.overflow = previousOverflow;
      previousFocus?.focus();
    };
  }, []);

  return <div className="estimation-detail-backdrop" role="presentation" onClick={onClose}>
    <section ref={dialog} className="estimation-detail-dialog" role="dialog" aria-modal="true" aria-labelledby="estimation-detail-title" onClick={event => event.stopPropagation()} onKeyDown={event => {
      if (event.key === "Escape") { event.preventDefault(); event.stopPropagation(); onClose(); }
      if (event.key === "Tab") {
        const controls = Array.from(event.currentTarget.querySelectorAll<HTMLElement>("button, summary, [tabindex='0']"));
        const first = controls[0];
        const last = controls[controls.length - 1];
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
      }
    }}>
      <header className="estimation-detail-header"><div><span className="step-label">HISTORIAL / ESTIMATION</span><h2 id="estimation-detail-title">{estimation.name}</h2><p>Saved {formatDateTime(estimation.timestamp)}</p></div><button type="button" className="icon-button" onClick={onClose} title="Close estimation" aria-label="Close estimation details"><X size={18} /></button></header>
      <div className="estimation-detail-content">
        <section className="estimation-cost-summary" aria-label="Saved workload costs">
          <div className="estimation-detail-total"><span>Workload cost / month</span><strong>{formatMoney(estimation.monthlyCost, currency)}</strong><small>Requested resources across {workload.replicas} replicas</small></div>
          <dl className="estimation-cost-rates"><div><dt>Per hour</dt><dd>{formatMoney(hourlyTotal, currency)}</dd></div><div><dt>Per day</dt><dd>{formatMoney(dailyTotal, currency)}</dd></div></dl>
        </section>
        <p className="estimation-cost-note">Requested-resource estimate, not the cluster infrastructure bill.{estimation.estimationMode === "cluster" && !estimation.simulation && " This older record has no saved cluster simulation. Run a new estimation to capture cluster impact."}</p>
        {estimation.estimationMode === "cluster" && estimation.simulation && <section className="estimation-simulation" aria-label="Saved cluster impact">
          <div className="estimation-section-heading"><h3>Saved cluster impact</h3><span>At estimation time / not live</span></div>
          <ClusterImpact simulation={estimation.simulation} clusterName={estimation.clusterName || "Cluster"} />
        </section>}
        <section className="estimation-detail-section"><h3>Workload &amp; pricing basis</h3><dl className="estimation-context-grid">
          <div><dt>Workload</dt><dd>{workload.name}</dd></div><div><dt>Namespace</dt><dd>{workload.namespace || "default"}</dd></div><div><dt>Replicas</dt><dd>{workload.replicas}</dd></div>
          <div><dt>Estimation source</dt><dd>{estimation.estimationMode === "cluster" ? estimation.clusterName || "Connected cluster" : "Catalog pricing"}</dd></div><div><dt>Provider / region</dt><dd>{pricing.provider.toUpperCase()} / {pricing.region}</dd></div><div><dt>Reference instance</dt><dd>{pricing.instanceType}</dd></div>
        </dl></section>
        <section className="estimation-detail-section"><div className="estimation-section-heading"><h3>Resource requests</h3><span>{workload.resources.length} container{workload.resources.length === 1 ? "" : "s"} / pod</span></div>
          <div className="resource-table-wrap"><table className="resource-table"><thead><tr><th scope="col">Container</th><th scope="col">CPU / pod</th><th scope="col">Memory / pod</th>{hasStorage && <th scope="col">Storage / pod</th>}{hasGPU && <th scope="col">GPU / pod</th>}</tr></thead><tbody>{workload.resources.map((resource, index) => <tr key={`${resource.name}-${index}`}><td>{resource.name}</td><td>{resource.cpuCores * 1000}m</td><td>{Number((resource.memoryGB * 1024).toFixed(2))} MiB</td>{hasStorage && <td>{resource.storageGB.toFixed(2)} GiB</td>}{hasGPU && <td>{resource.gpuUnits}</td>}</tr>)}</tbody></table></div>
          <div className="estimation-resource-total"><span>Total across {workload.replicas} replicas</span><strong>{Number(cpuTotal.toFixed(3))} CPU cores</strong><strong>{Number(memoryTotal.toFixed(3))} GiB memory</strong></div>
        </section>
        <details className="estimation-source-section"><summary><FileCode2 size={16} /> Manifest source <span>Saved snapshot</span></summary><pre className="history-source-preview"><code>{estimation.manifest}</code></pre></details>
      </div>
    </section>
  </div>;
}

async function downloadEstimationPdf(estimation: SavedEstimation) {
  const safeName = estimation.fullResult.workload.name.replace(/[^a-zA-Z0-9._-]+/g, "-").replace(/^-+|-+$/g, "") || "estimation";
  const safeId = estimation.id.replace(/[^a-zA-Z0-9_-]+/g, "-") || "record";
  try {
    const pdf = await createEstimationPdf(estimation);
    pdf.save(`kube-cost-workspace-${safeName}-${safeId}.pdf`);
  } catch {
    window.alert("The PDF could not be generated. Please try again.");
  }
}

async function downloadFullHistoryPdf(estimations: SavedEstimation[]) {
  try {
    let pdf: JsPDFDocument | undefined;
    const orderedEstimations = [...estimations].sort((first, second) => second.timestamp - first.timestamp);
    for (const [index, estimation] of orderedEstimations.entries()) {
      pdf = await createEstimationPdf(estimation, pdf, { index: index + 1, total: orderedEstimations.length });
    }
    if (!pdf) return;
    const date = new Date().toISOString().slice(0, 10);
    pdf.setProperties({ title: "Kube Cost Workspace Historial", subject: "Saved Kubernetes cost estimations", creator: "Kube Cost Workspace" });
    pdf.save(`kube-cost-workspace-historial-${date}.pdf`);
  } catch {
    window.alert("The history PDF could not be generated. Please try again.");
  }
}

export async function createEstimationPdf(estimation: SavedEstimation, existingPdf?: JsPDFDocument, recordPosition?: { index: number; total: number }) {
  const [{ jsPDF }, { autoTable }] = await Promise.all([import("jspdf"), import("jspdf-autotable")]);
  const { fullResult, simulation } = estimation;
  const { workload, pricing } = fullResult;
  const currency = estimation.currency || "USD";
  const pdf = existingPdf ?? new jsPDF();
  if (existingPdf) pdf.addPage();
  const pageWidth = pdf.internal.pageSize.getWidth();
  const pageHeight = pdf.internal.pageSize.getHeight();
  const margin = 16;
  const contentWidth = pageWidth - margin * 2;
  let cursorY = 17;

  const addHeading = (title: string) => {
    if (cursorY > pageHeight - 25) { pdf.addPage(); cursorY = 18; }
    pdf.setFont("helvetica", "bold");
    pdf.setFontSize(12);
    pdf.setTextColor(28, 53, 73);
    pdf.text(title, margin, cursorY);
    cursorY += 5;
  };

  const addTable = (head: string[], body: string[][]) => {
    autoTable(pdf, {
      startY: cursorY,
      margin: { left: margin, right: margin, bottom: 16 },
      head: [head],
      body,
      theme: "grid",
      styles: { font: "helvetica", fontSize: 8, cellPadding: 2.5, textColor: [41, 68, 87], lineColor: [217, 224, 221], lineWidth: 0.15, overflow: "linebreak" },
      headStyles: { fillColor: [243, 247, 246], textColor: [69, 86, 78], fontStyle: "bold" },
      alternateRowStyles: { fillColor: [250, 252, 251] },
    });
    cursorY = ((pdf as JsPDFDocument & { lastAutoTable?: { finalY: number } }).lastAutoTable?.finalY ?? cursorY) + 8;
  };

  pdf.setProperties({ title: `Kube Cost Workspace estimation - ${workload.name}`, subject: "Saved Kubernetes workload cost estimation", creator: "Kube Cost Workspace" });
  pdf.setFont("helvetica", "bold");
  pdf.setFontSize(19);
  pdf.setTextColor(23, 37, 31);
  pdf.text("Kube Cost Workspace estimation", margin, cursorY);
  cursorY += 8;
  pdf.setFont("helvetica", "normal");
  pdf.setFontSize(10);
  pdf.setTextColor(41, 68, 87);
  pdf.text(workload.name, margin, cursorY);
  cursorY += 5;
  pdf.setFontSize(8);
  pdf.setTextColor(102, 115, 109);
  pdf.text(`Saved ${formatDateTime(estimation.timestamp)}  |  ID: ${estimation.id}  |  ${estimation.estimationMode === "cluster" ? `Cluster: ${estimation.clusterName || "Connected cluster"}` : "Catalog pricing"}${recordPosition ? `  |  Record ${recordPosition.index} of ${recordPosition.total}` : ""}`, margin, cursorY);
  cursorY += 9;

  addHeading("Workload cost estimate");
  addTable(["Monthly", "Daily", "Hourly", "Replicas", "Namespace"], [[
    formatMoney(estimation.monthlyCost, currency), formatMoney(fullResult.dailyTotal, currency),
    formatMoney(fullResult.hourlyTotal, currency), String(workload.replicas), workload.namespace || "default",
  ]]);

  addHeading("Pricing basis");
  addTable(["Provider", "Region", "Reference instance", "Estimation source"], [[
    pricing.provider.toUpperCase(), pricing.region, pricing.instanceType,
    estimation.estimationMode === "cluster" ? estimation.clusterName || "Connected cluster" : "Catalog",
  ]]);

  addHeading("Resource requests per pod");
  addTable(["Container", "CPU", "Memory", "Storage", "GPU", "Hourly cost"], workload.resources.map(resource => [
    resource.name, `${resource.cpuCores} cores`, `${Number(resource.memoryGB.toFixed(3))} GiB`,
    `${Number(resource.storageGB.toFixed(3))} GiB`, String(resource.gpuUnits), formatMoney(resource.hourlyCost, currency),
  ]));
  const cpuTotal = workload.resources.reduce((total, resource) => total + resource.cpuCores, 0) * workload.replicas;
  const memoryTotal = workload.resources.reduce((total, resource) => total + resource.memoryGB, 0) * workload.replicas;
  pdf.setFont("helvetica", "bold");
  pdf.setFontSize(8);
  pdf.setTextColor(41, 68, 87);
  pdf.text(`Total requests across ${workload.replicas} replicas: ${Number(cpuTotal.toFixed(3))} CPU cores, ${Number(memoryTotal.toFixed(3))} GiB memory`, margin, cursorY - 4);
  cursorY += 3;

  if (simulation) {
    const impact = simulation.impact;
    addHeading("Cluster impact snapshot");
    pdf.setFont("helvetica", "normal");
    pdf.setFontSize(8);
    pdf.setTextColor(102, 115, 109);
    pdf.text(`Cost report captured ${formatDateTime(new Date(simulation.reportAt).getTime())}. This is a saved simulation, not a live cluster reading.`, margin, cursorY);
    cursorY += 5;
    addTable(["Metric", "Before", "After", "Change"], [
      ["Monthly infrastructure bill", formatMoney(impact.billedBefore.monthly, currency), formatMoney(impact.billedAfter.monthly, currency), formatMoney(impact.billedDelta.monthly, currency)],
      ["Idle capacity", formatMoney(impact.idleBefore.monthly, currency), formatMoney(impact.idleAfter.monthly, currency), formatMoney(impact.idleAfter.monthly - impact.idleBefore.monthly, currency)],
      ["Workload attribution", formatMoney(impact.currentRequested.monthly, currency), formatMoney(impact.requested.monthly, currency), formatMoney(impact.requested.monthly - impact.currentRequested.monthly, currency)],
      ["Plan savings", formatMoney(impact.planSavingsBefore.monthly, currency), formatMoney(impact.planSavingsAfter.monthly, currency), formatMoney(impact.planSavingsAfter.monthly - impact.planSavingsBefore.monthly, currency)],
    ]);
    addHeading("Replica placement");
    addTable(["Node", "Replicas", "Capacity"], [
      ...impact.placements.map(placement => [placement.node, String(placement.replicas), placement.new ? "Additional node" : "Existing node"]),
      ...(impact.unschedulable > 0 ? [["Pending", String(impact.unschedulable), "Does not fit modeled nodes"]] : []),
      ...(impact.placements.length === 0 && impact.unschedulable === 0 ? [["-", "0", "No replicas to place"]] : []),
    ]);
    if (simulation.peak) {
      addHeading("Autoscaler peak");
      addTable(["Peak replicas", "Additional nodes", "Monthly bill change"], [[
        String(simulation.peak.workload.replicas), String(simulation.peak.newNodes), formatMoney(simulation.peak.billedDelta.monthly, currency),
      ]]);
    }
    if (impact.warnings.length > 0) {
      addHeading("Warnings");
      const warningLines = pdf.splitTextToSize(impact.warnings.join("  "), contentWidth);
      pdf.setFont("helvetica", "normal");
      pdf.setFontSize(8);
      pdf.setTextColor(164, 68, 54);
      pdf.text(warningLines, margin, cursorY);
      cursorY += warningLines.length * 4 + 5;
    }
  }

  addHeading("Manifest source");
  pdf.setFont("courier", "normal");
  pdf.setFontSize(7);
  pdf.setTextColor(41, 68, 87);
  const sourceLines = pdf.splitTextToSize(estimation.manifest, contentWidth - 4);
  const lineHeight = 3.5;
  for (let index = 0; index < sourceLines.length; index++) {
    if (cursorY > pageHeight - margin) { pdf.addPage(); cursorY = margin; }
    const pageLines = Math.max(1, Math.floor((pageHeight - margin - cursorY) / lineHeight));
    const chunk = sourceLines.slice(index, index + pageLines);
    pdf.text(chunk, margin + 2, cursorY);
    cursorY += chunk.length * lineHeight;
    index += chunk.length - 1;
  }

  return pdf;
}

function formatCost(value: number, currency = "USD") {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  }).format(value);
}

function formatTime(timestamp: number) {
  const date = new Date(timestamp);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return "Just now";
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;

  return date.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

function formatDateTime(timestamp: number) {
  return new Date(timestamp).toLocaleString("en-US", { dateStyle: "medium", timeStyle: "short" });
}

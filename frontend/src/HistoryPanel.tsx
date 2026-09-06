import { Clock, RotateCcw, Search, Trash2, X } from "lucide-react";
import { useEffect, useState } from "react";
import { useEstimationHistory, SavedEstimation } from "./EstimationHistory";

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
              <div className="history-item-actions"><button type="button" className="history-restore-button" onClick={() => onRestoreEstimation(estimation)}><RotateCcw size={13} /> Restore</button><button type="button" className="history-delete-button" onClick={() => deleteEstimation(estimation.id)} title="Delete this estimation" aria-label={`Delete ${estimation.name}`}><Trash2 size={14} /></button></div>
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
  const normalizedQuery = query.trim().toLowerCase();
  const filteredEstimations = estimations.filter((estimation) => {
    const matchesFilter = filter === "all" || estimation.estimationMode === filter;
    const searchableText = [estimation.name, estimation.provider, estimation.region, estimation.clusterName].filter(Boolean).join(" ").toLowerCase();
    return matchesFilter && (!normalizedQuery || searchableText.includes(normalizedQuery));
  });

  function handleClearAll() {
    if (window.confirm("Remove all saved estimations? This cannot be undone.")) clearAll();
  }

  return (
    <section className="history-page">
      <header className="history-page-header">
        <div><span className="step-label">ESTIMATION REGISTRY</span><h1>Past estimations</h1><p>Review the decisions, assumptions, and resource costs from previous analyses.</p></div>
        <div className="history-page-count"><strong>{estimations.length}</strong><span>saved runs</span></div>
      </header>
      <div className="history-page-toolbar">
        <label className="history-search"><Search size={15} /><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search by manifest, provider, region, or cluster" aria-label="Search saved estimates" />{query && <button type="button" onClick={() => setQuery("")} aria-label="Clear search"><X size={13} /></button>}</label>
        <div className="history-filters" role="tablist" aria-label="Filter saved estimates">
          {(["all", "manual", "cluster"] as const).map((value) => <button key={value} type="button" role="tab" aria-selected={filter === value} className={filter === value ? "active" : ""} onClick={() => setFilter(value)}>{value === "all" ? "All runs" : value === "manual" ? "Manual" : "Cluster"}</button>)}
        </div>
        <button type="button" className="history-clear-button" onClick={handleClearAll} disabled={estimations.length === 0}><Trash2 size={13} /> Clear all</button>
      </div>
      <div className="history-page-list">
        {filteredEstimations.map((estimation) => <div key={estimation.id} className="history-page-item" role="button" tabIndex={0} onClick={() => setSelectedEstimation(estimation)} onKeyDown={(event) => { if (event.key === "Enter" || event.key === " ") setSelectedEstimation(estimation); }} aria-label={`View details for ${estimation.name}`}>
          <div className="history-page-item-icon"><Clock size={17} /></div>
          <div className="history-item-content"><div className="history-item-main"><strong className="history-item-name" title={estimation.name}>{estimation.name}</strong><span className="history-cost">{formatCost(estimation.monthlyCost, estimation.currency)}/mo</span></div><div className="history-meta"><span className={`history-mode ${estimation.estimationMode}`}>{estimation.estimationMode === "manual" ? `${estimation.provider?.toUpperCase() ?? "Manual"} · ${estimation.region ?? ""}` : estimation.clusterName || "Cluster"}</span><span className="history-time">{formatTime(estimation.timestamp)}</span></div></div>
          <span className="history-view-hint">View details</span><button type="button" className="history-delete-button" onClick={(event) => { event.stopPropagation(); deleteEstimation(estimation.id); }} aria-label={`Delete ${estimation.name}`}><Trash2 size={14} /></button>
        </div>)}
        {filteredEstimations.length === 0 && <div className="history-page-empty"><Clock size={24} /><strong>{estimations.length === 0 ? "No saved estimates yet" : "No matching estimates"}</strong><span>{estimations.length === 0 ? "Run an estimate and it will be available here for later review." : "Try a different search or filter."}</span></div>}
      </div>
      {selectedEstimation && <div className="estimation-detail-backdrop" role="presentation" onClick={() => setSelectedEstimation(null)}>
        <section className="estimation-detail-dialog" role="dialog" aria-modal="true" aria-labelledby="estimation-detail-title" onClick={(event) => event.stopPropagation()}>
          <header className="estimation-detail-header"><div><span className="step-label">PAST ESTIMATION</span><h2 id="estimation-detail-title">{selectedEstimation.name}</h2><p>{formatDateTime(selectedEstimation.timestamp)}</p></div><button type="button" className="history-close-button" onClick={() => setSelectedEstimation(null)} aria-label="Close estimation details"><X size={18} /></button></header>
          <div className="estimation-detail-content">
            <div className="estimation-detail-total"><span>Estimated monthly cost</span><strong>{formatCost(selectedEstimation.monthlyCost, selectedEstimation.currency)}</strong><small>{selectedEstimation.estimationMode === "manual" ? "Manual pricing estimate" : "Cluster-based estimate"}</small></div>
            <div className="estimation-detail-metrics"><div><span>Hourly</span><strong>{formatCost(selectedEstimation.fullResult.hourlyTotal, selectedEstimation.currency)}</strong></div><div><span>Daily</span><strong>{formatCost(selectedEstimation.fullResult.dailyTotal, selectedEstimation.currency)}</strong></div><div><span>Replicas</span><strong>{selectedEstimation.fullResult.workload.replicas}</strong></div></div>
            <section className="estimation-detail-section"><div className="section-header"><h3>Workload and pricing</h3><span>{selectedEstimation.fullResult.workload.resources.length} container{selectedEstimation.fullResult.workload.resources.length === 1 ? "" : "s"}</span></div><dl className="detail-list"><div><dt>Namespace</dt><dd>{selectedEstimation.fullResult.workload.namespace || "default"}</dd></div><div><dt>Provider</dt><dd>{selectedEstimation.fullResult.pricing.provider.toUpperCase()}</dd></div><div><dt>Region</dt><dd>{selectedEstimation.fullResult.pricing.region}</dd></div><div><dt>Worker node</dt><dd>{selectedEstimation.fullResult.pricing.instanceType}</dd></div></dl></section>
            <section className="estimation-detail-section"><div className="section-header"><h3>Requested resources</h3><span>Per pod</span></div><div className="resource-table-wrap"><table className="resource-table"><thead><tr><th>Container</th><th>CPU</th><th>Memory</th><th>Storage</th><th>GPU</th><th>Hourly</th></tr></thead><tbody>{selectedEstimation.fullResult.workload.resources.map((resource, index) => <tr key={`${resource.name}-${index}`}><td>{resource.name}</td><td>{resource.cpuCores} cores</td><td>{resource.memoryGB.toFixed(2)} GiB</td><td>{resource.storageGB.toFixed(2)} GiB</td><td>{resource.gpuUnits}</td><td>{formatCost(resource.hourlyCost, selectedEstimation.currency)}</td></tr>)}</tbody></table></div></section>
            <section className="estimation-detail-section"><div className="section-header"><h3>Manifest source</h3><span>Read-only snapshot</span></div><pre className="history-source-preview"><code>{selectedEstimation.manifest}</code></pre></section>
          </div>
        </section>
      </div>}
    </section>
  );
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

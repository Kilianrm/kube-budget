import { useState } from "react";
import {
  AlertCircle,
  CircleDollarSign,
  Clock,
  Cpu,
  Lightbulb,
  LoaderCircle,
  PlugZap,
  CloudOff,
  Wallet,
  FlaskConical,
} from "lucide-react";
import { CostAllocationRow, CostProjection, CostReport, CostSeries, CostTrend } from "./backend";
import { defaultHistoryWindow, historyWindows, useCostTrend } from "./costHistoryCache";
import { useCluster } from "./ClusterContext";
import { useCostReport } from "./CostReportContext";
import { BudgetForecastView } from "./BudgetForecast";
import { OptimizationsView } from "./Optimizations";
import { formatMoney, hoursPerMonth } from "./costFormat";
import { Segmented } from "./Segmented";
import { DataFreshness } from "./DataFreshness";
import { useClusterSnapshot } from "./ClusterSnapshotContext";

type CostView = "explorer" | "budget" | "optimize";

const viewTitles: Record<CostView, { title: string; description: string }> = {
  explorer: { title: "Cost explorer", description: "Where the bill goes, how it changes over time, and how much of it nobody requests." },
  budget: { title: "Budget & forecast", description: "Where this month is heading against its budget, and what moved the bill." },
  optimize: { title: "Optimizations", description: "One plan from today's bill to the optimized bill: what to change, what it saves, and how risky it is." },
};

const emptyProjection: CostProjection = { hourly: 0, daily: 0, monthly: 0, yearly: 0 };

export function CostSection({ onConnectClick, onTestWorkload }: { onConnectClick: () => void; onTestWorkload: () => void }) {
  const { isConnected } = useCluster();
  const { report, isLoading, error, setOptimization } = useCostReport();
  const { snapshot } = useClusterSnapshot();
  const [view, setView] = useState<CostView>("explorer");

  if (!isConnected()) {
    return (
      <section className="cluster-workspace cluster-locked">
        <div className="cluster-locked-content">
          <div className="locked-visual" aria-hidden="true"><div className="locked-icon"><CircleDollarSign size={48} /></div></div>
          <h2>Cost workspace locked</h2>
          <p>Connect a Kubernetes cluster to break down what its infrastructure costs and what its workloads actually ask for.</p>
          <button className="primary-button" type="button" onClick={onConnectClick}>
            <PlugZap size={18} /><span>Connect a cluster</span>
          </button>
        </div>
      </section>
    );
  }

  // Costs come from a cloud's public price list; the snapshot says whether
  // this cluster runs somewhere we can price.
  const platform = snapshot?.platform;
  if (platform && !platform.supported) {
    return (
      <section className="cluster-workspace cluster-locked">
        <div className="cluster-locked-content">
          <div className="locked-visual" aria-hidden="true"><div className="locked-icon"><CloudOff size={48} /></div></div>
          <h2>Costs are not available for this cluster</h2>
          <p>Cost figures come from the public price lists of AWS, Google Cloud and Azure. This cluster cannot be priced because {platform.reason}.</p>
          <p className="locked-hint">The Cluster section and catalog pricing in Manifest still work. To see costs, connect a cluster that runs on a supported cloud.</p>
          <button className="primary-button" type="button" onClick={onConnectClick}>
            <PlugZap size={18} /><span>Connect a different cluster</span>
          </button>
        </div>
      </section>
    );
  }

  return (
    <>
      <aside className="cluster-sidebar" aria-label="Cost workspace options">
        <span className="sidebar-label">COST</span>
        <button type="button" className={view === "explorer" ? "active" : ""} onClick={() => setView("explorer")}><CircleDollarSign size={16} /><span>Cost explorer</span></button>
        <button type="button" className={view === "budget" ? "active" : ""} onClick={() => setView("budget")}><Wallet size={16} /><span>Budget &amp; forecast</span></button>
        <button type="button" className={view === "optimize" ? "active" : ""} onClick={() => setView("optimize")}><Lightbulb size={16} /><span>Optimizations</span></button>
        <div className="cluster-sidebar-spacer" />
      </aside>
      <section className="cluster-management-view">
        <div className="management-header">
          <div>
            <span className="step-label">COST / {view.toUpperCase()}</span>
            <h1>{viewTitles[view].title}</h1>
            <p>{viewTitles[view].description}</p>
          </div>
          <div className="management-actions">
            <button type="button" className="secondary-button test-workload-button" onClick={onTestWorkload} title="Simulate deploying a manifest on this cluster">
              <FlaskConical size={14} /> Test a workload
            </button>
            <DataFreshness updatedAt={report ? String(report.report.generatedAt) : null} isRefreshing={isLoading} failed={Boolean(error && report)} />
          </div>
        </div>

        {error && <div className="error-message"><AlertCircle size={16} /><span>{error}</span></div>}
        {!report && isLoading && <div className="cluster-screen-content"><section className="cluster-panel"><div className="panel-footnote"><LoaderCircle className="spin" size={15} /> Collecting and pricing cluster resources...</div></section></div>}
        {report && view === "explorer" && <CostExplorerView report={report.report} clusterId={report.clusterId} onShowChanges={() => setView("budget")} />}
        {report && view === "budget" && <BudgetForecastView report={report.report} clusterId={report.clusterId} />}
        {report && view === "optimize" && <OptimizationsView report={report.report} clusterId={report.clusterId} optimization={report.optimization} onChange={setOptimization} />}
      </section>
    </>
  );
}

type AllocationGroup = "namespace" | "workload";

const billSegments = [
  { key: "workloads", label: "Workload requests", color: "#2a78d6" },
  { key: "volumes", label: "Persistent volumes", color: "#1baf7a" },
  { key: "idle", label: "Idle node capacity", color: "#eb6834" },
  { key: "shared", label: "Shared cluster costs", color: "#4a3aa7" },
] as const;

function CostExplorerView({ report, clusterId, onShowChanges }: { report: CostReport; clusterId: string; onShowChanges: () => void }) {
  const [group, setGroup] = useState<AllocationGroup>("namespace");
  const provisioned = totalFor(report, "provisioned");
  const requested = totalFor(report, "requested");
  const idle = report.idle ?? emptyProjection;
  const shared = report.shared ?? emptyProjection;
  const billedKinds = dimension(report, "provisioned", "subjectKind");
  const volumes = billedKinds.volume ?? emptyProjection;
  const allocated = requested.hourly + volumes.hourly;
  const share = (hourly: number) => (provisioned.hourly > 0 ? Math.round((hourly / provisioned.hourly) * 100) : 0);
  const items = report.items ?? [];
  const nodeCount = items.filter((item) => item.subject.kind === "node").length;
  const volumeCount = items.filter((item) => item.subject.kind === "volume").length;
  const idleShare = share(idle.hourly);

  return (
    <div className="cluster-screen-content">
      <div className="cluster-metrics">
        <CostMetric label="Billed infrastructure" projection={provisioned} currency={report.currency} detail={`${nodeCount} node${nodeCount === 1 ? "" : "s"}${shared.hourly > 0 ? " · control plane" : ""}${volumeCount > 0 ? ` · ${volumeCount} volume${volumeCount === 1 ? "" : "s"}` : ""}`} />
        <CostMetric label="Allocated to workloads" projection={projectHourly(allocated, provisioned)} currency={report.currency} detail={`${share(allocated)}% of the bill · requests and volumes`} tone="neutral" />
        <CostMetric label="Idle node capacity" projection={idle} currency={report.currency} detail={`${idleShare}% of the bill is requested by nobody`} tone={idleShare > 30 ? "warning" : "good"} />
        <CostMetric label="Shared cluster costs" projection={shared} currency={report.currency} detail="Control plane · not attributable to a team" />
      </div>

      <SpendOverTime clusterId={clusterId} currency={report.currency} capturedAt={String(report.generatedAt)} onShowChanges={onShowChanges} />

      <section className="cluster-panel">
        <PanelTitle title="Where the bill goes" />
        <BillBar
          values={{ workloads: requested.hourly, volumes: volumes.hourly, idle: idle.hourly, shared: shared.hourly }}
          total={provisioned}
         
          currency={report.currency}
        />
      </section>

      <section className="cluster-panel">
        <PanelTitle title="Node capacity by resource" />
        <ResourceEfficiency report={report} />
        <div className="panel-footnote">
          <Cpu size={14} /> {report.usage
            ? `Used is one metrics-server sample priced at the same node rates${report.usage.withoutUsage > 0 ? `; ${report.usage.withoutUsage} workload${report.usage.withoutUsage === 1 ? " has" : "s have"} no sample yet and count as requested only` : ""}. Requested but unused capacity is fixed by lowering requests; idle capacity by fewer or smaller nodes.`
            : "Requested is what pods reserve, not what they use; install metrics-server to compare with actual usage. Idle capacity is fixed by fewer or smaller nodes."}
        </div>
      </section>

      <section className="cluster-panel">
        <div className="cluster-panel-heading">
          <h2>Allocation</h2>
          <Segmented label="Group allocation by" value={group} onChange={setGroup} options={[{ value: "namespace", label: "Namespace" }, { value: "workload", label: "Workload" }]} />
        </div>
        <AllocationTable report={report} group={group} />
      </section>

      <section className="cluster-panel">
        <PanelTitle title="Billed infrastructure by node group" />
        <CostBars buckets={nodeGroupBuckets(report)} currency={report.currency} />
      </section>

      <AssumptionsPanel report={report} />
    </div>
  );
}

function BillBar({ values, total, currency }: { values: Record<(typeof billSegments)[number]["key"], number>; total: CostProjection; currency: string }) {
  const [focused, setFocused] = useState<string | null>(null);
  const segments = billSegments
    .map((segment) => ({ ...segment, hourly: values[segment.key] }))
    .filter((segment) => segment.hourly > 0);
  if (total.hourly <= 0 || segments.length === 0) {
    return <div className="panel-footnote"><AlertCircle size={14} /> Nothing billed was priced in this report.</div>;
  }

  const scale = monthly(total) / total.hourly;
  const percent = (hourly: number) => (hourly / total.hourly) * 100;
  const active = segments.find((segment) => segment.key === focused);

  return (
    <div className="bill-breakdown">
      <div className="bill-readout" aria-live="polite">
        {active
          ? <><span className="bill-swatch" style={{ background: active.color }} />{active.label} · <strong>{formatMoney(active.hourly * scale, currency)}</strong> · {percent(active.hourly).toFixed(1)}% of the bill</>
          : <>Total billed · <strong>{formatMoney(monthly(total), currency)}</strong> per month</>}
      </div>
      <div className="bill-bar" role="img" aria-label={segments.map((segment) => `${segment.label} ${percent(segment.hourly).toFixed(0)}%`).join(", ")}>
        {segments.map((segment) => (
          <span
            key={segment.key}
            className={focused && focused !== segment.key ? "dimmed" : ""}
            style={{ flexGrow: segment.hourly, background: segment.color }}
            onMouseEnter={() => setFocused(segment.key)}
            onMouseLeave={() => setFocused(null)}
          />
        ))}
      </div>
      <div className="bill-legend">
        {segments.map((segment) => (
          <button
            type="button"
            key={segment.key}
            className={focused === segment.key ? "active" : ""}
            onMouseEnter={() => setFocused(segment.key)}
            onMouseLeave={() => setFocused(null)}
            onFocus={() => setFocused(segment.key)}
            onBlur={() => setFocused(null)}
          >
            <span className="bill-swatch" style={{ background: segment.color }} />
            <span>{segment.label}</span>
            <strong>{formatMoney(segment.hourly * scale, currency)}</strong>
            <small>{percent(segment.hourly).toFixed(0)}%</small>
          </button>
        ))}
      </div>
    </div>
  );
}

function ResourceEfficiency({ report}: { report: CostReport }) {
  const billed = dimension(report, "provisioned", "component");
  const idle = (report.idleByComponent ?? {}) as Record<string, CostProjection>;
  const used = (report.usage?.used ?? {}) as Record<string, CostProjection>;
  const hasUsage = Boolean(report.usage);
  const rows = (["cpu", "memory", "gpu"] as const)
    .map((component) => {
      const total = billed[component]?.hourly ?? 0;
      const unused = Math.min(idle[component]?.hourly ?? 0, total);
      const requested = total - unused;
      return { component, total, requested, unused, used: Math.min(used[component]?.hourly ?? 0, requested) };
    })
    .filter((row) => row.total > 0);

  if (rows.length === 0) {
    return <div className="panel-footnote"><AlertCircle size={14} /> Node prices could not be split by resource.</div>;
  }

  // Every component shares the same window ratio, taken from a backend projection.
  const reference = billed[rows[0].component];
  const scale = (hourly: number) => hourly * (monthly(reference) / reference.hourly);
  const percentOf = (part: number, whole: number) => Math.round((part / whole) * 100);
  return (
    <>
      <div className="namespace-bars">
        {rows.map((row) => (
          <div className="namespace-bar" key={row.component}>
            <div>
              <span className="namespace-name">
                {componentLabels[row.component]} · {percentOf(row.requested, row.total)}% requested{hasUsage && ` · ${percentOf(row.used, row.total)}% used`}
              </span>
              <strong>{formatMoney(scale(row.requested), report.currency)}<small>of {formatMoney(scale(row.total), report.currency)} · {formatMoney(scale(row.unused), report.currency)} idle</small></strong>
            </div>
            <div className="split-track" title={hasUsage ? `${percentOf(row.used, row.total)}% used, ${percentOf(row.requested - row.used, row.total)}% requested but unused, ${percentOf(row.unused, row.total)}% idle` : `${percentOf(row.requested, row.total)}% requested, ${percentOf(row.unused, row.total)}% idle`}>
              {hasUsage
                ? <>
                  {row.used > 0 && <span style={{ flexGrow: row.used, background: "#2a78d6" }} />}
                  {row.requested - row.used > 0 && <span className="hatched-requested" style={{ flexGrow: row.requested - row.used }} />}
                </>
                : row.requested > 0 && <span style={{ flexGrow: row.requested, background: "#2a78d6" }} />}
              {row.unused > 0 && <span style={{ flexGrow: row.unused, background: "#eb6834" }} />}
            </div>
          </div>
        ))}
      </div>
      <div className="capacity-legend">
        {hasUsage
          ? <><span><i style={{ background: "#2a78d6" }} /> Used</span><span><i className="hatched-requested" /> Requested, not used</span></>
          : <span><i style={{ background: "#2a78d6" }} /> Requested</span>}
        <span><i style={{ background: "#eb6834" }} /> Idle</span>
      </div>
    </>
  );
}

const allocationPageSize = 15;

function AllocationTable({ report, group}: { report: CostReport; group: AllocationGroup }) {
  const [showAll, setShowAll] = useState(false);
  const rows = report.allocation?.[group] ?? [];
  const provisioned = totalFor(report, "provisioned");
  const itemsById = new Map((report.items ?? []).map((item) => [item.subject.id, item]));

  if (rows.length === 0) {
    return <div className="panel-footnote"><AlertCircle size={14} /> Nothing to allocate in this report.</div>;
  }

  const consumers = rows.filter((row) => row.kind === "consumer");
  const closing = rows.filter((row) => row.kind !== "consumer");
  const visible = showAll ? consumers : consumers.slice(0, allocationPageSize);
  const hidden = consumers.slice(visible.length);
  const hiddenTotal = hidden.reduce((total, row) => total + monthly(row.cost), 0);
  const unpriced = consumers.reduce((total, row) => total + row.unpriced, 0);
  const component = (row: CostAllocationRow, name: string) => {
    const value = monthly(row.components?.[name] ?? emptyProjection);
    return value > 0 ? formatMoney(value, report.currency) : "-";
  };
  const hasUsage = Boolean(report.usage);
  const efficiency = (row: CostAllocationRow) => {
    const cpu = row.efficiency?.cpu;
    const memory = row.efficiency?.memory;
    if (cpu === undefined && memory === undefined) return "-";
    const format = (value?: number) => (value === undefined ? "-" : `${Math.round(value * 100)}%`);
    return `CPU ${format(cpu)} · Mem ${format(memory)}`;
  };
  const share = (row: CostAllocationRow) => (provisioned.hourly > 0 ? `${((row.cost.hourly / provisioned.hourly) * 100).toFixed(1)}%` : "-");

  const renderRow = (row: CostAllocationRow) => {
    const item = itemsById.get(row.key);
    const kind = row.kind === "consumer" && group === "workload"
      ? (row.subjectKind === "volume" ? "PersistentVolumeClaim" : String(item?.detail?.kind ?? "Workload"))
      : "";
    return (
      <tr key={row.key} className={row.kind !== "consumer" ? `allocation-${row.kind}` : ""}>
        <td>
          <strong>{row.name}</strong>
          {row.kind === "consumer" && group === "workload" && <small>{kind} · {row.namespace}</small>}
          {row.kind === "consumer" && group === "namespace" && <small>{row.items} item{row.items === 1 ? "" : "s"}</small>}
          {row.kind === "idle" && <small>Billed node capacity no pod requests</small>}
          {row.kind === "shared" && <small>Control plane and other cluster-wide fees</small>}
        </td>
        <td className="numeric-cell">{component(row, "cpu")}</td>
        <td className="numeric-cell">{component(row, "memory")}</td>
        <td className="numeric-cell">{component(row, "storage")}</td>
        <td className="cost-cell">{row.confidence === "unknown" ? "Unknown" : formatMoney(monthly(row.cost), report.currency)}</td>
        <td className="numeric-cell">{share(row)}</td>
        {hasUsage && <td className="numeric-cell">{row.kind === "consumer" ? efficiency(row) : ""}</td>}
        <td>
          {row.unpriced > 0
            ? <span className="status-warning"><AlertCircle size={12} /> {row.unpriced} unpriced</span>
            : <span className={row.confidence === "exact" ? "status-good" : "status-warning"}>{row.confidence}</span>}
        </td>
      </tr>
    );
  };

  return (
    <>
      <div className="cluster-table-wrap">
        <table className="cluster-table allocation-table">
          <thead><tr><th>{group === "namespace" ? "Namespace" : "Workload"}</th><th>CPU</th><th>Memory</th><th>Storage</th><th>Monthly</th><th>Of bill</th>{hasUsage && <th className="numeric-header" title="Used cost over requested cost, from one metrics-server sample">Used / requested</th>}<th>Confidence</th></tr></thead>
          <tbody>
            {visible.map(renderRow)}
            {hidden.length > 0 && (
              <tr className="allocation-more">
                <td colSpan={4}><button type="button" className="link-button" onClick={() => setShowAll(true)}>Show {hidden.length} more</button></td>
                <td className="cost-cell">{formatMoney(hiddenTotal, report.currency)}</td>
                <td colSpan={hasUsage ? 3 : 2} />
              </tr>
            )}
            {closing.map(renderRow)}
          </tbody>
          <tfoot>
            <tr>
              <td><strong>Total billed</strong></td>
              <td colSpan={3} />
              <td className="cost-cell">{formatMoney(monthly(provisioned), report.currency)}</td>
              <td className="numeric-cell">100%</td>
              <td colSpan={hasUsage ? 2 : 1} />
            </tr>
          </tfoot>
        </table>
      </div>
      {unpriced > 0 && (
        <div className="panel-footnote">
          <AlertCircle size={14} /> {unpriced} workload{unpriced === 1 ? " has" : "s have"} no resource requests. What they consume is counted as idle until requests are set.
        </div>
      )}
    </>
  );
}

const trendWindows = historyWindows;

const trendSegments = [
  { key: "allocated", label: "Allocated", color: "#2a78d6" },
  { key: "idle", label: "Idle", color: "#eb6834" },
  { key: "shared", label: "Shared", color: "#4a3aa7" },
] as const;

function SpendOverTime({ clusterId, currency, capturedAt, onShowChanges }: { clusterId: string; currency: string; capturedAt: string; onShowChanges: () => void }) {
  const [windowIndex, setWindowIndex] = useState<number>(defaultHistoryWindow);
  const [focused, setFocused] = useState<number | null>(null);
  const selected = trendWindows[windowIndex];
  // capturedAt changes on every report, and every report writes a capture.
  const { trend, error } = useCostTrend(clusterId, selected.days, selected.bucket, capturedAt);

  const series = trend?.series;
  const points = series?.points ?? [];
  const columns = points.map((point) => {
    const allocated = Object.values(point.byNamespace ?? {}).reduce((total, spend) => total + spend, 0);
    return { point, values: { allocated, idle: point.idleUSD, shared: point.sharedUSD } as Record<(typeof trendSegments)[number]["key"], number> };
  });
  const highest = columns.reduce((maximum, column) => Math.max(maximum, column.point.provisionedUSD), 0);
  const coverage = series && series.windowHours > 0 ? Math.round((series.coveredHours / series.windowHours) * 100) : 0;
  const bucketLabel = (point: CostSeries["points"][number]) => new Date(point.start).toLocaleString(undefined, selected.bucket === "day" ? { month: "short", day: "numeric" } : { hour: "2-digit", minute: "2-digit" });
  const active = focused !== null ? columns[focused] : null;

  return (
    <section className="cluster-panel">
      <div className="cluster-panel-heading">
        <h2>Spend over time</h2>
        <Segmented label="History range" value={windowIndex} onChange={setWindowIndex} options={trendWindows.map((option, index) => ({ value: index, label: option.label, title: `Last ${option.previous.replace("previous ", "")}` }))} />
      </div>

      {error && <div className="panel-footnote"><AlertCircle size={14} /> {error}</div>}
      {!error && !series && <div className="panel-footnote"><LoaderCircle className="spin" size={14} /> Reading recorded captures...</div>}
      {series && (
        <>
          <div className="trend-summary">
            <div><span>Recorded billed spend</span><strong>{formatMoney(series.totalProvisionedUSD, currency)}</strong></div>
            <div><span>Average billed rate</span><strong>{series.coveredHours > 0 ? `${formatMoney((series.totalProvisionedUSD / series.coveredHours) * hoursPerMonth, currency)}/mo` : "-"}</strong><RateChange current={series} previous={trend!.previous} label={selected.previous} />{(trend!.drivers ?? []).length > 0 && <button type="button" className="link-button" onClick={onShowChanges}>What changed?</button>}</div>
            <div><span>Window captured</span><strong>{coverage}%</strong><small>{series.coveredHours.toFixed(1)} of {series.windowHours.toFixed(0)} hours</small></div>
          </div>

          {highest === 0
            ? <div className="panel-footnote"><AlertCircle size={14} /> Nothing recorded in this window yet. A capture is written every time the cost report runs.</div>
            : <>
              <div className="bill-readout" aria-live="polite">
                {active
                  ? active.point.coveredHours === 0
                    ? <>{bucketLabel(active.point)} · no capture</>
                    : <>{bucketLabel(active.point)} · <strong>{formatMoney(active.point.provisionedUSD, currency)}</strong> billed{trendSegments.map((segment) => <span key={segment.key} className="trend-readout-part"><span className="bill-swatch" style={{ background: segment.color }} />{segment.label} {formatMoney(active.values[segment.key], currency)}</span>)}{active.point.bucketHours > 0 && active.point.coveredHours / active.point.bucketHours < 0.9 && <> · {Math.round((active.point.coveredHours / active.point.bucketHours) * 100)}% captured</>}</>
                  : <>Hover a {selected.bucket} to see its split</>}
              </div>
              <div className="trend-chart" role="img" aria-label={`Billed spend per ${selected.bucket}, split into allocated, idle and shared`}>
                {columns.map((column, index) => (
                  <div
                    key={String(column.point.start)}
                    className={`trend-column ${focused !== null && focused !== index ? "dimmed" : ""}`}
                    onMouseEnter={() => setFocused(index)}
                    onMouseLeave={() => setFocused(null)}
                  >
                    {column.point.coveredHours === 0
                      ? <span className="trend-gap" />
                      : <div className="trend-stack" style={{ height: `${(column.point.provisionedUSD / highest) * 100}%` }}>
                        {[...trendSegments].reverse().filter((segment) => column.values[segment.key] > 0).map((segment) => (
                          <span key={segment.key} style={{ flexGrow: column.values[segment.key], background: segment.color }} />
                        ))}
                      </div>}
                  </div>
                ))}
              </div>
              <div className="trend-axis">
                {columns.map((column, index) => (
                  <span key={String(column.point.start)}>{index % Math.max(1, Math.ceil(columns.length / 8)) === 0 ? bucketLabel(column.point) : ""}</span>
                ))}
              </div>
              <div className="bill-legend">
                {trendSegments.map((segment) => (
                  <div key={segment.key} className="bill-legend-item">
                    <span className="bill-swatch" style={{ background: segment.color }} />
                    <span>{segment.label}</span>
                    <strong>{formatMoney(columns.reduce((total, column) => total + column.values[segment.key], 0), currency)}</strong>
                  </div>
                ))}
              </div>
              <TopNamespaces totals={series.totalByNamespace ?? {}} currency={currency} />
            </>}
          <div className="panel-footnote">
            <Clock size={14} /> Spend is measured from captures taken while the app runs; hatched {selected.bucket}s had none and are left empty rather than estimated. The change compares average rates, so different coverage does not distort it.
          </div>
        </>
      )}
    </section>
  );
}

function RateChange({ current, previous, label }: { current: CostSeries; previous: CostTrend["previous"]; label: string }) {
  if (current.coveredHours <= 0 || previous.coveredHours < 1) {
    return <small>No captures in the {label}</small>;
  }
  const now = current.totalProvisionedUSD / current.coveredHours;
  const before = previous.provisionedUSD / previous.coveredHours;
  if (before <= 0) return <small>No billed spend in the {label}</small>;
  const change = ((now - before) / before) * 100;
  if (Math.abs(change) < 0.5) return <small>Unchanged vs the {label}</small>;
  return <small>{change > 0 ? "▲" : "▼"} {Math.abs(change).toFixed(1)}% vs the {label}</small>;
}

function TopNamespaces({ totals, currency }: { totals: Record<string, number>; currency: string }) {
  const entries = Object.entries(totals).filter(([, spend]) => spend > 0).sort((left, right) => right[1] - left[1]);
  if (entries.length === 0) return null;
  const shown = entries.slice(0, 5);
  const rest = entries.slice(5).reduce((total, [, spend]) => total + spend, 0);
  const highest = shown[0][1];
  return (
    <div className="trend-namespaces">
      <h3>Top namespaces in this window</h3>
      <div className="namespace-bars">
        {shown.map(([name, spend]) => (
          <div className="namespace-bar" key={name}>
            <div><span className="namespace-name">{name}</span><strong>{formatMoney(spend, currency)}</strong></div>
            <div className="capacity-track"><span style={{ width: `${Math.round((spend / highest) * 100)}%` }} /></div>
          </div>
        ))}
        {rest > 0 && <div className="panel-footnote standalone">{entries.length - shown.length} more namespace{entries.length - shown.length === 1 ? "" : "s"} · {formatMoney(rest, currency)}</div>}
      </div>
    </div>
  );
}

function AssumptionsPanel({ report }: { report: CostReport }) {
  const assumptions = report.assumptions ?? [];
  const warnings = report.warnings ?? [];

  return (
    <details className="cluster-panel assumptions-panel" open={warnings.length > 0}>
      <summary className="cluster-panel-heading">
        <h2>Assumptions and gaps</h2>
        <small>{assumptions.length} assumption{assumptions.length === 1 ? "" : "s"} · {warnings.length} warning{warnings.length === 1 ? "" : "s"}</small>
      </summary>
      {assumptions.map((assumption) => (
        <div className="panel-footnote" key={`${assumption.key}-${assumption.detail}`}><CircleDollarSign size={14} /> {assumption.detail}</div>
      ))}
      {warnings.map((warning, index) => (
        <div className="panel-footnote" key={`${warning.code}-${warning.subject}-${index}`}><AlertCircle size={14} /> {warning.subject ? `${warning.subject}: ` : ""}{warning.message}</div>
      ))}
      <div className="panel-footnote">
        <AlertCircle size={14} /> Figures are estimates from published list prices. Data transfer, load balancers and NAT gateways are not priced, and discounts are not applied.
      </div>
    </details>
  );
}

function CostMetric({ label, projection, currency, detail, tone = "neutral" }: { label: string; projection: CostProjection; currency: string; detail?: string; tone?: "neutral" | "good" | "warning" }) {
  return (
    <div className={`cluster-metric ${tone}`}>
      <span>{label}</span>
      <strong title={`${formatMoney(projection.hourly, currency)} per hour · ${formatMoney(projection.daily, currency)} per day`}>{formatMoney(monthly(projection), currency)}<span className="rate-unit">/mo</span></strong>
      {detail && <small>{detail}</small>}
    </div>
  );
}

function CostBars({ buckets, currency }: { buckets: Record<string, CostProjection>; currency: string }) {
  const entries = Object.entries(buckets)
    .map(([name, projection]) => ({ name, value: monthly(projection) }))
    .filter((entry) => entry.value > 0)
    .sort((left, right) => right.value - left.value)
    .slice(0, 8);

  if (entries.length === 0) {
    return <div className="panel-footnote"><AlertCircle size={14} /> Nothing priced in this dimension.</div>;
  }

  const highest = entries[0].value;
  return (
    <div className="namespace-bars">
      {entries.map((entry) => (
        <div className="namespace-bar" key={entry.name}>
          <div>
            <span className="namespace-name">{entry.name}</span>
            <strong>{formatMoney(entry.value, currency)}</strong>
          </div>
          <div className="capacity-track"><span style={{ width: `${Math.round((entry.value / highest) * 100)}%` }} /></div>
        </div>
      ))}
    </div>
  );
}

function PanelTitle({ title }: { title: string }) {
  return <div className="cluster-panel-heading"><h2>{title}</h2></div>;
}

const componentLabels: Record<string, string> = {
  cpu: "CPU",
  memory: "Memory",
  storage: "Storage",
  gpu: "GPU",
  flat: "Flat fees",
};

function nodeGroupBuckets(report: CostReport): Record<string, CostProjection> {
  const buckets: Record<string, CostProjection> = { ...dimension(report, "provisioned", "parent") };
  const grouped = Object.values(buckets).reduce((total, projection) => total + projection.hourly, 0);
  const nodes = dimension(report, "provisioned", "subjectKind").node;
  if (nodes && nodes.hourly - grouped > 1e-9) {
    buckets["Nodes without a node group"] = projectHourly(nodes.hourly - grouped, nodes);
  }
  return buckets;
}

/** Scales an hourly figure with the same window ratios as a reference projection. */
function projectHourly(hourly: number, reference: CostProjection): CostProjection {
  if (reference.hourly <= 0) return { ...emptyProjection };
  const ratio = hourly / reference.hourly;
  return { hourly, daily: reference.daily * ratio, monthly: reference.monthly * ratio, yearly: reference.yearly * ratio };
}

function totalFor(report: CostReport, basis: string): CostProjection {
  return report.totals?.[basis] ?? emptyProjection;
}

function dimension(report: CostReport, basis: string, name: string): Record<string, CostProjection> {
  return (report.byDimension?.[basis]?.[name] as Record<string, CostProjection>) ?? {};
}

/** The explorer shows every rate per month, the unit budgets and bills use. */
function monthly(projection: CostProjection | undefined) {
  return projection?.monthly ?? 0;
}


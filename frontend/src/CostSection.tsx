import { useCallback, useEffect, useState } from "react";
import {
  AlertCircle,
  CircleDollarSign,
  Clock,
  Cpu,
  HardDrive,
  Layers3,
  Lightbulb,
  LoaderCircle,
  PlugZap,
  RefreshCw,
  Server,
  ServerCog,
} from "lucide-react";
import { getCostReport, getCostTrend, CostLineItem, CostProjection, CostRecommendation, CostReport, CostReportResult, CostTrend } from "./backend";
import { useCluster } from "./ClusterContext";

type CostView = "explorer" | "trend" | "optimize";
type CostWindow = "hourly" | "daily" | "monthly";

const windowLabels: Record<CostWindow, string> = {
  hourly: "Per hour",
  daily: "Per day",
  monthly: "Per month",
};

const emptyProjection: CostProjection = { hourly: 0, daily: 0, monthly: 0, yearly: 0 };

export function CostSection({ onConnectClick }: { onConnectClick: () => void }) {
  const { isConnected, clusterConnection } = useCluster();
  const [view, setView] = useState<CostView>("explorer");
  const [timeWindow, setTimeWindow] = useState<CostWindow>("monthly");
  const [report, setReport] = useState<CostReportResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");

  const refresh = useCallback(async () => {
    if (!clusterConnection?.context) {
      setError("The connected cluster does not have a kubeconfig context.");
      return;
    }
    setIsLoading(true);
    setError("");
    try {
      const nextReport = await getCostReport(
        clusterConnection.kubeconfigPath ?? "",
        clusterConnection.context,
        clusterConnection.namespace,
        clusterConnection.provider === "aws-eks" ? {
          name: "aws-eks",
          clusterName: clusterConnection.clusterName ?? clusterConnection.name,
          region: clusterConnection.region ?? "",
          profile: clusterConnection.profile ?? "default",
          roleArn: clusterConnection.roleArn ?? "",
        } : undefined,
        { region: clusterConnection.region ?? "" },
      );
      setReport(nextReport);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setIsLoading(false);
    }
  }, [clusterConnection?.context, clusterConnection?.kubeconfigPath, clusterConnection?.namespace, clusterConnection?.provider, clusterConnection?.clusterName, clusterConnection?.name, clusterConnection?.region, clusterConnection?.profile, clusterConnection?.roleArn]);

  useEffect(() => {
    if (isConnected()) void refresh();
  }, [isConnected, refresh]);

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

  return (
    <>
      <aside className="cluster-sidebar" aria-label="Cost workspace options">
        <span className="sidebar-label">COST</span>
        <button type="button" className={view === "explorer" ? "active" : ""} onClick={() => setView("explorer")}><CircleDollarSign size={16} /><span>Cost explorer</span></button>
        <button type="button" className={view === "trend" ? "active" : ""} onClick={() => setView("trend")}><Clock size={16} /><span>History</span></button>
        <button type="button" className={view === "optimize" ? "active" : ""} onClick={() => setView("optimize")}><Lightbulb size={16} /><span>Optimizations</span></button>
        <div className="cluster-sidebar-spacer" />
      </aside>
      <section className="cluster-management-view">
        <div className="management-header">
          <div>
            <span className="step-label">COST / {view.toUpperCase()}</span>
            <h1>{view === "explorer" ? "Cost explorer" : view === "trend" ? "Recorded history" : "Optimizations"}</h1>
            <p>{view === "explorer"
              ? "Compare what the provider bills against what the workloads request."
              : view === "trend"
                ? "Spend measured from recorded captures, with the periods that were never captured left empty."
                : "Turn the gap between billed and requested capacity into actions."}</p>
          </div>
          <div className="management-actions">
            {view !== "trend" && <div className="mode-selector" role="group" aria-label="Time window">
              {(Object.keys(windowLabels) as CostWindow[]).map((option) => (
                <button key={option} type="button" className={`mode-button ${timeWindow === option ? "active" : ""}`} onClick={() => setTimeWindow(option)}>
                  <span><strong>{option === "hourly" ? "Hour" : option === "daily" ? "Day" : "Month"}</strong></span>
                </button>
              ))}
            </div>}
            <button type="button" className="refresh-button" onClick={refresh} disabled={isLoading} title="Recalculate cost report" aria-label="Recalculate cost report">
              <RefreshCw className={isLoading ? "spin" : ""} size={15} />
            </button>
            <span className="cluster-data-badge"><span /> {report ? `Priced / ${new Date(report.report.generatedAt).toLocaleTimeString()}` : "Waiting for data"}</span>
          </div>
        </div>

        {error && <div className="error-message"><AlertCircle size={16} /><span>{error}</span></div>}
        {!report && isLoading && <div className="cluster-screen-content"><section className="cluster-panel"><div className="panel-footnote"><LoaderCircle className="spin" size={15} /> Collecting and pricing cluster resources...</div></section></div>}
        {report && view === "explorer" && <CostExplorerView report={report.report} timeWindow={timeWindow} />}
        {report && view === "trend" && <CostTrendView clusterId={report.clusterId} currency={report.report.currency} />}
        {report && view === "optimize" && <OptimizationView report={report.report} recommendations={report.recommendations ?? []} timeWindow={timeWindow} />}
      </section>
    </>
  );
}

function CostExplorerView({ report, timeWindow }: { report: CostReport; timeWindow: CostWindow }) {
  const provisioned = totalFor(report, "provisioned");
  const requested = totalFor(report, "requested");
  const idle = report.idle ?? emptyProjection;
  const coverage = provisioned.hourly > 0 ? Math.round((requested.hourly / provisioned.hourly) * 100) : 0;

  return (
    <div className="cluster-screen-content">
      <div className="cluster-metrics">
        <CostMetric label="Billed infrastructure" projection={provisioned} timeWindow={timeWindow} currency={report.currency} detail="Nodes, control plane and volumes" />
        <CostMetric label="Requested by workloads" projection={requested} timeWindow={timeWindow} currency={report.currency} detail="Priced from resource requests" />
        <CostMetric label="Idle capacity" projection={idle} timeWindow={timeWindow} currency={report.currency} detail="Billed but not requested" tone={idle.hourly > 0 ? "warning" : "good"} />
        <div className="cluster-metric neutral">
          <span>Request coverage</span>
          <strong>{coverage}%</strong>
          <small>of billed capacity is requested</small>
        </div>
      </div>

      <section className="cluster-panel">
        <PanelTitle title="Billed cost by resource type" />
        <CostBars buckets={dimension(report, "subjectKind")} timeWindow={timeWindow} currency={report.currency} />
      </section>

      <section className="cluster-panel">
        <PanelTitle title="Requested cost by namespace" />
        <CostBars buckets={dimension(report, "namespace")} timeWindow={timeWindow} currency={report.currency} />
      </section>

      <section className="cluster-panel">
        <PanelTitle title="Cost by component" />
        <div className="cost-line-group">
          {componentRows(report).map((row) => (
            <div className="cost-line" key={row.key}>
              <div>
                <span className="cost-line-icon">{row.icon}</span>
                <span>{row.label}</span>
                <strong>{formatMoney(pick(row.projection, timeWindow), report.currency)}</strong>
              </div>
              <div className="capacity-track"><span style={{ width: `${row.share}%` }} /></div>
            </div>
          ))}
        </div>
      </section>

      <section className="cluster-panel">
        <PanelTitle title="Largest line items" />
        <div className="cluster-table-wrap">
          <table className="cluster-table">
            <thead><tr><th>Subject</th><th>Type</th><th>Basis</th><th>{windowLabels[timeWindow]}</th><th>Confidence</th></tr></thead>
            <tbody>
              {topItems(report).map((item) => (
                <tr key={`${item.subject.kind}-${item.subject.id || item.subject.name}`}>
                  <td><strong>{item.subject.name}</strong>{item.subject.namespace && <small className="system-namespace-label">{item.subject.namespace}</small>}</td>
                  <td>{item.subject.kind}</td>
                  <td>{item.basis}</td>
                  <td className="cost-cell">{item.confidence === "unknown" ? "Unknown" : formatMoney(pick(item.cost, timeWindow), report.currency)}</td>
                  <td><span className={item.confidence === "exact" ? "status-good" : "status-warning"}>{item.confidence}</span></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <AssumptionsPanel report={report} />
    </div>
  );
}

function CostTrendView({ clusterId, currency }: { clusterId: string; currency: string }) {
  const [days, setDays] = useState(7);
  const [trend, setTrend] = useState<CostTrend | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    setIsLoading(true);
    setError("");
    getCostTrend(clusterId, days, days > 2 ? "day" : "hour")
      .then((result) => { if (!cancelled) setTrend(result); })
      .catch((reason) => { if (!cancelled) setError(reason instanceof Error ? reason.message : String(reason)); })
      .finally(() => { if (!cancelled) setIsLoading(false); });
    return () => { cancelled = true; };
  }, [clusterId, days]);

  if (error) return <div className="cluster-screen-content"><div className="error-message"><AlertCircle size={16} /><span>{error}</span></div></div>;
  if (!trend) return <div className="cluster-screen-content"><section className="cluster-panel"><div className="panel-footnote"><LoaderCircle className="spin" size={15} /> Reading recorded captures...</div></section></div>;

  const coverage = trend.windowHours > 0 ? Math.round((trend.coveredHours / trend.windowHours) * 100) : 0;
  const points = trend.points ?? [];
  const highest = points.reduce((maximum, point) => Math.max(maximum, point.provisionedUSD), 0);

  return (
    <div className="cluster-screen-content">
      <div className="cluster-metrics">
        <div className="cluster-metric neutral"><span>Recorded billed spend</span><strong>{formatMoney(trend.totalProvisionedUSD, currency)}</strong><small>Over the captured periods only</small></div>
        <div className="cluster-metric neutral"><span>Recorded requested spend</span><strong>{formatMoney(trend.totalRequestedUSD, currency)}</strong><small>Priced from resource requests</small></div>
        <div className="cluster-metric warning"><span>Recorded idle</span><strong>{formatMoney(trend.totalIdleUSD, currency)}</strong><small>Billed but not requested</small></div>
        <div className={`cluster-metric ${coverage > 70 ? "good" : "warning"}`}><span>Window coverage</span><strong>{coverage}%</strong><small>{trend.coveredHours.toFixed(1)} of {trend.windowHours.toFixed(0)} hours captured</small></div>
      </div>

      <section className="cluster-panel">
        <div className="cluster-panel-heading">
          <h2>Spend per {trend.bucket}</h2>
          <div className="mode-selector" role="group" aria-label="History window">
            {[1, 7, 30].map((option) => (
              <button key={option} type="button" className={`mode-button ${days === option ? "active" : ""}`} onClick={() => setDays(option)} disabled={isLoading}>
                <span><strong>{option === 1 ? "24h" : `${option}d`}</strong></span>
              </button>
            ))}
          </div>
        </div>
        {points.length === 0 || highest === 0
          ? <div className="panel-footnote"><AlertCircle size={14} /> Nothing recorded in this window yet. Captures are written every time a cost report runs.</div>
          : <div className="namespace-bars">
            {points.map((point) => {
              const covered = point.bucketHours > 0 ? point.coveredHours / point.bucketHours : 0;
              return (
                <div className="namespace-bar" key={point.start as unknown as string}>
                  <div>
                    <span className="namespace-name">
                      {new Date(point.start).toLocaleString(undefined, trend.bucket === "day" ? { month: "short", day: "numeric" } : { hour: "2-digit", minute: "2-digit" })}
                      {covered === 0 && <small className="system-namespace-label">No data captured</small>}
                      {covered > 0 && covered < 0.9 && <small className="system-namespace-label">{Math.round(covered * 100)}% captured</small>}
                    </span>
                    <strong>{covered === 0 ? "-" : formatMoney(point.provisionedUSD, currency)}</strong>
                  </div>
                  <div className="capacity-track"><span style={{ width: `${Math.round((point.provisionedUSD / highest) * 100)}%` }} /></div>
                </div>
              );
            })}
          </div>}
        <div className="panel-footnote">
          <AlertCircle size={14} /> Each capture's rate is integrated over the time it stayed valid. Periods with no capture are shown empty rather than estimated, so totals are what was observed, not a full-month bill.
        </div>
      </section>
    </div>
  );
}

function OptimizationView({ report, recommendations, timeWindow }: { report: CostReport; recommendations: CostRecommendation[]; timeWindow: CostWindow }) {
  const provisioned = totalFor(report, "provisioned");
  const idle = report.idle ?? emptyProjection;
  const idleShare = provisioned.hourly > 0 ? Math.round((idle.hourly / provisioned.hourly) * 100) : 0;
  const identified = recommendations.reduce((total, recommendation) => total + (recommendation.savings?.hourly ?? 0), 0);
  const blockers = recommendations.filter((recommendation) => recommendation.severity === "blocker").length;

  return (
    <div className="cluster-screen-content">
      <div className="cluster-metrics">
        <CostMetric label="Identified savings" projection={projectionOfSavings(recommendations)} timeWindow={timeWindow} currency={report.currency} detail={`${recommendations.length} finding${recommendations.length === 1 ? "" : "s"}`} tone={identified > 0 ? "warning" : "good"} />
        <CostMetric label="Idle capacity" projection={idle} timeWindow={timeWindow} currency={report.currency} detail={`${idleShare}% of the bill is not requested`} tone={idleShare > 40 ? "warning" : "neutral"} />
        <CostMetric label="Billed infrastructure" projection={provisioned} timeWindow={timeWindow} currency={report.currency} detail="Baseline the savings are measured against" />
        <div className={`cluster-metric ${blockers > 0 ? "warning" : "good"}`}>
          <span>Data blockers</span>
          <strong>{blockers}</strong>
          <small>{blockers > 0 ? "Savings below are a lower bound" : "The report is complete"}</small>
        </div>
      </div>

      <section className="cluster-panel">
        <PanelTitle title="Recommendations" />
        {recommendations.length === 0
          ? <div className="panel-footnote"><Lightbulb size={14} /> Nothing to act on in this snapshot.</div>
          : <div className="attention-list">
            {recommendations.map((recommendation) => (
              <RecommendationRow key={recommendation.id} recommendation={recommendation} timeWindow={timeWindow} currency={report.currency} />
            ))}
          </div>}
        <div className="panel-footnote">
          <Lightbulb size={14} /> Savings are projections of the current rate, not measured spend. Applying a change and re-running the report is the only way to confirm one.
        </div>
      </section>

      <AssumptionsPanel report={report} />
    </div>
  );
}

function AssumptionsPanel({ report }: { report: CostReport }) {
  const assumptions = report.assumptions ?? [];
  const warnings = report.warnings ?? [];

  return (
    <section className="cluster-panel">
      <PanelTitle title="Assumptions and gaps" />
      {assumptions.map((assumption) => (
        <div className="panel-footnote" key={`${assumption.key}-${assumption.detail}`}><CircleDollarSign size={14} /> {assumption.detail}</div>
      ))}
      {warnings.map((warning, index) => (
        <div className="panel-footnote" key={`${warning.code}-${warning.subject}-${index}`}><AlertCircle size={14} /> {warning.subject ? `${warning.subject}: ` : ""}{warning.message}</div>
      ))}
      <div className="panel-footnote">
        <AlertCircle size={14} /> Figures are estimates from published list prices. Data transfer, load balancers and NAT gateways are not priced, and discounts are not applied.
      </div>
    </section>
  );
}

function CostMetric({ label, projection, timeWindow, currency, detail, tone = "neutral" }: { label: string; projection: CostProjection; timeWindow: CostWindow; currency: string; detail?: string; tone?: "neutral" | "good" | "warning" }) {
  return (
    <div className={`cluster-metric ${tone}`}>
      <span>{label}</span>
      <strong>{formatMoney(pick(projection, timeWindow), currency)}</strong>
      {detail && <small>{detail}</small>}
    </div>
  );
}

function CostBars({ buckets, timeWindow, currency }: { buckets: Record<string, CostProjection>; timeWindow: CostWindow; currency: string }) {
  const entries = Object.entries(buckets)
    .map(([name, projection]) => ({ name, value: pick(projection, timeWindow) }))
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

function RecommendationRow({ recommendation, timeWindow, currency }: { recommendation: CostRecommendation; timeWindow: CostWindow; currency: string }) {
  const savings = recommendation.savings?.[timeWindow] ?? 0;
  return (
    <div className="cost-line">
      <div>
        <span className="cost-line-icon">{severityIcon(recommendation.severity)}</span>
        <span>
          {recommendation.title}
          <small className="system-namespace-label">{recommendation.rationale}</small>
          <small className="system-namespace-label">{recommendation.action}</small>
        </span>
        <strong>{savings > 0 ? formatMoney(savings, currency) : "No saving claimed"}</strong>
      </div>
    </div>
  );
}

function severityIcon(severity: string) {
  if (severity === "blocker") return <AlertCircle size={15} />;
  if (severity === "high") return <ServerCog size={15} />;
  if (severity === "medium") return <HardDrive size={15} />;
  return <Server size={15} />;
}

function projectionOfSavings(recommendations: CostRecommendation[]): CostProjection {
  return recommendations.reduce((total, recommendation) => ({
    hourly: total.hourly + (recommendation.savings?.hourly ?? 0),
    daily: total.daily + (recommendation.savings?.daily ?? 0),
    monthly: total.monthly + (recommendation.savings?.monthly ?? 0),
    yearly: total.yearly + (recommendation.savings?.yearly ?? 0),
  }), { ...emptyProjection });
}

function componentRows(report: CostReport) {
  const buckets = dimension(report, "component");
  const icons: Record<string, React.ReactNode> = {
    cpu: <Cpu size={15} />,
    memory: <Layers3 size={15} />,
    storage: <HardDrive size={15} />,
    gpu: <ServerCog size={15} />,
    flat: <Server size={15} />,
  };
  const labels: Record<string, string> = {
    cpu: "CPU",
    memory: "Memory",
    storage: "Storage",
    gpu: "GPU",
    flat: "Flat fees",
  };

  const rows = Object.entries(buckets).map(([key, projection]) => ({ key, projection, label: labels[key] ?? key, icon: icons[key] ?? <Server size={15} /> }));
  const highest = rows.reduce((maximum, row) => Math.max(maximum, row.projection.hourly), 0);
  return rows
    .filter((row) => row.projection.hourly > 0)
    .sort((left, right) => right.projection.hourly - left.projection.hourly)
    .map((row) => ({ ...row, share: highest > 0 ? Math.round((row.projection.hourly / highest) * 100) : 0 }));
}

function topItems(report: CostReport): CostLineItem[] {
  return [...(report.items ?? [])]
    .sort((left, right) => right.hourlyUSD - left.hourlyUSD)
    .slice(0, 12);
}

function totalFor(report: CostReport, basis: string): CostProjection {
  return report.totals?.[basis] ?? emptyProjection;
}

function dimension(report: CostReport, name: string): Record<string, CostProjection> {
  return (report.byDimension?.[name] as Record<string, CostProjection>) ?? {};
}

function pick(projection: CostProjection, timeWindow: CostWindow) {
  return projection?.[timeWindow] ?? 0;
}

function formatMoney(value: number, currency = "USD") {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
    minimumFractionDigits: value < 1 ? 4 : 2,
    maximumFractionDigits: 4,
  }).format(value);
}

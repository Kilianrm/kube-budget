import { useState } from "react";
import { AlertCircle, Check, CheckCircle2, ChevronDown, ChevronRight, Clock3, Copy, HardDrive, Layers3, RotateCcw, Server, ShieldAlert, Undo2 } from "lucide-react";
import {
  dismissRecommendation,
  markRecommendationApplied,
  reopenRecommendation,
  restoreRecommendation,
  AppliedRecommendation,
  CostReport,
  RecommendationEvent,
  OptimizationItem,
  OptimizationRecommendation,
  OptimizationResult,
} from "./backend";
import { formatMoney } from "./costFormat";
import { useCostForecast } from "./costHistoryCache";
import { Segmented } from "./Segmented";

const baseColor = "#2a78d6";
const savingColor = "#1baf7a";

const categories = [
  { key: "workloads", label: "Workloads", hint: "Changes to pod requests", icon: <Layers3 size={15} /> },
  { key: "nodes", label: "Nodes", hint: "Node count, machine type and purchase option", icon: <Server size={15} /> },
  { key: "storage", label: "Storage", hint: "Persistent volumes", icon: <HardDrive size={15} /> },
] as const;

type RecommendationTab = "open" | "applied" | "dismissed" | "history";

type Props = {
  report: CostReport;
  clusterId: string;
  optimization: OptimizationResult;
  onChange: (next: OptimizationResult) => void;
};

export function OptimizationsView({ report, clusterId, optimization, onChange }: Props) {
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [tab, setTab] = useState<RecommendationTab>("open");
  const plan = optimization.plan;
  const currency = report.currency;
  const applied = optimization.applied ?? [];
  const history = optimization.history ?? [];
  // A recommendation marked applied waits in Applied until a report no longer
  // detects it; it stays in the plan meanwhile, since the bill has not moved.
  const pendingById = new Map(applied.filter((entry) => entry.pending).map((entry) => [entry.id, entry]));
  const active = plan.recommendations.filter((recommendation) => !recommendation.dismissed && !pendingById.has(recommendation.id));
  const dismissed = plan.recommendations.filter((recommendation) => recommendation.dismissed);
  const savingsShare = plan.current.hourly > 0 ? Math.round((plan.savings.hourly / plan.current.hourly) * 100) : 0;

  const { forecast } = useCostForecast(clusterId, plan.current.hourly, report.generatedAt);
  const budget = forecast?.budget?.monthlyUSD ?? null;

  const act = async (id: string, action: typeof dismissRecommendation) => {
    setBusy(id);
    setError("");
    try {
      onChange(await action(clusterId, id));
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setBusy(null);
    }
  };

  const cardProps = (recommendation: OptimizationRecommendation) => ({
    recommendation,
    currency,
    busy: busy === recommendation.id,
    appliedAt: pendingById.get(recommendation.id)?.at as string | undefined,
    onDismiss: () => act(recommendation.id, dismissRecommendation),
    onRestore: () => act(recommendation.id, restoreRecommendation),
    onApplied: () => act(recommendation.id, markRecommendationApplied),
    onReopen: () => act(recommendation.id, reopenRecommendation),
  });
  const tabs: { value: RecommendationTab; label: string }[] = [
    { value: "open", label: `Open (${active.length})` },
    { value: "applied", label: `Applied (${applied.length})` },
    { value: "dismissed", label: `Dismissed (${dismissed.length})` },
    { value: "history", label: "History" },
  ];

  return (
    <div className="cluster-screen-content">
      {error && <div className="error-message"><AlertCircle size={16} /><span>{error}</span></div>}

      {plan.dataIssues.length > 0 && (
        <section className="cluster-panel fix-first">
          <div className="cluster-panel-heading">
            <h2><ShieldAlert size={16} /> Fix first</h2>
            <small>These make the numbers below incomplete. They carry no saving on their own.</small>
          </div>
          <div className="recommendation-list">
            {plan.dataIssues.map((issue) => <RecommendationCard key={issue.id} {...cardProps(issue)} dataIssue />)}
          </div>
        </section>
      )}

      <div className="cluster-metrics">
        <div className={`cluster-metric ${plan.savings.hourly > 0 ? "good" : "neutral"}`}>
          <span>Achievable savings</span>
          <strong>{formatMoney(plan.savings.monthly, currency)}/mo</strong>
          <small>{savingsShare}% of the bill · steps combined, never double counted</small>
        </div>
        <div className="cluster-metric neutral">
          <span>Bill after the plan</span>
          <strong>{formatMoney(plan.optimized.monthly, currency)}/mo</strong>
          <small>from {formatMoney(plan.current.monthly, currency)}/mo today</small>
        </div>
        <div className={`cluster-metric ${budget === null ? "neutral" : plan.optimized.monthly <= budget ? "good" : "warning"}`}>
          <span>Against your budget</span>
          <strong>{budget === null ? "No budget" : plan.optimized.monthly <= budget ? "Within budget" : `${formatMoney(plan.optimized.monthly - budget, currency)} over`}</strong>
          <small>{budget === null ? "Set one in Budget & forecast" : `${formatMoney(budget, currency)}/mo budget · today ${plan.current.monthly <= budget ? "within" : `${formatMoney(plan.current.monthly - budget, currency)} over`}`}</small>
        </div>
        <div className="cluster-metric neutral">
          <span>Open recommendations</span>
          <strong>{active.length}</strong>
          <small>{plan.dataIssues.length > 0 ? `${plan.dataIssues.length} data issue${plan.dataIssues.length === 1 ? "" : "s"} to fix first` : "No data issues"}{pendingById.size > 0 ? ` · ${pendingById.size} applied, awaiting confirmation` : ""}</small>
        </div>
      </div>

      <section className="cluster-panel">
        <div className="cluster-panel-heading"><h2>From today's bill to the optimized bill</h2><small>Monthly run rate</small></div>
        <SavingsWaterfall optimization={optimization} currency={currency} pending={pendingById} />
      </section>

      <div className="recommendation-tabs">
        <h2>Recommendations</h2>
        <Segmented label="Recommendations to show" value={tab} onChange={setTab} options={tabs} />
      </div>

      {tab === "open" && <>
        {active.length === 0 && (
          <section className="cluster-panel"><div className="panel-footnote standalone"><CheckCircle2 size={14} /> {plan.recommendations.length === 0 && plan.dataIssues.length === 0
            ? "Nothing to act on in this report. The cluster's requests, nodes and volumes match what it runs."
            : "No open recommendations. Applied and dismissed ones are in their tabs."}</div></section>
        )}
        {categories.map((category) => {
          const items = active.filter((recommendation) => recommendation.category === category.key);
          if (items.length === 0) return null;
          const total = items.reduce((sum, recommendation) => sum + recommendation.savings.monthly, 0);
          return (
            <section className="cluster-panel" key={category.key}>
              <div className="cluster-panel-heading">
                <h2>{category.icon} {category.label}</h2>
                <small>{category.hint}{total > 0 ? ` · ${formatMoney(total, currency)}/mo` : ""}</small>
              </div>
              <div className="recommendation-list">
                {items.map((recommendation) => <RecommendationCard key={recommendation.id} {...cardProps(recommendation)} />)}
              </div>
            </section>
          );
        })}
      </>}

      {tab === "applied" && <AppliedView applied={applied} plan={plan} currency={currency} cardProps={cardProps} />}

      {tab === "dismissed" && (
        <section className="cluster-panel">
          <div className="cluster-panel-heading"><h2>Dismissed</h2><small>Left out of the savings total until restored</small></div>
          {dismissed.length === 0
            ? <div className="panel-footnote standalone">Nothing dismissed.</div>
            : <div className="recommendation-list">{dismissed.map((recommendation) => <RecommendationCard key={recommendation.id} {...cardProps(recommendation)} />)}</div>}
        </section>
      )}

      {tab === "history" && <HistoryView history={history} currency={currency} />}

      <details className="cluster-panel assumptions-panel">
        <summary className="cluster-panel-heading"><h2>How the plan is calculated</h2><small>{plan.assumptions.length} assumptions</small></summary>
        {plan.assumptions.map((assumption) => (
          <div className="panel-footnote" key={assumption.key}><AlertCircle size={14} /> {assumption.detail}</div>
        ))}
        <div className="panel-footnote"><AlertCircle size={14} /> Savings are projections at list prices, not measured spend. Applying a change and refreshing the report is the only way to confirm one.</div>
      </details>
    </div>
  );
}

function SavingsWaterfall({ optimization, currency, pending }: { optimization: OptimizationResult; currency: string; pending: Map<string, AppliedRecommendation> }) {
  const plan = optimization.plan;
  if (plan.current.hourly <= 0) {
    return <div className="panel-footnote standalone"><AlertCircle size={14} /> Nothing billed was priced in this report.</div>;
  }
  const scale = (value: number) => `${(value / plan.current.monthly) * 100}%`;
  let remaining = plan.current.monthly;

  return (
    <div className="waterfall" role="table" aria-label="Savings waterfall">
      <div className="waterfall-row total" role="row">
        <span role="cell">Today</span>
        <div className="waterfall-track" role="cell"><i style={{ left: 0, width: "100%", background: baseColor }} /></div>
        <strong role="cell">{formatMoney(plan.current.monthly, currency)}</strong>
      </div>
      {plan.steps.map((step) => {
        const end = remaining;
        remaining -= step.savings.monthly;
        return (
          <div className="waterfall-row" role="row" key={step.id}>
            <span role="cell">{step.title}{pending.has(step.id) && <em className="waterfall-pending" title="Marked applied; the latest report still detects it"> · applied, pending</em>}</span>
            <div className="waterfall-track" role="cell"><i style={{ left: scale(remaining), width: scale(end - remaining), background: savingColor }} /></div>
            <strong role="cell">−{formatMoney(step.savings.monthly, currency)}</strong>
          </div>
        );
      })}
      <div className="waterfall-row total" role="row">
        <span role="cell">After the plan</span>
        <div className="waterfall-track" role="cell"><i style={{ left: 0, width: scale(plan.optimized.monthly), background: baseColor }} /></div>
        <strong role="cell">{formatMoney(plan.optimized.monthly, currency)}</strong>
      </div>
      {plan.steps.length === 0 && <div className="panel-footnote standalone">No step lowers the bill yet.</div>}
    </div>
  );
}

type CardProps = {
  recommendation: OptimizationRecommendation;
  currency: string;
  busy: boolean;
  appliedAt?: string;
  dataIssue?: boolean;
  onDismiss: () => void;
  onRestore: () => void;
  onApplied: () => void;
  onReopen: () => void;
};

function RecommendationCard({ recommendation, currency, busy, appliedAt, dataIssue, onDismiss, onRestore, onApplied, onReopen }: CardProps) {
  const [open, setOpen] = useState(false);
  const items = recommendation.items ?? [];
  const saving = recommendation.savings.monthly;
  const freed = recommendation.freedRequests.monthly;

  return (
    <article className={`recommendation-card ${recommendation.dismissed ? "dismissed" : ""} ${dataIssue ? "data-issue" : ""}`}>
      <header>
        <div>
          <h3>{recommendation.title}</h3>
          <div className="recommendation-chips">
            {!dataIssue && <span className={`chip effort-${recommendation.effort}`}>Effort: {recommendation.effort}</span>}
            {!dataIssue && <span className={`chip risk-${recommendation.risk}`}>Risk: {recommendation.risk}</span>}
            {!dataIssue && recommendation.confidence !== "exact" && <span className="chip">{recommendation.confidence}</span>}
            {appliedAt && <span className="chip applied"><Clock3 size={11} /> Applied {formatDate(appliedAt)} · still detected in the latest report</span>}
          </div>
        </div>
        {!dataIssue && (
          <div className="recommendation-saving">
            <strong>{saving > 0 ? `${formatMoney(saving, currency)}/mo` : "No billed saving yet"}</strong>
            {freed > 0 && <small>frees {formatMoney(freed, currency)}/mo of requests</small>}
          </div>
        )}
      </header>

      <p>{recommendation.rationale}</p>
      <p className="recommendation-action"><b>How to apply:</b> {recommendation.action}</p>

      {items.length > 0 && (
        <>
          <button type="button" className="link-button" onClick={() => setOpen(!open)} aria-expanded={open}>
            {open ? <ChevronDown size={13} /> : <ChevronRight size={13} />} {open ? "Hide" : "Show"} {items.length} {items.length === 1 ? "resource" : "resources"}
          </button>
          {open && <ItemTable items={items} currency={currency} savingsLabel={dataIssue ? null : freed > 0 ? "Requests freed" : "Saving"} />}
        </>
      )}

      {!dataIssue && (
        <footer>
          {recommendation.dismissed
            ? <button type="button" className="secondary-button" disabled={busy} onClick={onRestore}><RotateCcw size={13} /> Restore</button>
            : appliedAt
            ? <button type="button" className="secondary-button" disabled={busy} onClick={onReopen} title="Withdraw the mark: the change was not made after all"><RotateCcw size={13} /> Not applied, reopen</button>
            : <>
              <button type="button" className="secondary-button" disabled={busy} onClick={onApplied} title="Records today's billed rate so the realized change can be checked later"><Check size={13} /> Mark as applied</button>
              <button type="button" className="link-button muted" disabled={busy} onClick={onDismiss}><Undo2 size={13} /> Dismiss</button>
            </>}
        </footer>
      )}
    </article>
  );
}

function ItemTable({ items, currency, savingsLabel }: { items: OptimizationItem[]; currency: string; savingsLabel: string | null }) {
  const showSavings = savingsLabel !== null;
  return (
    <div className="cluster-table-wrap">
      <table className="cluster-table item-table">
        <thead><tr><th>Resource</th><th>Change</th>{showSavings && <th className="numeric-header">{savingsLabel} / mo</th>}<th>Command</th></tr></thead>
        <tbody>
          {items.map((item, index) => (
            <tr key={`${item.subject.kind}-${item.subject.namespace ?? ""}-${item.subject.name}-${index}`}>
              <td><strong>{item.subject.name}</strong>{item.subject.namespace && <small>{item.subject.namespace}</small>}</td>
              <td>{item.change}</td>
              {showSavings && <td className="numeric-cell">{item.savings.monthly > 0 ? formatMoney(item.savings.monthly, currency) : "-"}</td>}
              <td>{item.command ? <CopyCommand command={item.command} /> : null}{item.note && <small className="item-note">{item.note}</small>}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function CopyCommand({ command }: { command: string }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(command);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      setCopied(false);
    }
  };
  return (
    <div className="command-line">
      <code>{command}</code>
      <button type="button" onClick={copy} title="Copy command" aria-label="Copy command">{copied ? <Check size={13} /> : <Copy size={13} />}</button>
    </div>
  );
}

function AppliedView({ applied, plan, currency, cardProps }: { applied: AppliedRecommendation[]; plan: OptimizationResult["plan"]; currency: string; cardProps: (recommendation: OptimizationRecommendation) => CardProps }) {
  const byId = new Map(plan.recommendations.map((recommendation) => [recommendation.id, recommendation]));
  const pending = applied.filter((entry) => entry.pending && byId.has(entry.id));
  const confirmed = applied.filter((entry) => !entry.pending);

  if (applied.length === 0) {
    return <section className="cluster-panel"><div className="panel-footnote standalone"><AlertCircle size={14} /> Nothing marked as applied yet. Mark a recommendation once the change is made, and the next reports confirm it.</div></section>;
  }
  return <>
    {pending.length > 0 && (
      <section className="cluster-panel">
        <div className="cluster-panel-heading"><h2><Clock3 size={15} /> Waiting for confirmation</h2><small>Confirmed when a report no longer detects them</small></div>
        <div className="recommendation-list">
          {pending.map((entry) => <RecommendationCard key={entry.id} {...cardProps(byId.get(entry.id)!)} />)}
        </div>
      </section>
    )}
    {confirmed.length > 0 && (
      <section className="cluster-panel">
        <div className="cluster-panel-heading"><h2><CheckCircle2 size={15} /> Confirmed</h2><small>Billed rate since each change, from the current report</small></div>
        <div className="cluster-table-wrap">
          <table className="cluster-table">
            <thead><tr><th>Recommendation</th><th>Applied</th><th>Confirmed</th><th className="numeric-header">Expected</th><th className="numeric-header">Bill change since</th></tr></thead>
            <tbody>
              {confirmed.map((entry, index) => {
                const realized = entry.realized.monthly;
                return (
                  <tr key={`${entry.id}-${index}`}>
                    <td><strong>{entry.title}</strong></td>
                    <td>{formatDate(entry.at)}</td>
                    <td>{entry.confirmedAt ? formatDate(entry.confirmedAt) : "-"}</td>
                    <td className="numeric-cell">−{formatMoney(entry.expected.monthly, currency)}/mo</td>
                    <td className="numeric-cell">{Math.abs(realized) < 0.005 ? "No change" : `${realized > 0 ? "−" : "+"}${formatMoney(Math.abs(realized), currency)}/mo`}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
        <div className="panel-footnote"><AlertCircle size={14} /> The change since includes everything else that moved the bill in between, so read it as evidence rather than proof.</div>
      </section>
    )}
  </>;
}

const actionLabels: Record<string, { label: string; tone: string }> = {
  applied: { label: "Marked applied", tone: "applied" },
  confirmed: { label: "Confirmed", tone: "confirmed" },
  reopened: { label: "Reopened", tone: "neutral" },
  dismissed: { label: "Dismissed", tone: "neutral" },
  restored: { label: "Restored", tone: "neutral" },
};

function HistoryView({ history, currency }: { history: RecommendationEvent[]; currency: string }) {
  return (
    <section className="cluster-panel">
      <div className="cluster-panel-heading"><h2>History</h2><small>Everything done with this cluster's recommendations, newest first</small></div>
      {history.length === 0
        ? <div className="panel-footnote standalone">No activity yet.</div>
        : <div className="cluster-table-wrap">
          <table className="cluster-table">
            <thead><tr><th>When</th><th>Recommendation</th><th>Action</th><th className="numeric-header">Saving at the time</th></tr></thead>
            <tbody>
              {history.map((event, index) => {
                const action = actionLabels[event.action] ?? { label: event.action, tone: "neutral" };
                return (
                  <tr key={`${event.id}-${String(event.at)}-${index}`}>
                    <td>{new Date(event.at as string).toLocaleString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })}</td>
                    <td><strong>{event.title}</strong></td>
                    <td><span className={`chip history-${action.tone}`}>{action.label}</span></td>
                    <td className="numeric-cell">{event.savings.monthly > 0 ? `${formatMoney(event.savings.monthly, currency)}/mo` : "-"}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>}
    </section>
  );
}

function formatDate(value: unknown) {
  return new Date(value as string).toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
}

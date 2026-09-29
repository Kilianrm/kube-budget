import { useState } from "react";
import { AlertCircle, Check, CheckCircle2, ChevronDown, Circle, ChevronRight, Clock3, Copy, HardDrive, Layers3, RotateCcw, Server, ShieldAlert, Undo2 } from "lucide-react";
import {
  claimRecommendation,
  dismissRecommendation,
  markRecommendationApplied,
  reopenRecommendation,
  restoreRecommendation,
  AppliedRecommendation,
  CostReport,
  DoneResource,
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

// Tabs follow a recommendation's life: what to do, what is being done, what
// is done (with the activity log), and what was set aside.
type RecommendationTab = "open" | "progress" | "done" | "dismissed";

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
  const progress = optimization.progress ?? {};
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
    done: progress[recommendation.id] ?? [],
    pausedReason: recommendation.paused ? plan.paused : undefined,
    onDismiss: () => act(recommendation.id, dismissRecommendation),
    onRestore: () => act(recommendation.id, restoreRecommendation),
    onApplied: () => act(recommendation.id, markRecommendationApplied),
    onReopen: () => act(recommendation.id, reopenRecommendation),
  });
  const inProgress = applied.filter((entry) => entry.pending && plan.recommendations.some((recommendation) => recommendation.id === entry.id));
  const confirmed = applied.filter((entry) => !entry.pending);
  const tabs: { value: RecommendationTab; label: string }[] = [
    { value: "open", label: `Open (${active.length})` },
    { value: "progress", label: `In progress (${inProgress.length})` },
    { value: "done", label: `Done (${confirmed.length})` },
    { value: "dismissed", label: `Dismissed (${dismissed.length})` },
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
          <small>{plan.dataIssues.length > 0 ? `${plan.dataIssues.length} data issue${plan.dataIssues.length === 1 ? "" : "s"} to fix first` : "No data issues"}{pendingById.size > 0 ? ` · ${pendingById.size} being applied` : ""}</small>
        </div>
      </div>

      <section className="cluster-panel">
        <div className="cluster-panel-heading"><h2>From today's bill to the optimized bill</h2><small>Monthly run rate</small></div>
        <SavingsWaterfall optimization={optimization} currency={currency} confirmed={confirmed} />
      </section>

      <div className="recommendation-tabs">
        <h2>Recommendations</h2>
        <Segmented label="Recommendations to show" value={tab} onChange={setTab} options={tabs} />
      </div>

      {tab === "open" && <>
        {active.length === 0 && (
          <section className="cluster-panel"><div className="panel-footnote standalone"><CheckCircle2 size={14} /> {plan.recommendations.length === 0 && plan.dataIssues.length === 0
            ? "Nothing to act on in this report. The cluster's requests, nodes and volumes match what it runs."
            : "No open recommendations. The ones in progress, done and dismissed are in their tabs."}</div></section>
        )}
        {categories.map((category) => {
          const items = active.filter((recommendation) => recommendation.category === category.key);
          const paused = category.key === "nodes" ? plan.paused : "";
          if (items.length === 0 && !paused) return null;
          const total = items.reduce((sum, recommendation) => sum + (recommendation.paused ? 0 : recommendation.savings.monthly), 0);
          return (
            <section className="cluster-panel" key={category.key}>
              <div className="cluster-panel-heading">
                <h2>{category.icon} {category.label}</h2>
                <small>{category.hint}{total > 0 ? ` · ${formatMoney(total, currency)}/mo` : ""}</small>
              </div>
              {paused && <div className="panel-footnote paused-note"><Clock3 size={14} /> Node recommendations are paused: {paused}. One node change runs at a time, since a cluster halfway through one would make them propose conflicting changes. They show their last known values until it finishes.</div>}
              <div className="recommendation-list">
                {items.map((recommendation) => <RecommendationCard key={recommendation.id} {...cardProps(recommendation)} />)}
              </div>
            </section>
          );
        })}
      </>}

      {tab === "progress" && <InProgressView applied={inProgress} plan={plan} cardProps={cardProps} />}
      {tab === "done" && <DoneView confirmed={confirmed} history={history} claimable={optimization.claimable ?? []} currency={currency} busy={busy} onClaim={(id) => act(id, claimRecommendation)} />}

      {tab === "dismissed" && (
        <section className="cluster-panel">
          <div className="cluster-panel-heading"><h2>Dismissed</h2><small>Left out of the savings total until restored</small></div>
          {dismissed.length === 0
            ? <div className="panel-footnote standalone">Nothing dismissed.</div>
            : <div className="recommendation-list">{dismissed.map((recommendation) => <RecommendationCard key={recommendation.id} {...cardProps(recommendation)} />)}</div>}
        </section>
      )}


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

// SavingsWaterfall is rebuilt from every report, so nothing in it moves
// silently: steps keep the plan's order, a change in progress shows what is
// left and what it already saved, a paused step keeps its last value without
// counting it, and whatever changed since the report before is labelled.
function SavingsWaterfall({ optimization, currency, confirmed }: { optimization: OptimizationResult; currency: string; confirmed: AppliedRecommendation[] }) {
  const plan = optimization.plan;
  if (plan.current.hourly <= 0) {
    return <div className="panel-footnote standalone"><AlertCircle size={14} /> Nothing billed was priced in this report.</div>;
  }
  const scale = (value: number) => `${(Math.max(value, 0) / plan.current.monthly) * 100}%`;
  let remaining = plan.current.monthly;
  const gone = plan.gone ?? [];
  // what the bill did since the oldest confirmed change: its baseline is the
  // current bill plus the drop measured since
  const oldest = confirmed.reduce<AppliedRecommendation | null>((first, entry) => !first || new Date(entry.at as string) < new Date(first.at as string) ? entry : first, null);

  return (
    <div className="waterfall" role="table" aria-label="Savings waterfall">
      <div className="waterfall-row total" role="row">
        <span role="cell">Today</span>
        <div className="waterfall-track" role="cell"><i style={{ left: 0, width: "100%", background: baseColor }} /></div>
        <strong role="cell">{formatMoney(plan.current.monthly, currency)}</strong>
      </div>
      {(plan.temporary ?? []).map((cost) => (
        <div className="waterfall-note" key={cost.label}><Clock3 size={12} /> Includes {formatMoney(cost.cost.monthly, currency)}/mo for {cost.label}.</div>
      ))}
      {plan.steps.map((step) => {
        if (step.status === "paused") {
          return (
            <div className="waterfall-row paused" role="row" key={step.id} title={step.note}>
              <span role="cell">{step.title}<em className="waterfall-tag"> · paused</em></span>
              <div className="waterfall-track" role="cell"><i style={{ left: scale(remaining - step.savings.monthly), width: scale(step.savings.monthly) }} /></div>
              <strong role="cell">({formatMoney(step.savings.monthly, currency)})</strong>
            </div>
          );
        }
        const end = remaining;
        remaining -= step.savings.monthly;
        return (
          <div className={`waterfall-row ${step.status === "applying" ? "applying" : ""}`} role="row" key={step.id}>
            <span role="cell">
              {step.title}
              {step.status === "applying" && <em className="waterfall-tag"> · in progress</em>}
              {step.change === "new" && <em className="waterfall-tag change"> · new</em>}
              {step.change === "changed" && <em className="waterfall-tag change"> · was −{formatMoney(step.previous.monthly, currency)}</em>}
              {step.status === "applying" && step.achieved.monthly > 0 && <small className="waterfall-achieved">{formatMoney(step.achieved.monthly, currency)}/mo already saved</small>}
            </span>
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
      {plan.paused && plan.steps.some((step) => step.status === "paused") && (
        <div className="waterfall-note"><Clock3 size={12} /> Paused steps show their last value in brackets and are left out of the total: {plan.paused}.</div>
      )}
      {gone.length > 0 && (
        <div className="waterfall-note">
          <AlertCircle size={12} /> Since the last report: {gone.map((step, index) => (
            <span key={step.id}>{index > 0 ? " · " : ""}{step.title} {step.change === "done" ? "done" : `left the plan (was −${formatMoney(step.previous.monthly, currency)})`}</span>
          ))}
        </div>
      )}
      {oldest && (
        <div className="waterfall-note achieved">
          <CheckCircle2 size={12} /> {confirmed.length} {confirmed.length === 1 ? "change" : "changes"} confirmed since {formatDate(oldest.at)} · bill {formatMoney(oldest.realized.monthly + plan.current.monthly, currency)} → {formatMoney(plan.current.monthly, currency)}/mo
        </div>
      )}
    </div>
  );
}

type CardProps = {
  recommendation: OptimizationRecommendation;
  currency: string;
  busy: boolean;
  appliedAt?: string;
  // Items already done while the recommendation is still detected.
  done?: DoneResource[];
  // Why a paused node recommendation cannot be started yet.
  pausedReason?: string;
  dataIssue?: boolean;
  onDismiss: () => void;
  onRestore: () => void;
  onApplied: () => void;
  onReopen: () => void;
};

function RecommendationCard({ recommendation, currency, busy, appliedAt, done: tracked = [], pausedReason, dataIssue, onDismiss, onRestore, onApplied, onReopen }: CardProps) {
  const [open, setOpen] = useState(false);
  const applying = recommendation.applying ?? false;
  // A recommendation being applied is frozen: its items stay listed, and the
  // ones the cluster already shows done move to the done rows.
  const items = (recommendation.items ?? []).filter((item) => !item.done);
  const trackedKeys = new Set(tracked.map((entry) => `${entry.namespace ?? ""}/${entry.name}`));
  const done: DoneRow[] = [
    ...tracked,
    ...(recommendation.items ?? [])
      .filter((item) => item.done && !trackedKeys.has(`${item.subject.namespace ?? ""}/${item.subject.name}`))
      .map((item) => ({ name: item.subject.name, namespace: item.subject.namespace, at: undefined, savings: item.savings, item })),
  ];
  const milestones = recommendation.milestones ?? [];
  const saving = recommendation.savings.monthly;
  const freed = recommendation.freedRequests.monthly;

  return (
    <article className={`recommendation-card ${recommendation.dismissed ? "dismissed" : ""} ${dataIssue ? "data-issue" : ""} ${pausedReason ? "paused" : ""}`}>
      <header>
        <div>
          <h3>{recommendation.title}</h3>
          <div className="recommendation-chips">
            {!dataIssue && <span className={`chip effort-${recommendation.effort}`}>Effort: {recommendation.effort}</span>}
            {!dataIssue && <span className={`chip risk-${recommendation.risk}`}>Risk: {recommendation.risk}</span>}
            {!dataIssue && recommendation.confidence !== "exact" && <span className="chip">{recommendation.confidence}</span>}
            {pausedReason && <span className="chip" title={`Last known values. It can start when this finishes: ${pausedReason}`}><Clock3 size={11} /> Paused · last known values</span>}
            {appliedAt && <span className="chip applied" title="Frozen as it was when you started; each report ticks off what the cluster shows done"><Clock3 size={11} /> Applying since {formatDateTime(appliedAt)}{applying ? " · frozen" : ""}</span>}
            {done.length > 0 && <span className="chip history-confirmed"><CheckCircle2 size={11} /> {done.length} of {done.length + items.length} done</span>}
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

      {applying && milestones.length > 0 && (
        <ul className="milestones" aria-label="Progress, checked against the cluster">
          {milestones.map((milestone) => (
            <li key={milestone.label} className={milestone.done ? "done" : ""}>
              {milestone.done ? <CheckCircle2 size={13} /> : <Circle size={13} />} {milestone.label}
            </li>
          ))}
        </ul>
      )}

      {items.length + done.length > 0 && (
        <>
          <button type="button" className="link-button" onClick={() => setOpen(!open)} aria-expanded={open}>
            {open ? <ChevronDown size={13} /> : <ChevronRight size={13} />} {open ? "Hide" : "Show"} {items.length + done.length} {items.length + done.length === 1 ? "resource" : "resources"}{done.length > 0 && ` (${done.length} done)`}
          </button>
          {open && <ItemTable items={items} done={done} currency={currency} savingsLabel={dataIssue ? null : freed > 0 ? "Requests freed" : "Saving"} />}
        </>
      )}

      {!dataIssue && (
        <footer>
          {recommendation.dismissed
            ? <button type="button" className="secondary-button" disabled={busy} onClick={onRestore}><RotateCcw size={13} /> Restore</button>
            : appliedAt
            ? <button type="button" className="secondary-button" disabled={busy} onClick={onReopen} title="Unfreeze it: the change is not being made after all"><RotateCcw size={13} /> Stop applying</button>
            : <>
              <button type="button" className="secondary-button" disabled={busy || Boolean(pausedReason)} onClick={onApplied} title={pausedReason ? `One node change at a time: this can start when it finishes (${pausedReason})` : "Freezes this recommendation while you apply it and records today's billed rate. Each report ticks off what the cluster shows done."}><Check size={13} /> Start applying</button>
              <button type="button" className="link-button muted" disabled={busy} onClick={onDismiss}><Undo2 size={13} /> Dismiss</button>
            </>}
        </footer>
      )}
    </article>
  );
}

type DoneRow = Pick<DoneResource, "name" | "namespace" | "at" | "savings" | "item">;

// ItemTable lists what is still to do, then what is already done, each done
// row with the change and command it had when it was last detected.
function ItemTable({ items, done = [], currency, savingsLabel }: { items: OptimizationItem[]; done?: DoneRow[]; currency: string; savingsLabel: string | null }) {
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
              <td className="command-cell">
                {item.check && <CopyCommand command={item.check} />}
                {item.command && <CopyCommand command={item.command} />}
                {(item.steps ?? []).length > 0 && (
                  <ol className="command-steps">
                    {(item.steps ?? []).map((step, index) => <li key={index}><CopyCommand command={step} /></li>)}
                  </ol>
                )}
                {item.note && <small className="item-note">{item.note}</small>}
              </td>
            </tr>
          ))}
          {done.map((entry) => (
            <tr key={`done-${entry.namespace ?? ""}-${entry.name}`} className="done-row">
              <td><strong>{entry.name}</strong>{entry.namespace && <small>{entry.namespace}</small>}<span className="chip history-confirmed"><CheckCircle2 size={11} /> {entry.at ? `Done ${formatDateTime(entry.at)}` : "Done"}</span></td>
              <td>{entry.item?.change ?? "-"}</td>
              {showSavings && <td className="numeric-cell">{entry.savings.monthly > 0 ? formatMoney(entry.savings.monthly, currency) : "-"}</td>}
              <td className="command-cell">{entry.item?.command ? <CopyCommand command={entry.item.command} /> : <small className="item-note">No longer detected</small>}</td>
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

function InProgressView({ applied, plan, cardProps }: { applied: AppliedRecommendation[]; plan: OptimizationResult["plan"]; cardProps: (recommendation: OptimizationRecommendation) => CardProps }) {
  const byId = new Map(plan.recommendations.map((recommendation) => [recommendation.id, recommendation]));
  if (applied.length === 0) {
    return <section className="cluster-panel"><div className="panel-footnote standalone"><AlertCircle size={14} /> Nothing in progress. Start applying a recommendation before you make the change: it stays frozen while you work, and each report ticks off what the cluster shows done.</div></section>;
  }
  return (
    <section className="cluster-panel">
      <div className="cluster-panel-heading"><h2><Clock3 size={15} /> In progress</h2><small>Frozen as they were when you started · done when the cluster shows the whole change</small></div>
      <div className="recommendation-list">
        {applied.map((entry) => <RecommendationCard key={entry.id} {...cardProps(byId.get(entry.id)!)} />)}
      </div>
    </section>
  );
}

// DoneView holds everything past: recommendations that disappeared and may be
// yours, the confirmed changes with what they saved, and the activity log.
function DoneView({ confirmed, history, claimable, currency, busy, onClaim }: { confirmed: AppliedRecommendation[]; history: RecommendationEvent[]; claimable: string[]; currency: string; busy: string | null; onClaim: (id: string) => void }) {
  // the newest "no longer detected" entry of each claimable recommendation
  const gone = claimable
    .map((id) => history.find((event) => event.id === id && event.action === "gone"))
    .filter((event): event is RecommendationEvent => event !== undefined);
  return <>
    {gone.length > 0 && (
      <section className="cluster-panel">
        <div className="cluster-panel-heading"><h2><AlertCircle size={15} /> No longer detected</h2><small>They disappeared without being applied here. Record one only if the change was yours: a missed metrics sample or another change can also make one disappear.</small></div>
        <div className="cluster-table-wrap">
          <table className="cluster-table">
            <thead><tr><th>Recommendation</th><th>Not detected since</th><th className="numeric-header">Saving at the time</th><th /></tr></thead>
            <tbody>
              {gone.map((event) => (
                <tr key={event.id}>
                  <td><strong>{event.title}</strong></td>
                  <td>{formatDateTime(event.at)}</td>
                  <td className="numeric-cell">{event.savings.monthly > 0 ? `${formatMoney(event.savings.monthly, currency)}/mo` : "-"}</td>
                  <td className="numeric-cell">
                    <button type="button" className="secondary-button" disabled={busy === event.id} onClick={() => onClaim(event.id)} title="The change was yours, made outside KubeBudget: record it as applied with the saving it had">
                      <Check size={13} /> Record as applied
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    )}
    <section className="cluster-panel">
      <div className="cluster-panel-heading"><h2><CheckCircle2 size={15} /> Confirmed</h2><small>Billed rate since each change, from the current report. Click a row for its details.</small></div>
      {confirmed.length === 0
        ? <div className="panel-footnote standalone">No change confirmed yet.</div>
        : <>
          <div className="cluster-table-wrap">
            <table className="cluster-table">
              <thead><tr><th>Recommendation</th><th>Applied</th><th>Confirmed</th><th className="numeric-header">Expected</th><th className="numeric-header">Bill change since</th></tr></thead>
              <tbody>
                {confirmed.map((entry, index) => <ConfirmedRow key={`${entry.id}-${index}`} entry={entry} currency={currency} />)}
              </tbody>
            </table>
          </div>
          <div className="panel-footnote"><AlertCircle size={14} /> The change since includes everything else that moved the bill in between, so read it as evidence rather than proof. A change recorded as yours is measured from the last report that still detected it.</div>
        </>}
    </section>
    <ActivityLog history={history} currency={currency} />
  </>;
}

function ConfirmedRow({ entry, currency }: { entry: AppliedRecommendation; currency: string }) {
  const [open, setOpen] = useState(false);
  const realized = entry.realized.monthly;
  const details = entry.details;
  const freed = (details?.freedRequests.monthly ?? 0) > 0;
  // Items tracked one by one are all in done; others are shown as last
  // recommended. A done item recorded without its own snapshot borrows the
  // one in the recommendation's details when it is there.
  const detailItems = new Map((details?.items ?? []).map((item) => [`${item.subject.namespace ?? ""}/${item.subject.name}`, item]));
  const done = (entry.done ?? []).map((item) => item.item ? item : { ...item, item: detailItems.get(`${item.namespace ?? ""}/${item.name}`) });
  const doneKeys = new Set(done.map((item) => `${item.namespace ?? ""}/${item.name}`));
  const remaining = (details?.items ?? []).filter((item) => !doneKeys.has(`${item.subject.namespace ?? ""}/${item.subject.name}`));
  return (
    <>
      <tr className="expandable-row" onClick={() => setOpen(!open)} aria-expanded={open}>
        <td><span className="row-toggle">{open ? <ChevronDown size={13} /> : <ChevronRight size={13} />}<strong>{entry.title}</strong></span></td>
        <td>{formatDateTime(entry.at)}{entry.unmarked && <span className="chip history-neutral outside-chip" title="Recorded as applied after it disappeared: the change was made outside KubeBudget. The time is when a report last detected it.">Outside KubeBudget</span>}</td>
        <td>{entry.confirmedAt ? formatDateTime(entry.confirmedAt) : "-"}</td>
        <td className="numeric-cell">−{formatMoney(entry.expected.monthly, currency)}/mo</td>
        <td className="numeric-cell">{Math.abs(realized) < 0.005 ? "No change" : `${realized > 0 ? "−" : "+"}${formatMoney(Math.abs(realized), currency)}/mo`}</td>
      </tr>
      {open && (
        <tr className="expanded-row">
          <td colSpan={5}>
            {details
              ? <div className="confirmed-details">
                <p>{details.rationale}</p>
                <p className="recommendation-action"><b>How to apply:</b> {details.action}</p>
                {remaining.length + done.length > 0 && <ItemTable items={remaining} done={done} currency={currency} savingsLabel={freed ? "Requests freed" : "Saving"} />}
              </div>
              : <div className="panel-footnote standalone">Details were not kept for changes confirmed before this version.</div>}
          </td>
        </tr>
      )}
    </>
  );
}

const actionLabels: Record<string, { label: string; tone: string }> = {
  applied: { label: "Started applying", tone: "applied" },
  confirmed: { label: "Confirmed", tone: "confirmed" },
  resolved: { label: "Resolved outside KubeBudget", tone: "confirmed" },
  "item-done": { label: "Resource done", tone: "confirmed" },
  gone: { label: "No longer detected", tone: "neutral" },
  returned: { label: "Detected again", tone: "neutral" },
  claimed: { label: "Recorded as applied", tone: "confirmed" },
  reopened: { label: "Stopped applying", tone: "neutral" },
  dismissed: { label: "Dismissed", tone: "neutral" },
  restored: { label: "Restored", tone: "neutral" },
};

// ActivityLog is the audit trail: every action on this cluster's
// recommendations, folded away since the sections above answer the usual
// questions.
function ActivityLog({ history, currency }: { history: RecommendationEvent[]; currency: string }) {
  return (
    <details className="cluster-panel assumptions-panel">
      <summary className="cluster-panel-heading"><h2>Activity log</h2><small>{history.length === 0 ? "No activity yet" : `${history.length} entries, newest first`}</small></summary>
      {history.length > 0 && (
        <div className="cluster-table-wrap">
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
        </div>
      )}
    </details>
  );
}

// formatDateTime is for the moments of a change, where the hour matters as
// much as the day.
function formatDateTime(value: unknown) {
  return new Date(value as string).toLocaleString(undefined, { month: "short", day: "numeric", year: "numeric", hour: "2-digit", minute: "2-digit" });
}

function formatDate(value: unknown) {
  return new Date(value as string).toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
}

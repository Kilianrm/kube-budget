import { AlertCircle, ArrowRight, CheckCircle2, PlusCircle, RefreshCw, Server, TrendingUp } from "lucide-react";
import { SimulationImpact, SimulationResult } from "./backend";
import { formatMoney } from "./costFormat";

type Props = {
  simulation: SimulationResult;
  isRefreshing: boolean;
  onRefresh: () => void;
};

/** What deploying the manifest does to the connected cluster's bill. */
export function ClusterImpact({ simulation, isRefreshing, onRefresh }: Props) {
  const impact = simulation.impact;
  const before = simulation.forecastBefore;
  const after = simulation.forecastAfter;
  const budget = after?.budget ?? before?.budget;

  return (
    <div className="cluster-impact">
      <Verdict impact={impact} />

      <div className="impact-metrics">
        <Change label="Monthly bill" before={impact.billedBefore.monthly} after={impact.billedAfter.monthly} note={impact.billedDelta.monthly > 0 ? `+${formatMoney(impact.billedDelta.monthly)}/mo` : "No change"} />
        <Change label="Idle capacity" before={impact.idleBefore.monthly} after={impact.idleAfter.monthly} note="Capacity paid for but not requested" />
        {before && after
          ? <Change
            label={`Forecast for ${new Date(after.forecast.monthStart as string).toLocaleString(undefined, { month: "long" })}`}
            before={before.forecast.totalUSD}
            after={after.forecast.totalUSD}
            note={budget ? budgetNote(after.budget ?? budget) : "Deployed now, for the rest of the month"}
            warning={Boolean(after.budget && after.budget.state !== "on-track" && before.budget?.state === "on-track")}
          />
          : <div className="impact-metric"><span>Forecast</span><strong>-</strong><small>History could not be read</small></div>}
        <Change label="Plan savings" before={impact.planSavingsBefore.monthly} after={impact.planSavingsAfter.monthly} note="From Optimizations, per month" />
      </div>

      {impact.warnings.length > 0 && impact.warnings.map((warning) => (
        <div className="budget-alert" key={warning}><AlertCircle size={15} /> {warning}</div>
      ))}

      <section className="impact-section">
        <h3><Server size={14} /> Where the {impact.workload.replicas === 1 ? "replica lands" : `${impact.workload.replicas} replicas land`}</h3>
        <div className="placement-list">
          {impact.placements.map((placement) => (
            <div className={`placement ${placement.new ? "new" : ""}`} key={placement.node}>
              {placement.new ? <PlusCircle size={14} /> : <Server size={14} />}
              <span>{placement.node}</span>
              <b>{placement.replicas} {placement.replicas === 1 ? "replica" : "replicas"}</b>
            </div>
          ))}
          {impact.unschedulable > 0 && <div className="placement pending"><AlertCircle size={14} /><span>Pending</span><b>{impact.unschedulable}</b></div>}
        </div>
      </section>

      {impact.planChanges.length > 0 && (
        <section className="impact-section">
          <h3><TrendingUp size={14} /> What it changes in the optimization plan</h3>
          {impact.planChanges.map((change) => (
            <div className="plan-change" key={change.id}>
              <span>{change.title}</span>
              <b>{formatMoney(change.before.monthly)} <ArrowRight size={12} /> {formatMoney(change.after.monthly)}/mo</b>
            </div>
          ))}
        </section>
      )}

      {simulation.peak && <PeakImpact peak={simulation.peak} />}

      <details className="impact-assumptions">
        <summary>How this is simulated</summary>
        <p>Requests are priced at {simulation.estimate.pricing.instanceType} rates, the machine type the cluster mostly runs, the same rates its own workloads get.</p>
        {impact.assumptions.map((assumption) => <p key={assumption.key}>{assumption.detail}</p>)}
      </details>

      <div className="impact-footnote">
        <span>Against the cluster's cost report from {new Date(simulation.reportAt as string).toLocaleTimeString()}.</span>
        <button type="button" className="link-button" onClick={onRefresh} disabled={isRefreshing}>
          <RefreshCw size={12} className={isRefreshing ? "spin" : ""} /> Refresh and simulate again
        </button>
      </div>
    </div>
  );
}

function Verdict({ impact }: { impact: SimulationImpact }) {
  const requested = formatMoney(impact.requested.monthly);
  if (impact.fits) {
    return (
      <div className="impact-verdict fits">
        <CheckCircle2 size={22} />
        <div>
          <strong>Fits in capacity the cluster already pays for</strong>
          <p>It uses {requested}/mo of today's idle capacity, so the bill does not change.</p>
        </div>
        <b>+{formatMoney(0)}<small>/mo billed</small></b>
      </div>
    );
  }
  if (impact.newNodes === 0) {
    return (
      <div className="impact-verdict blocked">
        <AlertCircle size={22} />
        <div>
          <strong>Cannot be scheduled</strong>
          <p>No node the cluster can add is large enough for a replica.</p>
        </div>
        <b>—</b>
      </div>
    );
  }
  return (
    <div className="impact-verdict grows">
      <PlusCircle size={22} />
      <div>
        <strong>Needs {impact.newNodes} more {impact.newNodeType} {impact.newNodes === 1 ? "node" : "nodes"}</strong>
        <p>It requests {requested}/mo, but the cluster pays for whole machines: the bill grows by the nodes it adds.</p>
      </div>
      <b>+{formatMoney(impact.billedDelta.monthly)}<small>/mo billed</small></b>
    </div>
  );
}

function PeakImpact({ peak }: { peak: SimulationImpact }) {
  return (
    <section className="impact-section">
      <h3><TrendingUp size={14} /> At the autoscaler's maximum ({peak.workload.replicas} replicas)</h3>
      <p className="impact-peak">
        {peak.fits
          ? "Still fits in current capacity: no extra nodes at peak."
          : `Needs ${peak.newNodes} more ${peak.newNodeType} ${peak.newNodes === 1 ? "node" : "nodes"} at peak: +${formatMoney(peak.billedDelta.monthly)}/mo while scaled out.`}
      </p>
    </section>
  );
}

function Change({ label, before, after, note, warning }: { label: string; before: number; after: number; note: string; warning?: boolean }) {
  const moved = Math.abs(after - before) >= 0.005;
  return (
    <div className={`impact-metric ${warning ? "warning" : ""}`}>
      <span>{label}</span>
      <strong>{moved ? <>{formatMoney(before)} <ArrowRight size={13} /> {formatMoney(after)}</> : formatMoney(after)}</strong>
      <small>{note}</small>
    </div>
  );
}

function budgetNote(budget: NonNullable<SimulationResult["forecastAfter"]>["budget"]) {
  if (!budget) return "";
  const limit = formatMoney(budget.monthlyUSD);
  switch (budget.state) {
    case "exceeded": return `Budget of ${limit} already exceeded`;
    case "over": return `Over the ${limit} budget`;
    case "at-risk": return `${Math.round(budget.projectedRatio * 100)}% of the ${limit} budget`;
    default: return `Within the ${limit} budget`;
  }
}

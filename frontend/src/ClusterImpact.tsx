import { AlertCircle, ArrowRight, CheckCircle2, PlusCircle, RefreshCw, Server, TrendingUp } from "lucide-react";
import { SimulationImpact, SimulationResult } from "./backend";
import { formatMoney } from "./costFormat";

type Props = {
  simulation: SimulationResult;
  clusterName: string;
};

/** What deploying the manifest does to the connected cluster's bill. */
export function ClusterImpact({ simulation, clusterName }: Props) {
  const impact = simulation.impact;
  const before = simulation.forecastBefore;
  const after = simulation.forecastAfter;
  const budget = after?.budget ?? before?.budget;

  return (
    <div className="cluster-screen-content cluster-impact">
      <Verdict impact={impact} />

      <div className="cluster-metrics impact-metrics">
        <Change label="Monthly bill" before={impact.billedBefore.monthly} after={impact.billedAfter.monthly} note={impact.billedDelta.monthly > 0 ? `+${formatMoney(impact.billedDelta.monthly)}/mo` : "No change"} />
        <Change label="Idle capacity" before={impact.idleBefore.monthly} after={impact.idleAfter.monthly} note="Capacity paid for but not requested" />
        {before && after
          ? <Change
            label={`Forecast for ${new Date(after.forecast.monthStart as string).toLocaleString(undefined, { month: "long" })}`}
            before={before.forecast.totalUSD}
            after={after.forecast.totalUSD}
            note={budget ? budgetNote(after.budget ?? budget) : "Change starts now, for the rest of the month"}
            warning={Boolean(after.budget && after.budget.state !== "on-track" && before.budget?.state === "on-track")}
          />
          : <div className="cluster-metric impact-metric"><span>Forecast</span><strong>-</strong><small>History could not be read</small></div>}
        <Change label="Plan savings" before={impact.planSavingsBefore.monthly} after={impact.planSavingsAfter.monthly} note="From Optimizations, per month" lowerIsWorse />
      </div>

      {impact.warnings.length > 0 && impact.warnings.map((warning) => (
        <div className="budget-alert" key={warning}><AlertCircle size={15} /> {warning}</div>
      ))}

      <WorkloadComparison simulation={simulation} clusterName={clusterName} />

      <section className="cluster-panel">
        <div className="cluster-panel-heading"><h2><Server size={14} /> Replica placement</h2><small>{impact.workload.replicas} replicas</small></div>
        <div className="cluster-table-wrap">
          <table className="cluster-table impact-placement-table">
            <thead><tr><th scope="col">Node</th><th scope="col">Replicas</th><th scope="col">Capacity</th></tr></thead>
            <tbody>
              {impact.placements.map((placement) => <tr key={placement.node}>
                <td>{placement.node}</td><td>{placement.replicas}</td>
                <td><span className={placement.new ? "status-warning" : "status-good"}>{placement.new ? "Additional node" : "Existing node"}</span></td>
              </tr>)}
              {impact.unschedulable > 0 && <tr><td>Pending</td><td className="impact-negative">{impact.unschedulable}</td><td><span className="status-warning">Does not fit modeled nodes</span></td></tr>}
              {impact.placements.length === 0 && impact.unschedulable === 0 && <tr><td colSpan={3}>No replicas to place.</td></tr>}
            </tbody>
          </table>
        </div>
      </section>

      {simulation.peak && <PeakImpact peak={simulation.peak} />}

      <details className="cluster-panel impact-assumptions">
        <summary>Simulation assumptions</summary>
        <p>Requests are priced at {simulation.estimate.pricing.instanceType} rates, the machine type the cluster mostly runs, the same rates its own workloads get.</p>
        {impact.assumptions.map((assumption) => <p key={assumption.key}>{assumption.detail}</p>)}
      </details>

      <div className="impact-footnote">
        <span>Against the cluster's cost report from {new Date(simulation.reportAt as string).toLocaleTimeString()}.</span>
      </div>
    </div>
  );
}

function WorkloadComparison({ simulation, clusterName }: { simulation: SimulationResult; clusterName: string }) {
  const impact = simulation.impact;
  const current = impact.current;
  const proposed = impact.workload;
  const unknown = impact.operation === "unknown";
  const label = unknown ? "Workload status unknown" : current ? "Update existing workload" : "New workload";
  const currentValue = (value: string | number) => unknown ? "Unknown" : current ? value : "Not deployed";
  const rows = [
    { label: "Replicas", current: currentValue(current?.replicas ?? 0), proposed: proposed.replicas, increased: proposed.replicas > (current?.replicas ?? 0) },
    { label: "CPU request / pod", current: currentValue(`${((current?.perReplica.cpuCores ?? 0) * 1000).toLocaleString()}m`), proposed: `${(proposed.perReplica.cpuCores * 1000).toLocaleString()}m`, increased: proposed.perReplica.cpuCores > (current?.perReplica.cpuCores ?? 0) },
    { label: "Memory request / pod", current: currentValue(`${((current?.perReplica.memoryGB ?? 0) * 1024).toLocaleString()} MiB`), proposed: `${(proposed.perReplica.memoryGB * 1024).toLocaleString()} MiB`, increased: proposed.perReplica.memoryGB > (current?.perReplica.memoryGB ?? 0) },
    ...(proposed.perReplica.gpuUnits || current?.perReplica.gpuUnits ? [{ label: "GPU request / pod", current: currentValue(current?.perReplica.gpuUnits ?? 0), proposed: proposed.perReplica.gpuUnits, increased: proposed.perReplica.gpuUnits > (current?.perReplica.gpuUnits ?? 0) }] : []),
    { label: "Attributed cost / month", current: unknown || (current && !impact.currentCostKnown) ? "Unknown" : formatMoney(impact.currentRequested.monthly), proposed: formatMoney(simulation.estimate.monthlyTotal), increased: (!current || impact.currentCostKnown) && simulation.estimate.monthlyTotal - impact.currentRequested.monthly >= 0.005 },
  ];
  const delta = simulation.estimate.monthlyTotal - impact.currentRequested.monthly;

  return <section className="cluster-panel manifest-comparison" aria-label="Manifest change preview">
    <div className="cluster-panel-heading"><h2>{unknown ? <AlertCircle size={15} /> : current ? <RefreshCw size={15} /> : <PlusCircle size={15} />} Workload comparison</h2><small>{label}</small></div>
    <p className="manifest-identity">{proposed.kind} / {proposed.namespace}/{proposed.name}</p>
    <p className="manifest-baseline">{unknown ? "Existence could not be verified" : current ? "Compared with the existing workload" : "No matching workload in the collected report"} in {clusterName}.</p>
    <div className="cluster-table-wrap"><table className="cluster-table manifest-comparison-table">
      <caption className="sr-only">Current and proposed workload requests and attributed cost</caption>
      <thead><tr><th scope="col">Resource</th><th scope="col">Current</th><th scope="col">Proposed</th></tr></thead>
      <tbody>{rows.map((row) => <tr key={row.label}><th scope="row">{row.label}</th><td>{row.current}</td><td className={!unknown && row.increased ? "impact-negative" : undefined}>{row.proposed}</td></tr>)}</tbody>
    </table></div>
    {!unknown && (!current || impact.currentCostKnown) && <p className="manifest-cost-delta">Attributed cost change: <strong className={delta >= 0.005 ? "impact-negative" : undefined}>{delta > 0 ? "+" : ""}{formatMoney(delta)}/month</strong></p>}
    <p className="manifest-baseline">Analysis only; no changes applied. {current ? "Steady-state replacement; temporary rollout overlap excluded. Existing nodes are retained." : unknown ? "Capacity impact assumes additional demand." : "Capacity impact assumes a new workload."}</p>
    <p className="manifest-baseline">Current attribution uses collected node rates; proposed attribution uses the cluster's reference machine rates.</p>
  </section>;
}

function Verdict({ impact }: { impact: SimulationImpact }) {
  const requested = formatMoney(impact.requested.monthly);
  if (impact.fits) {
    return (
      <div className="cluster-overview-status good impact-verdict">
        <div className="cluster-overview-status-icon"><CheckCircle2 size={20} /></div>
        <div>
          <h2>{impact.current ? "Updated workload fits in existing capacity" : "Fits in capacity the cluster already pays for"}</h2>
          <p>{impact.current ? "Replaces the existing workload's requests without additional nodes. Estimated node bill does not change." : `It uses ${requested}/mo of today's idle capacity, so the estimated node bill does not change.`}</p>
        </div>
        <b>+{formatMoney(0)}<small>/mo billed</small></b>
      </div>
    );
  }
  if (impact.newNodes === 0) {
    return (
      <div className="cluster-overview-status warning impact-verdict">
        <div className="cluster-overview-status-icon"><AlertCircle size={20} /></div>
        <div>
          <h2>Does not fit the modeled nodes</h2>
          <p>No modeled node is large enough for a replica.</p>
        </div>
        <b>-</b>
      </div>
    );
  }
  return (
    <div className="cluster-overview-status warning impact-verdict">
      <div className="cluster-overview-status-icon"><PlusCircle size={20} /></div>
      <div>
        <h2>Needs <span className="impact-negative">{impact.newNodes}</span> more {impact.newNodeType} {impact.newNodes === 1 ? "node" : "nodes"}</h2>
        <p>{impact.current ? "After replacing the existing requests, the proposed workload needs additional capacity." : `It requests ${requested}/mo, but the cluster pays for whole machines.`} Estimated node cost grows by the nodes it adds.</p>
      </div>
      <b className={impact.billedDelta.monthly >= 0.005 ? "impact-negative" : undefined}>+{formatMoney(impact.billedDelta.monthly)}<small>/mo billed</small></b>
    </div>
  );
}

function PeakImpact({ peak }: { peak: SimulationImpact }) {
  return (
    <section className="cluster-panel">
      <div className="cluster-panel-heading"><h2><TrendingUp size={14} /> Autoscaler peak</h2><small>{peak.workload.replicas} replicas</small></div>
      <p className="impact-peak">
        {peak.fits
          ? "Still fits in current capacity: no extra nodes at peak."
          : <>Needs <span className={peak.newNodes > 0 ? "impact-negative" : undefined}>{peak.newNodes}</span> more {peak.newNodeType} {peak.newNodes === 1 ? "node" : "nodes"} at peak: <span className={peak.billedDelta.monthly >= 0.005 ? "impact-negative" : undefined}>+{formatMoney(peak.billedDelta.monthly)}/mo</span> while scaled out.</>}
      </p>
    </section>
  );
}

function Change({ label, before, after, note, warning, lowerIsWorse = false }: { label: string; before: number; after: number; note: string; warning?: boolean; lowerIsWorse?: boolean }) {
  const moved = Math.abs(after - before) >= 0.005;
  const unfavorable = moved && (lowerIsWorse ? after < before : after > before);
  return (
    <div className={`cluster-metric impact-metric ${warning ? "warning" : ""}`}>
      <span>{label}</span>
      <strong>{moved && <>{formatMoney(before)} <ArrowRight size={13} /> </>}<span className={unfavorable || warning ? "impact-negative" : undefined}>{formatMoney(after)}</span></strong>
      <small className={warning || (unfavorable && note.startsWith("+")) ? "impact-negative" : undefined}>{note}</small>
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

# Cost Model and Explorer

This document describes the current cluster cost report, history, forecast,
and optimization model. It replaces the original implementation proposal;
the packages and workflows described here are implemented in the repository.

## Cost report

The cost explorer consumes a `costmodel.CostReport` built from a collected
cluster snapshot. Its central distinction is the basis of each cost:

| Basis | Meaning | Source |
|-------|---------|--------|
| `requested` | Cost represented by workload resource requests | Pods and their node placement, or a representative machine rate when placement is unavailable |
| `provisioned` | Cost of the infrastructure and billable resources present | Nodes, available managed control plane, and persistent volumes |

Reports keep totals and dimension breakdowns separate by basis. They also
include an idle projection for billed node capacity not requested by pods and
a shared projection for costs not attributed to a workload, such as the
managed control plane. Allocations reconcile provisioned cost among consumers,
idle capacity, and shared costs.

Observed usage is optional. When metrics-server samples are available for the
required pods, the report includes usage and efficiency summaries priced at
the same rates as requests. A point-in-time usage sample is not historical
utilization and does not change the requested-cost basis.

## Pricing and assumptions

- Nodes are priced individually by machine type, region, and purchase option.
  Not-ready or unschedulable nodes remain billable.
- When pod placement is available, requested costs use the rates of the nodes
  running those pods. Without placement data, the report uses a representative
  machine type for per-unit request rates and reports its assumptions.
- Managed control-plane pricing is included when provider metadata identifies
  one. Volume pricing uses the provider's default storage rate; storage-class
  specific catalog rates are not resolved.
- Spot pricing is identified separately and carries a variability assumption.
- Missing all CPU and memory requests makes a workload's requested cost
  unknown. Partial requests are priced on the values present and reported as
  incomplete data.
- A missing rate creates an unpriced item and warning; it is excluded from
  totals rather than treated as zero.
- These are estimates from bundled public list-price catalogs, not invoice
  reconciliation. Discounts, savings plans, data transfer, public IPs, and
  some infrastructure charges are not modeled.

`core/costmodel.Project` is the shared conversion from hourly USD to daily,
monthly, and yearly run rates. The monthly convention is 730 hours. These
projections are not observed spend.

## History and forecast

The application stores compact cost captures locally while reports are
requested. Captures contain rates and allocation summaries rather than full
line items. History integrates each sample only until the next sample or the
maximum allowed gap; uncovered intervals remain gaps and are not interpolated.
The app must be running to collect new samples.

The history view supports hourly and daily buckets, coverage, namespace and
node-group totals, and comparison against the preceding window. Change drivers
compare average hourly rates so windows with different sample coverage are
meaningfully comparable.

The monthly forecast separates:

- **Recorded:** spend integrated from captured rates.
- **Estimated:** elapsed time without captures, estimated from the recorded
  average rate (or current run rate when there is no recorded coverage).
- **Projected:** the rest of the calendar month at the current billed run rate.

A per-cluster monthly budget classifies this forecast as on track, at risk,
over budget, or exceeded. Forecasts and budget alerts are local to the app;
alerts surface while the app is open.

## Optimization plan

`core/optimize.Build` simulates an ordered combined scenario rather than
adding independent recommendation savings. The plan covers orphan volumes,
unusable nodes, consolidation, request rightsizing, machine-type changes, and
spot candidates when the collected data supports them. Each step claims only
the incremental savings it adds to the simulated plan.

Data issues such as unknown requests, unpriced resources, or missing usage are
shown separately and can prevent later node-level recommendations. Rightsizing
can show freed requested capacity separately from billed savings; billed
savings are claimed only when the simulated plan can remove capacity.

Recommendations include risk, effort, rationale, assumptions, and commands or
ordered steps when the action is sufficiently specific. Commands are advice
for the user to review and run. They are never executed by the application.

The recommendation store tracks open, applying, confirmed, dismissed, and
no-longer-detected states. Starting an application freezes the recommendation
details so progress can be compared against the same proposed change while the
live plan changes. Completion is confirmed from later cluster reports; a
recommendation disappearing by itself is not treated as proof that it was
applied.

## Manifest impact simulation

Manifest mode can simulate a new or updated Deployment against the latest
cluster report. It estimates request cost using the cluster's pricing rates,
places replicas against available node capacity, estimates additional nodes
when needed, rebuilds the report and optimization plan, and compares the result
with the current state. The forecast changes only for the remaining part of
the month.

The placement model is an estimate, not a Kubernetes scheduler dry run. It
does not model tolerations, affinity, topology spread, disruption budgets, or
temporary rollout overlap. When pod placement was not collected, free capacity
is estimated from cluster totals. New nodes use a representative untainted
machine type and include the cluster's DaemonSet overhead.

## Scope boundaries

- The Cost section requires a cluster whose platform and region can be priced.
  Cluster inventory remains available when cost pricing is unsupported.
- AWS, Google Cloud, and Azure list-price catalogs are available; direct
  provider management and node-group command generation are EKS-specific.
- Usage is sampled from metrics-server when available. Historical per-workload
  usage peaks are not stored, so one sample should not be treated as a reliable
  rightsizing history.
- Costs reflect collected resources and available catalog data. Missing
  collection, unsupported resource rates, or namespace-scoped snapshots can
  limit what the report represents; warnings and assumptions should be read
  alongside totals.
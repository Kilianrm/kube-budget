# Cost Explorer ? Core Design

Status: proposal / design phase. No code committed yet.

This document defines the strategy for the **Cost Explorer** in Cluster Mode, and how its core
is shared with **Manifest Mode** (estimations) and the future **Optimizations** module.

---

## 1. The problem with today's model

Today the cost math is spread across layers:

- `core/engine.Estimate` returns a single `Total`, implicitly **hourly**.
- `internal/adapters/wails.ManifestAdapter` multiplies by `24` and `730` to produce daily/monthly.
- `frontend/src/App.tsx#ClusterCostExplorer` re-implements cost math in TypeScript, using only
  EKS node groups and a hardcoded node hourly rate.
- Providers derive per-core/per-GB rates from node prices with a hidden 50/50 split assumption.

Consequence: three different definitions of "cost" exist, none of them reconcilable, and the
Optimizations module would become a fourth. The fix is **one cost kernel, many lenses**.

---

## 2. Central concept: two lenses and the gap between them

Every cluster cost question is answered by one of two views:

| Lens | Question | Source of truth | Direction |
|---|---|---|---|
| **Allocated** | What do my workloads *ask for*? | Pod/container `resources.requests` | bottom-up (demand) |
| **Provisioned** | What is the cloud *billing me*? | Nodes, control plane, disks, LBs | top-down (supply) |

```
Provisioned cost  ???????????????????????????????????????????  (what you pay)
        ?
        ??? Allocated cost (sum of workload requests � rate)   ?? (what you asked for)
        ?        ?
        ?        ??? Used cost (actual utilization)  [future, needs metrics]
        ?
        ??? Idle / slack = Provisioned ? Allocated             ?? (Optimizations input)
```

- **Cost Explorer** = render both lenses + the gap, sliced by dimension and projected over time.
- **Optimizations** = rules that read the gap and emit recommendations with a saving figure.
- **Manifest Mode** = the *allocated* lens applied to a single hypothetical workload.

This is the shared responsibility the three modules need. They must never compute prices
independently ? they all consume the same `CostReport`.

---

## 3. Proposed package layout

```
core/
  pricing/      rate cards, catalogs, RateResolver interface   (exists, extend)
  costmodel/    subjects, line items, projections, report      (new ? the kernel)
  engine/       workload ? line item                           (exists, refactor)
  allocation/   requests ? allocated cost, idle/slack math     (new)
  optimize/     rules over CostReport ? recommendations        (new, later)

internal/application/costexplorer/   snapshot ? CostReport     (new use case)
internal/storage/snapshot/           snapshot persistence      (new, phase 3)
```

Rule: `core/*` stays free of Kubernetes, Wails, AWS SDK and file I/O, exactly as today.

---

## 4. The kernel: `core/costmodel`

```go
// Basis: what the cost was computed from. Never mix bases silently.
type Basis string
const (
    BasisRequested   Basis = "requested"   // pod resource requests
    BasisLimits      Basis = "limits"
    BasisProvisioned Basis = "provisioned" // node/instance hours, flat fees
    BasisUsed        Basis = "used"        // metrics-server / Prometheus (future)
)

type SubjectKind string
const (
    SubjectWorkload     SubjectKind = "workload"
    SubjectNamespace    SubjectKind = "namespace"
    SubjectNode         SubjectKind = "node"
    SubjectNodeGroup    SubjectKind = "nodegroup"
    SubjectControlPlane SubjectKind = "controlplane"
    SubjectVolume       SubjectKind = "volume"
    SubjectAddon        SubjectKind = "addon"
    SubjectCluster      SubjectKind = "cluster"
)

type Subject struct {
    Kind      SubjectKind
    ID        string // stable key (UID when available)
    Name      string
    Namespace string
    ParentID  string // node ? nodegroup, workload ? namespace
    Labels    map[string]string // for arbitrary grouping (team, app, env)
}

type Component string // "cpu" | "memory" | "storage" | "gpu" | "flat"

// Confidence is first-class: the UI must never present a guess as a fact.
type Confidence string
const (
    ConfidenceExact     Confidence = "exact"     // flat published price
    ConfidenceDerived   Confidence = "derived"   // split from a node price
    ConfidenceEstimated Confidence = "estimated" // fallback catalog entry
    ConfidenceUnknown   Confidence = "unknown"   // no rate, cost excluded
)

type LineItem struct {
    Subject     Subject
    Basis       Basis
    Usage       Usage                 // cores, GB, GB, GPU units
    Components  map[Component]float64 // hourly USD per component
    HourlyUSD   float64
    Confidence  Confidence
    Assumptions []Assumption          // human-readable, surfaced in UI
}

// Projection is the ONLY place hourly?daily?monthly happens.
type Projection struct {
    Hourly  float64
    Daily   float64
    Monthly float64
    Yearly  float64
}

const (
    HoursPerDay   = 24.0
    HoursPerMonth = 730.0 // 365*24/12, documented convention
    HoursPerYear  = 8760.0
)

func Project(hourlyUSD float64) Projection

type CostReport struct {
    GeneratedAt time.Time
    Scope       Scope // cluster, provider, region, namespace filter
    Currency    string
    Items       []LineItem
    Totals      map[Basis]Projection
    Idle        Projection // provisioned ? allocated
    ByDimension map[Dimension]map[string]Projection // namespace, node, workload kind, label
    Warnings    []Warning
}
```

Why this shape:

- **Projection lives in core**, so the Wails adapter and the frontend stop doing arithmetic.
- **`Basis` prevents the classic bug** of adding node cost and workload cost into one number.
- **`Confidence` + `Assumptions`** implement the existing "no silent assumptions" principle.
- **`Labels` on `Subject`** gives free grouping by team/app/env later, without schema changes.

---

## 5. Rate resolution

Extract rate lookup behind an interface so hardcoded catalogs can be swapped for live
pricing APIs without touching core:

```go
type RateRequest struct {
    Provider    string
    Region      string
    SKU         string // instance type, disk type, "controlplane", ...
    Purchase    PurchaseOption // on-demand | spot | reserved | savings-plan
}

type RateCard struct {
    CPUUSDPerCoreHour   float64
    MemoryUSDPerGBHour  float64
    StorageUSDPerGBHour float64
    GPUUSDPerUnitHour   float64
    FlatUSDPerHour      float64 // control plane, LB, NAT
    CPUCostShare        float64 // explicit; today implicitly 0.5
    Confidence          Confidence
    Source              string  // "static-catalog@2025-09", "aws-pricing-api"
    Assumptions         []Assumption
}

type RateResolver interface {
    Resolve(RateRequest) (RateCard, error)
}
```

Resolution chain with explicit degradation:
`exact SKU in region ? SKU in default region ? family average ? unknown (excluded + warning)`.

The current 50/50 CPU/memory split becomes `RateCard.CPUCostShare` and is emitted as an
assumption in every derived line item.

---

## 6. What the Cost Explorer must cost

The current view only covers worker nodes. The full provisioned picture:

| Subject | Source available today | Notes |
|---|---|---|
| Worker nodes | `Snapshot.Nodes[].InstanceType` | bill per node-hour, even if NotReady/unschedulable |
| Node groups | `Snapshot.Provider.NodeGroups[]` | capacity type (spot/on-demand) matters |
| Control plane | provider constant | EKS/GKE/AKS flat hourly fee |
| Persistent volumes | `Snapshot.Resources` (PVCs) | rate per storage class, $/GB-month ? /hour |
| Add-ons | `Snapshot.Provider.Addons[]` | mostly $0, some paid |
| Load balancers / NAT | **not collected yet** | needs Service type=LoadBalancer + Ingress collection |
| Egress | not collected | out of scope; declare it |

Allocated side, already available: `Snapshot.Workloads[].Requests`,
`Snapshot.Namespaces[].Requests`, `Snapshot.Workloads[].Containers[]`.

### Edge cases that must be decided in the core, not in the UI

- **DaemonSets** scale with node count ? replicas = number of schedulable nodes, not `spec.replicas`.
- **HPA-backed workloads** ? cost is a *band* (`MinTotal`/`MaxTotal` already exist); the explorer
  should show current + band, not a single number.
- **Jobs / CronJobs** are ephemeral ? cost per run � schedule frequency, not continuous hourly.
- **`MissingRequests: true`** ? the workload's allocated cost is **unknown**, not zero. Counting it
  as zero silently inflates "idle" and corrupts optimization recommendations.
- **Spot capacity** ? different rate and a volatility warning.
- **Multi-container pods / sidecars** ? already handled by the converter; keep per-container detail
  in `LineItem` so the Optimizations module can target a single container.

---

## 7. The time dimension: hourly / daily / monthly

Two distinct things that must not be confused in the UI wording:

### 7.1 Run-rate (available now, from one snapshot)
Extrapolation of the current hourly rate: `daily = hourly � 24`, `monthly = hourly � 730`.
Label it **"run rate" / "projected"**, never "spend". This covers the immediate requirement.

### 7.2 Actual cost over a window (needs persistence)
Requires a history of snapshots and time-weighted integration:

```
cost(window) = ? over consecutive snapshots  hourlyRate(s?) � ?t(s?, s???)
```

Design it now, implement in phase 3:

```go
type SnapshotStore interface {
    Put(ctx context.Context, s cluster.Snapshot) error
    Range(ctx context.Context, clusterID string, from, to time.Time) ([]cluster.Snapshot, error)
}

type CostTimeSeries struct {
    Bucket  Bucket // hour | day | month
    Points  []CostPoint // At, Allocated, Provisioned, Idle
}
```

Storage choice: local SQLite (or newline-delimited JSON) under the user config dir, keyed by
cluster ID + `SchemaVersion` (the snapshot is already versioned at `SchemaVersion: 3`).
Retention: hourly for 7 days, daily rollup for 90 days.

This also upgrades `EstimationHistory` from localStorage to a real backend store, giving both
modules the same persistence layer.

---

## 8. How the three modules share the core

```
                        ????????????????????
                        ?  core/costmodel  ?  Subject, LineItem, Projection, CostReport
                        ????????????????????
                ???????????????????????????????????
                ?                ?                ?
        core/engine        core/allocation    core/optimize
      (one workload)     (cluster-wide roll-up)  (rules over a CostReport)
                ?                ?                ?
        Manifest Mode      Cost Explorer     Optimizations
```

- `core/engine` keeps its current contract but returns a `LineItem` instead of a bare `Estimate`.
- `core/allocation` builds the cluster-wide `CostReport` from a `Snapshot` + `RateResolver`.
- `core/optimize` is a **pure function**: `func Analyze(CostReport) []Recommendation`.

```go
type Recommendation struct {
    ID          string
    Subject     Subject
    Rule        string // "idle-node", "over-request", "missing-requests", "cheaper-instance"
    Severity    Severity
    Current     Projection
    Proposed    Projection
    Savings     Projection
    Confidence  Confidence
    Rationale   string
    Action      string // suggested manifest patch / node group change
}
```

Because `Recommendation.Savings` is a `Projection` built from the same kernel, the Optimizations
page and the Cost Explorer can never disagree on numbers. A recommendation is also
directly replayable through Manifest Mode ("what would this cost after the change?").

### Navigation

The three modules map onto the UI as two workspaces plus one shared section:

| Section | Owns | Data source |
|---|---|---|
| Manifest | single-workload estimates | `manifest.Result` |
| Cluster | inventory, health, capacity | `cluster.Snapshot` |
| **Cost** | Cost explorer + Optimizations | one `CostReport` |

Cost Explorer and Optimizations are two views of the same report, so they share one section, one
fetch and one time-window control. Splitting them would mean fetching and formatting the same
report twice and risking two different numbers on screen.

---

## 9. Implementation order

1. **Kernel** ? DONE. `core/costmodel` (Subject, Basis, LineItem, Projection, CostReport).
   The `�24 / �730` math now exists only in `costmodel.Project`.
2. **Rate resolver** ? DONE. `pricing.RateResolver` (`ResolveNode`, `ResolveResources`,
   `ResolveFlat`) implemented once in `internal/providers/catalog`; aws/azure/gcp are now data-only
   declarations. Control-plane and spot rates added, `CPUCostShare` is explicit and emitted as an
   assumption.
3. **Allocation** ? DONE. `core/allocation` prices a neutral `Cluster` (nodes, workloads,
   volumes, control plane) into a `CostReport` with both lenses and the idle gap. DaemonSet
   replicas follow the node count, and `MissingRequests` produces an unknown-cost item.
4. **Use case + adapter** ? DONE. `internal/application/costexplorer` projects a
   `cluster.Snapshot` into the allocation model; `ClusterAdapter.GetCostReport` exposes it and the
   Wails bindings are regenerated.
5. **UI** ? DONE. The Cost Explorer and Optimizations now live in one top-level **Cost** section
   (`frontend/src/CostSection.tsx`) driven by a single `CostReport` fetch: hourly/daily/monthly
   toggle, billed vs requested vs idle, breakdowns by resource type / namespace / component, and
   an assumptions & gaps panel. The duplicated `awsNodeHourlyRate` table and the `cost` view in
   the cluster sidebar were deleted; the cluster section is inventory and health only.
6. **Optimizations** ? DONE. `core/optimize.Analyze(CostReport) []Recommendation` is a pure
   function with rules for missing requests, unpriced resources, unusable nodes, orphan volumes,
   consolidation and spot candidates. `GetCostReport` returns them next to the report.
7. **Persistence** ? DONE. `internal/storage/costhistory` appends a compact capture (hourly rates
   plus rollups, no line items) to a private NDJSON file under the user config directory on every
   report. `core/costseries` integrates those captures into spend per hour or day and reports
   uncaptured periods as gaps. `GetCostTrend` and the Cost section's History view expose it.

8. **Reconciliation** — DONE. The collector records every pod with its node and owning controller
   (snapshot schema 4), and each pod is priced at the rate of the node it runs on. Breakdowns are
   keyed by basis (`byDimension[basis][dimension]`), so billed and requested cost are never summed.
   Idle is node capacity only, split per component (`idleByComponent`); the control plane is
   `shared` and volumes are charged to their namespace. `allocation[namespace|workload]` rows
   (consumers, then `__idle__` and `__shared__`) always add up to the provisioned total. Ephemeral
   storage requests are not priced: they are the node's own disk.

9. **History in the explorer** — DONE. The separate History view is gone: the explorer's "Spend over
   time" panel shows allocated / idle / shared spend per hour or day, the top namespaces of the
   window, and the change in average billed rate against the previous window of equal length
   (rates, not totals, because coverage differs). History records are schema 2; version 1 idle
   included the control plane and volumes and is ignored.
10. **Usage** — DONE. The collector reads `metrics.k8s.io` pod metrics over REST (no metrics client
   dependency) and degrades to a warning without metrics-server. Usage is attached to a workload
   only when every scheduled pod was sampled, priced at the same node rates as its requests, and
   summarized as `usage` (cluster) and `efficiency` (per allocation row). `optimize` gained a
   `rightsize-requests` rule: flag a resource used below 50% of its request, suggest usage + 50%.
   Usage is a single sample; recording per-workload peaks in the history store is the next step
   before the rule can be trusted for memory.

11. **Budget & forecast** — DONE. A third Cost view. `costseries.BuildForecast` splits the calendar
   month (local time, real month length) into recorded spend, an estimate for uncaptured elapsed
   hours at the average recorded rate, and the rest of the month at the current run rate.
   `EvaluateBudget` places it against a per-cluster monthly budget (`internal/storage/budget`,
   a private JSON file) as on-track / at-risk (≥90%) / over / exceeded, with the date the run
   rate uses it up. `costseries.Drivers` explains a change between two windows by average rate
   per namespace, node group, idle and shared; namespaces + idle + shared add up to the change,
   node groups are the same money seen from the infrastructure side and are shown apart.
   Alerts only surface while the app is open.

12. **Optimization plan** — DONE. `optimize.Build(report, inputs)` replaces the independent rules
   with one simulated cluster (usable nodes as a pool of the dominant machine type, 15% headroom,
   at least two nodes). Steps run in order — orphan volumes, unusable nodes, consolidation,
   rightsizing, node type, spot — and each claims only the saving it adds, so the total is a
   combined scenario shown as a waterfall. Rightsizing reports the requests it frees separately
   from the billed saving (the nodes it lets you remove). Data issues (missing requests, unpriced
   resources, no metrics) are listed apart and hold back node-level steps. Every recommendation
   carries category, effort, risk and per-resource items with exact `kubectl` commands where one
   command is exact. Orphan volumes are Bound claims no running pod mounts; Pending claims are
   not billed. Dismissals and an applied journal (with the billed rate at that moment) live in
   `internal/storage/recommendations`; the adapter caches the last report so acting re-plans
   without collecting the cluster again.

13. **Requests and managed add-ons** — DONE. The collector reports which resources a workload leaves
   unrequested (`missingResources`). Only a workload that declares no CPU and no memory has an
   unknown cost and holds back node-level steps; one that omits a single resource (EKS ships
   `aws-node` and `kube-proxy` without memory requests) is priced on what it declares and listed
   as a non-blocking "incomplete requests" data issue. System namespaces (`kube-*`, `amazon-`,
   `aws-`, `gke-`, `gmp-`, `azure-`) are costed but never rightsized or listed as incomplete.
   Rightsizing never suggests below 25m CPU / 250Mi memory per pod, the VPA defaults.

14. **Cluster impact (what-if)** — DONE. Manifest mode's "Connected cluster" option (previously a
   placeholder that sent no pricing and failed) prices a manifest with the cluster's own rates
   (the most common untainted machine type) and calls `ClusterAdapter.SimulateManifest`.
   `core/whatif.Simulate` places each replica on the untainted node with the most free room,
   using per-node requests now carried on node items, adds nodes of the cluster's usual type
   (already carrying its DaemonSets) only for what fits nowhere, then rebuilds the report and the
   plan with the same kernel and compares: billed, idle, plan savings. The forecast moves only
   from now on (`Forecast.WithRunRateChange`). Both sections read one shared report
   (`CostReportContext`), fetched once per connection. The plan's capacity model now packs whole
   pods first-fit with each node's DaemonSet overhead, so it agrees with the simulation.
15. **Recommendation lifecycle** — DONE. A recommendation is open, applied or dismissed.
   Marking it applied is idempotent: it moves to Applied as *pending* and stays in the plan's
   savings, because the bill has not moved yet. The first report that no longer detects its ID
   confirms it (`recommendations.Store.Confirm`), after which the same ID can come back as a new
   occurrence. "Not applied, reopen" withdraws a pending mark (see 20 for how applying works
   now). A recommendation that disappears without being applied or dismissed is not assumed
   applied: it can vanish for many reasons (a transient cluster state, a missed metrics sample,
   usage near a threshold, another step changing, an unrelated change). The store tracks it as
   `Absent`; once `goneAfter` (2) reports in a row miss it, it is logged as "No longer detected",
   with no saving claimed, and as "Detected again" if it returns, so a flicker never reaches the
   log. The user can claim a logged absence ("Record as applied", `Store.Claim`) within 30 days:
   it is then confirmed at once, flagged `unmarked`, dated to the last report that detected it
   (`State.DetectedAt`), with the rates seen then. Entries written as "resolved" by versions that
   recorded disappearances automatically stay as they were. Items of a recommendation are tracked
   one by one (`SeparateItems`: orphan volumes, unusable nodes, rightsizing; `Progress`) only
   while it is being applied; each keeps its last detected snapshot, so the resource table lists
   what is left and, below it, what is done with the change and command it had. The confirmed
   entry carries every done item with its own date, plus the recommendation as last detected
   (`Details`), so a confirmed row expands to its rationale, instructions and commands. Every
   action (started applying, confirmed, item done, no longer detected, detected again, recorded
   as applied, stopped applying, dismissed, restored) goes to a per-cluster history log of 200
   entries. The tabs follow the life of a recommendation: Open, In progress (frozen cards), Done
   (claimable "no longer detected" ones, the confirmed changes, and the activity log folded
   away), Dismissed. Stores write only when the state actually changed.
16. **Supported clusters** — DONE. Every snapshot carries a `platform`: the cloud read from the
   nodes' `providerID` scheme (`aws`, `gce`, `azure`; `kind`, `k3s`… are not clouds), the most
   common region label, and whether a price catalog covers both (`clusterPlatform`). An EKS
   connection stands in when no node reports one. Without an explicit pricing provider, the cost
   report prices with the detected platform, so an EKS cluster added through a plain kubeconfig
   context is priced too. An unsupported cluster locks the Cost section with the reason and
   disables Manifest's "Connected cluster" pricing; the Cluster section works everywhere.
17. **Node-count command on EKS** — DONE. For clusters connected through EKS, the plan gets
   the cluster name, region and each node group's desired and minimum size (`Inputs.EKS`).
   "Run on N fewer nodes" then names the largest on-demand group and gives the
   `aws eks update-nodegroup-config` command that shrinks it, lowering the minimum too when
   needed, plus a `kubectl get nodes` check. Deleting a node would not save anything: the node
   group replaces it. When the cut would empty that group, or outside EKS, only the per-group
   summary is shown.

18. **EKS without the EKS connection** — DONE. A kubeconfig context that authenticates with
   `aws eks get-token` (what `aws eks update-kubeconfig` writes) names the cluster, region, profile
   and role; the snapshot reads the EKS metadata with that identity, exactly as the EKS
   connection does (`eksTargetFor`). Other authenticators are left alone. Nodes also carry their
   `eks.amazonaws.com/nodegroup` and `capacityType` labels, which now decide the node group and
   purchase option first; matching by instance type is the fallback, since two groups can share
   a type. When the EKS call fails, a warning says so and the labels still group and price the
   nodes, but node-count changes have no command, because the group sizes are unknown.

19. **Machine-type commands on EKS** — DONE. A node group's instance type cannot change in place,
   so "Switch to X nodes" gives ordered `Steps` instead of one command: create a group of the new
   type copied from the old one (launch template, AMI type unless `CUSTOM`, subnets, role, labels
   without `alpha.eksctl.io/` and `eks.amazonaws.com/`, taints) and sized for the steps above; wait
   for it; cordon and drain the old group by its `eks.amazonaws.com/nodegroup` label; delete it.
   Only when a single on-demand group running only the old type holds the pool; otherwise the row
   keeps the summary.
20. **Applying freezes a recommendation** — DONE. A plan recomputed from every report reacts to a
   cluster halfway through a change: creating the new node group makes "Run on 2 fewer nodes"
   appear, cordoning makes "Recover or remove 2 nodes" appear, and the recommendation being applied
   drops out at the first step and was confirmed early. "Start applying" (formerly "Mark as
   applied") now stores the recommendation as it is (`Applied.Details`), and the plan shows that
   frozen copy instead of the live one (`Inputs.Applying`). Its progress is read from the cluster:
   items that are changes of their own are `Done` once the live plan no longer lists them; node
   changes carry a `Target` and are checked by `Milestones` (group size; new group ACTIVE, old nodes
   cordoned, old group deleted; spot share). It is confirmed only when `Complete`. While a node
   recommendation is applied, or an EKS node group is CREATING, UPDATING or DELETING
   (`Inputs.NodeTransition`), the other node steps pause (`Plan.Paused`), rightsizing claims no
   node saving, and paused steps are carried over so none is recorded as resolved. The waterfall
   counts what is left of a frozen saving. Marks made before freezing have no details and follow
   the live recommendation as before.

21. **Spot commands on EKS** — DONE. With one on-demand EKS group, "Run stateless workloads on
   spot" gives ordered steps: create `<group>-spot` with the group's type and up to three priced
   types of the same size (a spot group with one type is often left without capacity); patch
   Deployments and CronJobs to prefer `eks.amazonaws.com/capacityType=SPOT` and StatefulSets to
   require `NotIn SPOT` (safe on nodes without the label); then shrink the on-demand group by the
   nodes moved, keeping one at least. Groups eksctl created (`alpha.eksctl.io/nodegroup-name`)
   get `eksctl create nodegroup --spot` and `eksctl scale nodegroup`, so the new group owns its
   stack, role and launch template: borrowing the old group's, as the machine-type commands do,
   makes `eksctl delete cluster` fail on the old group's role. eksctl takes no taints on the command
   line, so a tainted eksctl group gets no commands. Milestones: spot group active, on-demand group
   shrunk, spot share reached. The catalog gains `m5.large` and `m7i.large`, neither cheaper than
   `m6i.large`, as fallbacks.

22. **A waterfall that explains itself** — DONE. The plan is rebuilt from every report, so it
   moves while changes are made; it now says why instead of letting bars appear and vanish. Steps
   keep the plan's order. A step being applied shows what is left and, apart, what it already
   saved (`Step.Achieved`). A node step paused by a node change keeps its last known value
   (`Inputs.LastKnown`, from the report before), shown in brackets and left out of the total, and
   its card stays in Open with Start applying disabled: one node change runs at a time, since node
   steps resize the same groups and assume one another; workload and storage changes run beside
   it. Today's bill lists what a change in progress adds only until it finishes, such as a new
   group running beside the one it replaces (`Plan.Temporary`). Against the report before
   (`State.Previous`), steps are marked new or changed ("was −$X"), and steps that left the plan are
   listed as done or gone (`Plan.Gone`). A line sums what the bill did since the oldest confirmed
   change. Reports are known by `GeneratedAt`: acting on a recommendation replans the same report,
   which no longer counts as a new one, so two clicks can no longer log a recommendation as gone.

Grouping by team or label is out of scope by decision; allocation stays at namespace and workload.

Steps 1?5 deliver the requested hourly/daily/monthly diagnosis with the data already collected.
Steps 6?7 need no further core redesign.

---

## 10. Non-goals (state them in the UI)

- Not a billing reconciliation tool ? figures are estimates from published list prices.
- No egress/data-transfer cost.
- No enterprise discounts, savings plans or committed-use discounts in phase 1.
- Utilization-based (`BasisUsed`) cost requires metrics-server/Prometheus, which the collector
  does not yet read.

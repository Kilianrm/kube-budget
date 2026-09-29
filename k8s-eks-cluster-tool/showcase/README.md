# KubeBudget showcase cluster

A small EKS cluster built so that every KubeBudget view has something real to
show. The basic cluster in `../` runs a handful of lightly requested pods on two
`t3.medium` nodes, which is already close to the cheapest possible EKS setup,
so there is little for the platform to find. This one is designed around stories.

```
showcase/
├── showcase.sh                  create | status | scenario <name> <on|off> | destroy
├── cluster/eks-showcase.yaml    eksctl config: two node groups, no NAT gateway
├── workloads/                   kustomize base, one file per story
├── scenarios/                   manifests switched on during the demo
└── manifests/                   manifests to simulate in Manifest mode (never applied)
```

## What it creates

| Piece | Why it is there |
|---|---|
| `general` node group, 3 × `m6i.large` on-demand | Memory-rich machines under CPU-heavy requests, so the plan can propose a better machine type |
| `batch-spot` node group, 1 spot node (`c6i.large`, falling back to `c7i.large`, `c5d.large` or `c7a.large`), tainted | A second node group and purchase option: the explorer splits cost by group and prices spot at its own rate. Several types let AWS relaunch the node when one type has no spot capacity |
| `shop/checkout-api` (3 pods, 500m / 1Gi each, near-idle nginx) | The over-requested service: rightsizing flags it and the freed requests let the cluster drop a node |
| `shop/recommendation-engine` (2 pods, CPU and memory held near the request) | The well-sized service: high efficiency, left alone by the plan |
| `shop/orders-db` StatefulSet with a 10 GiB volume | Stateful work: storage charged to its namespace, never proposed for spot, volume never called an orphan |
| `shop/old-backup` 50 GiB claim, written once by a Job that was then cleaned up | The orphan volume: bound and billed, but nothing mounts it any more |
| `batch/queue-worker` on the spot group, `batch/nightly-report` CronJob | Batch work already on spot; Job pods costed only while they run |
| `platform/node-agent` DaemonSet | Scales with the node count and is left out of the spot estimate |
| metrics-server | Usage vs requests, efficiency and rightsizing |

All machine types come from the KubeBudget price catalog for `us-east-1`, so
every node is priced. The two groups use different machine types on purpose:
KubeBudget matches a node to its group by its `eks.amazonaws.com/nodegroup`
label, and falls back to the instance type when the label is missing. Connecting
with the kubeconfig option works too: KubeBudget reads the cluster, region and
AWS profile from the `aws eks get-token` command in the kubeconfig. The
spot fallbacks all cost more than `c6i.large` on demand, so the plan still
proposes `c6i.large` as the better machine type; only the spot node's own price
moves a little with the type AWS launches.

## Cost

About **$0.45 per hour, roughly $10–11 per day** while it runs: the EKS control
plane ($0.10/h), three `m6i.large` ($0.096/h each), one spot compute node, 60 GiB
of gp3 volumes, the node root disks and four public IPv4 addresses. There is no
NAT gateway, which saves about $1 per day. KubeBudget itself reports about
$310/month at list prices; it does not price root disks or public IPs.

Create it the day before the interview and destroy it right after.

## Prerequisites

The same as the basic cluster: `aws`, `eksctl` and `kubectl`, authenticated
against the AWS account (`aws sts get-caller-identity`). See `../commands.md`.

## Lifecycle

```sh
./showcase.sh create            # 15–20 minutes
./showcase.sh status
./showcase.sh destroy           # asks you to type the cluster name
```

`create` is safe to rerun: it reuses an existing cluster, installs
metrics-server only when it is missing, and reapplies the workloads.

`destroy` deletes the volume claims first and waits for their EBS volumes to
go. Deleting only the cluster would leave those volumes behind, still billing.

Every `kubectl` call in the script names this cluster's context explicitly, so
it never touches another cluster that happens to be current.

## Preparing the demo

**The day before (history for Budget & forecast and "What changed?")**

KubeBudget records a capture every time the cost report runs, and each
capture covers up to 90 minutes. "What changed?" compares two windows and
needs at least an hour of captures in each.

1. `./showcase.sh create`, then connect KubeBudget (Amazon EKS,
   `kube-budget-showcase`, `us-east-1`).
2. Open the Cost section and refresh the report roughly every hour for a few
   hours. Each refresh is one capture.
3. In Budget & forecast, set a budget of **$250/month**. The forecast lands
   above it, so the page shows the "over budget" state and the date the budget
   runs out.

**On the day**

Refresh the report once before you start, so the plan uses fresh usage.

## Walkthrough

**1. Cost explorer: where the bill goes**
- The bill bar: workload requests, persistent volumes, idle node capacity and
  the shared control plane. The allocation table adds up to the billed total.
- Idle capacity is the largest part. Open "Node capacity by resource": CPU is
  mostly requested, memory mostly idle. That is the m6i-vs-workload mismatch the
  plan acts on later.
- Group by workload: `checkout-api` has low used/requested; `recommendation-engine`
  sits near 100%.
- Billed infrastructure by node group: `general` vs `batch-spot`, with the spot
  node priced lower.

**2. Optimizations: from today's bill to the optimized bill**
- The waterfall: each step claims only the saving it adds, so nothing is counted twice.
- Storage: delete `old-backup`. Open the resources to show the exact
  `kubectl delete pvc` command and the data-loss note.
- Nodes: run on one fewer node, then a better machine type (`c6i.large`), then
  spot for the stateless share. The node-count step shows the exact
  `aws eks update-nodegroup-config` command for the `general` group.
- Workloads: rightsizing shows the requests it frees separately from the billed
  saving, which only comes from nodes it lets you remove.
- Dismiss a recommendation and watch the later steps recalculate.
- Start applying one, for example "Run on 1 fewer node": it freezes, the other
  node recommendations pause, and after running its command and refreshing, the
  milestone ticks and the change moves to Confirmed. The machine-type change
  shows three milestones ticking one by one as you run its steps.
- Spot: the steps create `general-spot` with eksctl (m6i.large, m5.large,
  m7i.large), patch the stateless workloads to prefer spot and `orders-db` to
  stay off it, then shrink `general`.

**3. Scenario: data you cannot trust**
```sh
./showcase.sh scenario missing-requests on
```
Refresh the report: "Fix first" appears and the node-level steps are held
back, because the cluster's demand is incomplete. Turn it off and refresh to
show the plan coming back.

**4. Scenario: a node nobody can use**
```sh
./showcase.sh scenario cordoned-node on
```
Refresh: the cordoned node is billed but unusable, and the plan proposes
recovering or draining it, with the command.

**5. Budget & forecast: where the month is heading**
- Recorded, estimated and projected spend kept apart, against the $250 budget.

**6. Scenario: what changed?**
```sh
./showcase.sh scenario traffic-spike on
```
Refresh, then open "What changed?". `checkout-api` doubles its requests. The
nodes stay the same, so the bill does not move: the shop namespace grows and
idle capacity shrinks by the same amount. This is a good moment to explain the
difference between consumption and billed cost.

**7. Manifest mode: what would deploying this do to the bill?**

In Manifest mode, choose **Connected cluster** and estimate the two files in
`manifests/`. Nothing is applied to the cluster; KubeBudget simulates it.

- `fits-in-idle.yaml` (3 × 500m / 512Mi): fits in capacity the cluster already
  pays for, so **+$0 billed**. Idle capacity shrinks, and "Run on 1 fewer node"
  leaves the plan because that capacity is now used.
- `needs-new-nodes.yaml` (2 × 1 core / 2Gi): requests only about $53/month, but
  no node has a free core left once its DaemonSets are counted, so the cluster
  needs **two more whole `m6i.large` nodes, about +$140/month**. The budget
  forecast moves with it.

The contrast between the two is the point to make: what a workload costs is
not what it adds to the bill.

Turn every scenario off before the next run:

```sh
./showcase.sh scenario missing-requests off
./showcase.sh scenario cordoned-node off
./showcase.sh scenario traffic-spike off
```

## What the plan should show

A simulation of this cluster through the KubeBudget pipeline, with the EKS
add-ons exactly as AWS ships them, gives roughly **$310 → $132 per month**:

| Step | Saving / month |
|---|---|
| Delete `old-backup` (50 GiB, not mounted) | ~$4 |
| Run on one fewer node | ~$58 |
| Rightsize `checkout-api`, `orders-db` and `node-agent` | ~$58 (one more node) |
| Switch to `c6i.large` nodes | ~$12 |
| Run stateless workloads on spot | ~$46 |

Real numbers move with the usage metrics-server samples. Two things are worth
pointing out while you show it:

- Rightsizing never suggests less than 25m CPU or 250Mi memory per pod, the
  Vertical Pod Autoscaler's defaults, so `checkout-api` goes from 500m / 1Gi to
  25m / 250Mi rather than to its near-zero sample.
- EKS add-ons in `kube-system` are costed but never proposed for rightsizing,
  and `aws-node` / `kube-proxy`, which request CPU but no memory, are priced on
  what they declare instead of blocking the plan.

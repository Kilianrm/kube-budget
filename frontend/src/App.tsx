import {
  AlertCircle,
  Activity,
  ArrowRight,
  Boxes,
  Check,
  ChevronDown,
  CircleDollarSign,
  Cpu,
  Clock,
  Database,
  Cloud,
  Code2,
  FileCode2,
  HardDrive,
  Gauge,
  KeyRound,
  Layers3,
  Lightbulb,
  LoaderCircle,
  Network,
  PlugZap,
  RefreshCw,
  RotateCcw,
  Search,
  ServerCog,
  SlidersHorizontal,
  UploadCloud,
  X,
  Zap,
} from "lucide-react";
import { ChangeEvent, DragEvent, useCallback, useEffect, useRef, useState } from "react";
import { estimateManifest, getClusterSnapshot, ClusterSnapshot, ManifestResult } from "./backend";
import { ClusterProvider, useCluster } from "./ClusterContext";
import { ConnectionModal } from "./ConnectionModal";
import { EstimationHistoryProvider, useEstimationHistory, SavedEstimation } from "./EstimationHistory";
import { EstimationRegistry } from "./HistoryPanel";

const exampleManifest = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: checkout-api
  namespace: production
spec:
  replicas: 3
  selector:
    matchLabels:
      app: checkout-api
  template:
    metadata:
      labels:
        app: checkout-api
    spec:
      containers:
        - name: api
          image: example/checkout-api:1.0
          resources:
            requests:
              cpu: 500m
              memory: 512Mi
              ephemeral-storage: 1Gi
`;type Provider = "aws" | "azure" | "gcp";

const providerCatalog: Record<Provider, Record<string, string[]>> = {
  aws: {
    "us-east-1": ["m6i.large", "m6i.xlarge", "c6i.large", "t3.medium"],
    "eu-west-1": ["m6i.large", "m6i.xlarge", "c6i.large", "t3.medium"],
  },
  gcp: {
    "us-central1": ["e2-standard-2", "e2-standard-4", "n2-standard-2"],
    "europe-west1": ["e2-standard-2", "e2-standard-4", "n2-standard-2"],
  },
  azure: {
    eastus: ["Standard_D2s_v5", "Standard_D4s_v5", "Standard_F2s_v2"],
    westeurope: ["Standard_D2s_v5", "Standard_D4s_v5", "Standard_F2s_v2"],
  },
};

type ResultTab = "overview" | "resources" | "source";
type AppSection = "estimate" | "cluster" | "optimization";
type ClusterView = "overview" | "workloads" | "resources" | "cost" | "namespaces" | "nodes";
type WorkloadFilter = "all" | "incomplete" | "incomplete-application" | "incomplete-system";

function formatMoney(value: number, currency = "USD") {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
    minimumFractionDigits: value < 1 ? 4 : 2,
    maximumFractionDigits: 4,
  }).format(value);
}

type EstimationMode = "manual" | "cluster";
type EstimateView = "estimator" | "registry";

function AppContent() {
  const { isConnected, clusterConnection } = useCluster();
  const { saveEstimation } = useEstimationHistory();
  const [activeSection, setActiveSection] = useState<AppSection>("estimate");
  const [estimateView, setEstimateView] = useState<EstimateView>("estimator");
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [estimationMode, setEstimationMode] = useState<EstimationMode>("manual");
  const [showConnectHint, setShowConnectHint] = useState(true);
  const fileInput = useRef<HTMLInputElement>(null);
  const [fileName, setFileName] = useState("");
  const [manifest, setManifest] = useState("");
  const [provider, setProvider] = useState<Provider>("aws");
  const [region, setRegion] = useState("us-east-1");
  const [instanceType, setInstanceType] = useState("m6i.large");
  const [result, setResult] = useState<ManifestResult | null>(null);
  const [activeTab, setActiveTab] = useState<ResultTab>("overview");
  const [isDragging, setIsDragging] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");

  // Auto-switch to cluster mode when cluster is connected
  useEffect(() => {
    if (isConnected()) {
      setEstimationMode("cluster");
      setShowConnectHint(false);
    }
  }, [isConnected()]);

  function acceptFile(file?: File) {
    if (!file) return;
    if (!file.name.match(/\.(ya?ml|json)$/i)) {
      setError("Choose a YAML, YML, or JSON manifest.");
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      setFileName(file.name);
      setManifest(String(reader.result ?? ""));
      setResult(null);
      setError("");
    };
    reader.onerror = () => setError(`Could not read ${file.name}.`);
    reader.readAsText(file);
  }

  function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    acceptFile(event.target.files?.[0]);
    event.target.value = "";
  }

  function handleDrop(event: DragEvent<HTMLDivElement>) {
    event.preventDefault();
    setIsDragging(false);
    acceptFile(event.dataTransfer.files?.[0]);
  }

  function loadExample() {
    setFileName("checkout-api.yaml");
    setManifest(exampleManifest);
    setResult(null);
    setError("");
  }

  function resetWorkspace() {
    setFileName("");
    setManifest("");
    setResult(null);
    setError("");
    setActiveTab("overview");
  }

  async function runEstimate() {
    if (!manifest.trim()) {
      setError("Add a Kubernetes manifest before estimating its cost.");
      return;
    }

    if (estimationMode === "cluster" && !isConnected()) {
      setError("No cluster connected. Please connect a cluster or switch to manual pricing mode.");
      return;
    }

    setIsLoading(true);
    setError("");
    try {
      const estimate = await estimateManifest({
        documents: [{ name: fileName || "untitled.yaml", content: manifest }],
        provider: estimationMode === "manual" ? provider : undefined,
        region: estimationMode === "manual" ? region : undefined,
        instanceType: estimationMode === "manual" ? instanceType : undefined,
        useClusterData: estimationMode === "cluster" && isConnected() ? true : false,
        clusterInfo: estimationMode === "cluster" && isConnected() && clusterConnection ? clusterConnection : undefined,
      });
      setResult(estimate);
      setActiveTab("overview");

      // Save to history
      saveEstimation({
        name: fileName || "untitled.yaml",
        estimationMode,
        provider: estimationMode === "manual" ? provider : undefined,
        region: estimationMode === "manual" ? region : undefined,
        instanceType: estimationMode === "manual" ? instanceType : undefined,
        clusterName: estimationMode === "cluster" ? clusterConnection?.name : undefined,
        monthlyCost: estimate.monthlyTotal,
        currency: estimate.currency,
        manifest,
        fullResult: estimate,
      });
    } catch (reason) {
      setResult(null);
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setIsLoading(false);
    }
  }

  function handleRestoreEstimation(estimation: SavedEstimation) {
    setFileName(estimation.name);
    setManifest(estimation.manifest);
    setResult(estimation.fullResult);
    setActiveTab("overview");

    if (estimation.estimationMode === "manual") {
      setEstimationMode("manual");
      if (estimation.provider) setProvider(estimation.provider as Provider);
      if (estimation.region) setRegion(estimation.region);
      if (estimation.instanceType) setInstanceType(estimation.instanceType);
    } else {
      setEstimationMode("cluster");
    }
  }

  return (
    <main className="app-shell">
      <header className="topbar">
        <div className="brand">
          <div className={`brand-mark ${activeSection === "cluster" ? "cluster" : activeSection === "optimization" ? "optimization" : ""}`} aria-hidden="true">
            {activeSection === "estimate" ? <CircleDollarSign size={21} /> : activeSection === "cluster" ? <Network size={21} /> : <Lightbulb size={21} />}
          </div>
          <div>
            <strong>KubeBudget</strong>
            <span>{activeSection === "estimate" ? "Manifest workspace" : activeSection === "cluster" ? "Cluster workspace" : "Optimization workspace"}</span>
          </div>
        </div>
        <nav className="primary-nav" aria-label="Main sections">
          <button
            type="button"
            className={activeSection === "estimate" ? "active estimate" : ""}
            onClick={() => { setActiveSection("estimate"); setEstimateView("estimator"); }}
            aria-current={activeSection === "estimate" ? "page" : undefined}
          >
            <CircleDollarSign size={16} /> Manifest
          </button>
          <button
            type="button"
            className={activeSection === "cluster" ? "active cluster" : ""}
            onClick={() => setActiveSection("cluster")}
            aria-current={activeSection === "cluster" ? "page" : undefined}
          >
            <Network size={16} /> Cluster
          </button>
          <button
            type="button"
            className={activeSection === "optimization" ? "active optimization" : ""}
            onClick={() => setActiveSection("optimization")}
            aria-current={activeSection === "optimization" ? "page" : undefined}
          >
            <Lightbulb size={16} /> Optimization
          </button>
        </nav>
        <button
          type="button"
          className={`cluster-status-button ${isConnected() ? "connected" : "disconnected"}`}
          onClick={() => setIsModalOpen(true)}
          title={isConnected() ? `Connected to ${clusterConnection?.name}` : "Click to connect a cluster"}
        >
          <Zap size={16} />
          <span>{isConnected() ? clusterConnection?.name || "Connected" : "Not connected"}</span>
        </button>
      </header>

      <ConnectionModal isOpen={isModalOpen} onClose={() => setIsModalOpen(false)} />

      <div className="app-body">
        {activeSection === "estimate" && <aside className="app-sidebar" aria-label="Estimator navigation">
          <span className="sidebar-label">WORKSPACE</span>
          <button type="button" className={estimateView === "estimator" ? "active" : ""} onClick={() => setEstimateView("estimator")}><Gauge size={17} /><span>Estimator</span></button>
          <button type="button" className={estimateView === "registry" ? "active" : ""} onClick={() => setEstimateView("registry")}><Clock size={17} /><span>Saved estimates</span></button>
        </aside>}
        <div className={`app-main ${activeSection === "cluster" ? "cluster-app-main" : ""}`}>
      {activeSection === "cluster" ? <ClusterConnection /> : activeSection === "optimization" ? <OptimizationPanel /> : estimateView === "registry" ? <EstimationRegistry /> : <section className={`workspace ${result ? "has-result" : "manifest-stage"}`}>
        <aside className="input-pane">
          <div className="pane-heading">
            <div><span className="step-label">01 / INPUT</span><h1>Manifest</h1></div>
            {(manifest || result) && (
              <button className="icon-button" type="button" onClick={resetWorkspace} title="Reset workspace" aria-label="Reset workspace">
                <RotateCcw size={17} />
              </button>
            )}
          </div>

          <div
            className={`drop-zone ${isDragging ? "is-dragging" : ""} ${manifest ? "has-file" : ""}`}
            onDragEnter={() => setIsDragging(true)}
            onDragLeave={() => setIsDragging(false)}
            onDragOver={(event) => event.preventDefault()}
            onDrop={handleDrop}
          >
            <input ref={fileInput} type="file" accept=".yaml,.yml,.json" onChange={handleFileChange} hidden />
            <button className="drop-file-button" type="button" onClick={() => fileInput.current?.click()}>
              <span className="drop-icon">{manifest ? <Check size={19} /> : <UploadCloud size={21} />}</span>
              <span className="drop-copy">
                <strong>{fileName || "Drop one manifest here"}</strong>
                <span>{manifest ? "Ready to estimate" : "YAML or JSON, up to one workload"}</span>
              </span>
            </button>
            {!manifest && <button type="button" className="text-button" onClick={(event) => { event.stopPropagation(); loadExample(); }}>Use example</button>}
          </div>

          <label className="field-label" htmlFor="manifest-source">
            <span>Source</span><span>{manifest.split("\n").length} lines</span>
          </label>
          <textarea
            id="manifest-source"
            className="manifest-editor"
            value={manifest}
            onChange={(event) => { setManifest(event.target.value); setResult(null); setError(""); }}
            placeholder="Paste a Deployment manifest here..."
            spellCheck={false}
          />

          <div className="estimation-mode-section">
            <div className="mode-header">
              <span className="mode-label">Estimation source</span>
              {!isConnected() && (
                <button
                  type="button"
                  className="quick-connect-button"
                  onClick={() => setIsModalOpen(true)}
                  title="Connect a cluster to enable better estimates"
                >
                  <PlugZap size={14} />
                  Connect cluster
                </button>
              )}
            </div>
            <div className="mode-selector" role="group" aria-label="Estimation mode">
              <button
                type="button"
                className={`mode-button ${estimationMode === "manual" ? "active" : ""}`}
                onClick={() => {
                  setEstimationMode("manual");
                  setResult(null);
                }}
                title="Use manual pricing assumptions"
              >
                <Cloud size={16} />
                <span>
                  <strong>Manual Pricing</strong>
                  <small>Set custom provider & instance</small>
                </span>
              </button>
              <button
                type="button"
                className={`mode-button ${estimationMode === "cluster" ? "active" : ""} ${!isConnected() ? "disabled" : ""}`}
                onClick={() => {
                  if (isConnected()) {
                    setEstimationMode("cluster");
                    setResult(null);
                  }
                }}
                disabled={!isConnected()}
                title={isConnected() ? "Use connected cluster data for better accuracy" : "Connect a cluster first"}
              >
                <Network size={16} />
                <span>
                  <strong>From Cluster</strong>
                  <small>{isConnected() ? clusterConnection?.name : "Connect a cluster"}</small>
                </span>
                <span className={`value-badge ${isConnected() ? "connected" : "disconnected"}`}>
                  {isConnected() ? "More accurate" : "Connect to activate"}
                </span>
              </button>
            </div>
          </div>

          {!isConnected() && showConnectHint && (
            <div className="onboarding-hint">
              <div className="hint-content">
                <Lightbulb size={16} />
                <div>
                  <strong>Pro tip: Connect a cluster</strong>
                  <p>Get more accurate cost estimates by connecting your Kubernetes cluster. We'll automatically use live workload data.</p>
                </div>
                <button
                  type="button"
                  className="hint-close-button"
                  onClick={() => setShowConnectHint(false)}
                  aria-label="Dismiss hint"
                >
                  <X size={16} />
                </button>
              </div>
            </div>
          )}

          {estimationMode === "manual" && <div className="pricing-section">
            <div className="section-title"><Cloud size={16} /><span>Pricing assumptions</span></div>
            <div className="control-grid">
              <label className="select-field">
                <span>Provider</span>
                <div className="select-wrap">
                  <select value={provider} onChange={(event) => {
                    const nextProvider = event.target.value as Provider;
                    const nextRegion = nextProvider === "aws"
                      ? "us-east-1"
                      : nextProvider === "gcp" ? "us-central1" : "eastus";
                    setProvider(nextProvider);
                    setRegion(nextRegion);
                    setInstanceType(providerCatalog[nextProvider][nextRegion][0]);
                    setResult(null);
                  }}>
                    <option value="aws">AWS</option>
                    <option value="azure">Azure</option>
                    <option value="gcp">GCP</option>
                  </select><ChevronDown size={15} />
                </div>
              </label>
              <label className="select-field">
                <span>Region</span>
                <div className="select-wrap">
                  <select value={region} onChange={(event) => { setRegion(event.target.value); setInstanceType(providerCatalog[provider][event.target.value][0]); setResult(null); }}>
                    {Object.keys(providerCatalog[provider]).map((value) => <option key={value}>{value}</option>)}
                  </select><ChevronDown size={15} />
                </div>
              </label>
              <label className="select-field full-width">
                <span>Worker instance</span>
                <div className="select-wrap">
                  <select value={instanceType} onChange={(event) => { setInstanceType(event.target.value); setResult(null); }}>
                    {(providerCatalog[provider][region] ?? []).map((value) => <option key={value}>{value}</option>)}
                  </select><ChevronDown size={15} />
                </div>
              </label>
            </div>
          </div>
          }

          {error && <div className="error-message" role="alert"><AlertCircle size={17} /><span>{error}</span></div>}

          <button className="estimate-button" type="button" disabled={isLoading || !manifest.trim()} onClick={runEstimate}>
            {isLoading ? <LoaderCircle className="spin" size={18} /> : <CircleDollarSign size={18} />}
            {isLoading ? "Estimating..." : "Estimate manifest"}
            {!isLoading && <ArrowRight size={18} />}
          </button>
        </aside>

        <section className="result-pane">
          <div className="pane-heading result-heading">
            <div><span className="step-label">02 / ESTIMATE</span><h2>{result ? result.workload.name : "Cost analysis"}</h2></div>
            {result && <div className="result-heading-actions"><button type="button" className="new-estimate-button" onClick={resetWorkspace}><RotateCcw size={14} /> New estimate</button><div className="valid-badge"><Check size={14} /> Valid deployment</div></div>}
          </div>

          {!result ? (
            <div className="empty-state">
              <div className="empty-visual" aria-hidden="true">
                <div className="visual-node visual-main"><CircleDollarSign size={27} /></div>
                <div className="visual-node visual-one"><FileCode2 size={18} /></div>
                <div className="visual-node visual-two"><ServerCog size={18} /></div>
                <div className="visual-node visual-three"><Boxes size={18} /></div>
                <span className="connector connector-one" /><span className="connector connector-two" /><span className="connector connector-three" />
              </div>
              <h3>Ready for a workload</h3>
              <p>Add a Deployment manifest to calculate requested CPU, memory, storage, and GPU costs.</p>
              <div className="empty-capabilities">
                <span><Check size={14} /> Per-container breakdown</span>
                <span><Check size={14} /> Hourly to monthly totals</span>
                <span><Check size={14} /> AWS node pricing</span>
              </div>
            </div>
          ) : (
            <div className="result-content">
              <nav className="tabs" aria-label="Estimate views">
                {(["overview", "resources", "source"] as ResultTab[]).map((tab) => (
                  <button key={tab} type="button" className={activeTab === tab ? "active" : ""} onClick={() => setActiveTab(tab)}>
                    {tab === "overview" && <Gauge size={15} />}
                    {tab === "resources" && <Layers3 size={15} />}
                    {tab === "source" && <Code2 size={15} />}
                    {tab[0].toUpperCase() + tab.slice(1)}
                  </button>
                ))}
              </nav>

              {activeTab === "overview" && <Overview result={result} />}
              {activeTab === "resources" && <Resources result={result} />}
              {activeTab === "source" && <pre className="source-preview"><code>{manifest}</code></pre>}
            </div>
          )}
        </section>
      </section>}
        </div>
      </div>
    </main>
  );
}

function ClusterLockedState({ onConnectClick }: { onConnectClick: () => void }) {
  return (
    <section className="cluster-workspace cluster-locked">
      <div className="cluster-locked-content">
        <div className="locked-visual" aria-hidden="true">
          <div className="locked-icon"><AlertCircle size={48} /></div>
        </div>
        <h2>Cluster management locked</h2>
        <p>Connect to a Kubernetes cluster to access cluster management features and view live workload data.</p>
        <button className="primary-button" type="button" onClick={onConnectClick}>
          <PlugZap size={18} />
          <span>Connect a cluster</span>
          <ArrowRight size={18} />
        </button>
      </div>
    </section>
  );
}

function ClusterConnection() {
  const { isConnected, clusterConnection } = useCluster();
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [activeView, setActiveView] = useState<ClusterView>("overview");
  const [workloadFilter, setWorkloadFilter] = useState<WorkloadFilter>("all");
  const [snapshot, setSnapshot] = useState<ClusterSnapshot | null>(null);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [snapshotError, setSnapshotError] = useState("");
  const manualRefreshEnabled = clusterConnection?.refreshIntervalMs === null;

  const refreshSnapshot = useCallback(async () => {
    if (!clusterConnection?.context) {
      setSnapshotError("The connected cluster does not have a kubeconfig context.");
      return;
    }
    setIsRefreshing(true);
    setSnapshotError("");
    try {
      const nextSnapshot = await getClusterSnapshot(
        clusterConnection.kubeconfigPath ?? "",
        clusterConnection.context,
        clusterConnection.namespace,
      );
      setSnapshot(nextSnapshot);
    } catch (reason) {
      setSnapshotError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setIsRefreshing(false);
    }
  }, [clusterConnection?.context, clusterConnection?.kubeconfigPath, clusterConnection?.namespace]);

  useEffect(() => {
    if (isConnected()) void refreshSnapshot();
  }, [isConnected, refreshSnapshot]);

  useEffect(() => {
    const intervalMs = clusterConnection?.refreshIntervalMs;
    if (!intervalMs) return;

    const intervalId = window.setInterval(() => {
      if (document.visibilityState === "visible" && !isRefreshing) {
        void refreshSnapshot();
      }
    }, intervalMs);

    return () => window.clearInterval(intervalId);
  }, [clusterConnection?.refreshIntervalMs, isRefreshing, refreshSnapshot]);

  if (!isConnected()) {
    return (
      <>
        <ClusterLockedState onConnectClick={() => setIsModalOpen(true)} />
        <ConnectionModal isOpen={isModalOpen} onClose={() => setIsModalOpen(false)} />
      </>
    );
  }

  return (
    <>
      <aside className="cluster-sidebar" aria-label="Cluster management options">
        <span className="sidebar-label">CLUSTER</span>
        <ClusterNavButton view="overview" activeView={activeView} onSelect={setActiveView} icon={<Activity size={16} />} label="Overview" />
        <ClusterNavButton view="workloads" activeView={activeView} onSelect={setActiveView} icon={<Boxes size={16} />} label="Workloads" />
        <ClusterNavButton view="resources" activeView={activeView} onSelect={setActiveView} icon={<Database size={16} />} label="Resources" />
        <ClusterNavButton view="namespaces" activeView={activeView} onSelect={setActiveView} icon={<Layers3 size={16} />} label="Namespaces" />
        <ClusterNavButton view="nodes" activeView={activeView} onSelect={setActiveView} icon={<ServerCog size={16} />} label="Nodes" />
        <ClusterNavButton view="cost" activeView={activeView} onSelect={setActiveView} icon={<CircleDollarSign size={16} />} label="Cost explorer" />
        <div className="cluster-sidebar-spacer" />
      </aside>
      <section className="cluster-management-view">
        <ClusterScreenHeader view={activeView} snapshot={snapshot} isRefreshing={isRefreshing} onRefresh={refreshSnapshot} manualRefreshEnabled={manualRefreshEnabled} />
        {snapshotError && <div className="error-message"><AlertCircle size={16} /><span>{snapshotError}</span></div>}
        {!snapshot && isRefreshing && <div className="cluster-screen-content"><section className="cluster-panel"><div className="panel-footnote"><LoaderCircle className="spin" size={15} /> Collecting live cluster resources...</div></section></div>}
        {snapshot && snapshot.warnings.length > 0 && <div className="cluster-screen-content"><div className="error-message"><AlertCircle size={16} /><span>{snapshot.warnings.map((warning) => warning.message).join(" ")}</span></div></div>}
        {snapshot && activeView === "overview" && <ClusterOverview snapshot={snapshot} onNavigate={setActiveView} onShowWorkloads={(filter) => { setWorkloadFilter(filter); setActiveView("workloads"); }} />}
        {snapshot && activeView === "workloads" && <ClusterWorkloads snapshot={snapshot} filter={workloadFilter} onFilterChange={setWorkloadFilter} onRefresh={refreshSnapshot} isRefreshing={isRefreshing} manualRefreshEnabled={manualRefreshEnabled} />}
        {snapshot && activeView === "resources" && <ClusterResources snapshot={snapshot} />}
        {snapshot && activeView === "cost" && <ClusterCostExplorer snapshot={snapshot} />}
        {snapshot && activeView === "namespaces" && <ClusterNamespaces snapshot={snapshot} />}
        {snapshot && activeView === "nodes" && <ClusterNodes snapshot={snapshot} />}
      </section>
      <ConnectionModal isOpen={isModalOpen} onClose={() => setIsModalOpen(false)} />
    </>
  );
}

function ClusterNavButton({ view, activeView, onSelect, icon, label }: { view: ClusterView; activeView: ClusterView; onSelect: (view: ClusterView) => void; icon: React.ReactNode; label: string }) {
  return <button type="button" className={activeView === view ? "active" : ""} onClick={() => onSelect(view)}>{icon}<span>{label}</span></button>;
}

const clusterViewTitles: Record<ClusterView, { title: string; description: string }> = {
  overview: { title: "Cluster overview", description: "A high-level view of health, capacity, and requested cost." },
  workloads: { title: "Workloads", description: "Inspect the workloads that make up this cluster's requested capacity." },
  resources: { title: "Resources", description: "Inspect batch, networking, storage, configuration, and policy resources." },
  cost: { title: "Cost explorer", description: "Understand where requested monthly cost is concentrated." },
  namespaces: { title: "Namespaces", description: "Compare workload count and requested cost across namespaces." },
  nodes: { title: "Nodes", description: "Review node readiness and the capacity available to workloads." },
};

function ClusterScreenHeader({ view, snapshot, isRefreshing, onRefresh, manualRefreshEnabled }: { view: ClusterView; snapshot: ClusterSnapshot | null; isRefreshing: boolean; onRefresh: () => void; manualRefreshEnabled: boolean }) {
  const details = clusterViewTitles[view];
  return <div className="management-header">
    <div><span className="step-label">CLUSTER / {view.toUpperCase()}</span><h1>{details.title}</h1><p>{details.description}</p></div>
    <div className="management-actions">
      {manualRefreshEnabled && <button type="button" className="refresh-button" onClick={onRefresh} disabled={isRefreshing} title="Refresh cluster snapshot" aria-label="Refresh cluster snapshot"><RefreshCw className={isRefreshing ? "spin" : ""} size={15} /></button>}
      <span className="cluster-data-badge"><span /> {snapshot ? `Live / ${new Date(snapshot.collectedAt).toLocaleTimeString()}` : "Waiting for data"}</span>
    </div>
  </div>;
}

function DemoMetric({ label, value, detail, icon, tone = "neutral", onClick }: { label: string; value: string; detail?: string; icon?: React.ReactNode; tone?: "neutral" | "good" | "warning"; onClick?: () => void }) {
  const className = `cluster-metric ${detail ? "" : "compact "}${icon ? "with-icon " : ""}${onClick ? "interactive " : ""}${tone}`;
  const content = <><span>{label}</span><strong>{value}</strong>{detail && <small>{detail}</small>}{icon && <span className="cluster-metric-icon" aria-hidden="true">{icon}</span>}</>;
  return onClick
    ? <button type="button" className={className} onClick={onClick} aria-label={`View ${label.toLowerCase()}`}>{content}</button>
    : <div className={className}>{content}</div>;
}

function ClusterOverview({ snapshot, onNavigate, onShowWorkloads }: { snapshot: ClusterSnapshot; onNavigate: (view: ClusterView) => void; onShowWorkloads: (filter: WorkloadFilter) => void }) {
  const { summary } = snapshot;
  const usableNodes = snapshot.nodes.filter((node) => node.ready && node.schedulable);
  const unavailableNodes = snapshot.nodes.filter((node) => !node.ready || !node.schedulable);
  const usableRequests = usableNodes.reduce((total, node) => addResourceValues(total, node.requests), emptyResourceValues());
  const usableAllocatable = usableNodes.reduce((total, node) => addResourceValues(total, node.allocatable), emptyResourceValues());
  const unavailableCapacity = unavailableNodes.reduce((total, node) => addResourceValues(total, node.allocatable), emptyResourceValues());
  const unhealthyWorkloads = snapshot.workloads.filter((workload) => workload.readyReplicas < workload.desiredReplicas).length;
  const incompleteApplicationWorkloads = snapshot.workloads.filter((workload) => workload.missingRequests && !isSystemNamespace(workload.namespace)).length;
  const incompleteSystemWorkloads = snapshot.workloads.filter((workload) => workload.missingRequests && isSystemNamespace(workload.namespace)).length;
  const cordonedNodes = snapshot.nodes.filter((node) => node.ready && !node.schedulable).length;
  const health = summary.nodeCount > 0 && usableNodes.length === summary.nodeCount ? "Healthy" : "Attention";
  return <div className="cluster-screen-content">
    <div className="cluster-metrics"><DemoMetric label="Cluster health" value={health} detail={`${usableNodes.length} of ${summary.nodeCount} nodes available`} icon={<Activity size={25} />} tone={health === "Healthy" ? "good" : "warning"} onClick={() => onNavigate("nodes")} /><DemoMetric label="Workloads" value={String(summary.workloadCount)} icon={<Boxes size={25} />} onClick={() => onNavigate("workloads")} /><DemoMetric label="Namespaces" value={String(summary.namespaceCount)} icon={<Layers3 size={25} />} onClick={() => onNavigate("namespaces")} /><DemoMetric label="Other resources" value={String(summary.resourceCount)} icon={<Database size={25} />} onClick={() => onNavigate("resources")} /></div>
    <div className="cluster-grid-two">
      <section className="cluster-panel"><PanelHeading title="Requests on usable nodes" action="View nodes" onAction={() => onNavigate("nodes")} /><div className="capacity-list"><CapacityRow label="CPU requests" value={resourcePercent(usableRequests.cpuMilli, usableAllocatable.cpuMilli)} color="blue" /><CapacityRow label="Memory requests" value={resourcePercent(usableRequests.memoryBytes, usableAllocatable.memoryBytes)} color="green" /><CapacityRow label="Ephemeral storage requests" value={resourcePercent(usableRequests.storageBytes, usableAllocatable.storageBytes)} color="orange" /></div><div className="panel-footnote"><Activity size={14} /> Ready, schedulable nodes only. Requested resources are not observed utilization.</div></section>
      <section className="cluster-panel"><PanelHeading title="Operational status" /><div className="attention-list"><AttentionRow icon={incompleteApplicationWorkloads > 0 ? <AlertCircle size={16} /> : <Check size={16} />} title={incompleteApplicationWorkloads > 0 ? `${incompleteApplicationWorkloads} application workloads have incomplete requests` : "Application workload requests are complete"} detail={incompleteApplicationWorkloads > 0 ? "CPU or memory request is missing" : "CPU and memory requests are configured"} tone={incompleteApplicationWorkloads > 0 ? "warning" : "good"} onClick={() => onShowWorkloads("incomplete-application")} /><AttentionRow icon={incompleteSystemWorkloads > 0 ? <AlertCircle size={16} /> : <Check size={16} />} title={incompleteSystemWorkloads > 0 ? `${incompleteSystemWorkloads} system workloads have incomplete requests` : "System workload requests are complete"} detail={incompleteSystemWorkloads > 0 ? "Review missing CPU or memory requests in platform-managed namespaces" : "CPU and memory requests are configured"} tone={incompleteSystemWorkloads > 0 ? "warning" : "good"} onClick={() => onShowWorkloads("incomplete-system")} /><AttentionRow icon={unavailableNodes.length > 0 ? <AlertCircle size={16} /> : <Check size={16} />} title={unavailableNodes.length > 0 ? `${unavailableNodes.length} nodes are unavailable for scheduling` : "All nodes are available for scheduling"} detail={unavailableNodes.length > 0 ? `${formatCPU(unavailableCapacity.cpuMilli)} CPU and ${formatBytes(unavailableCapacity.memoryBytes)} memory unavailable${cordonedNodes > 0 ? `; ${cordonedNodes} cordoned` : ""}` : "No NotReady or cordoned nodes"} tone={unavailableNodes.length > 0 ? "warning" : "good"} /><AttentionRow icon={unhealthyWorkloads > 0 ? <AlertCircle size={16} /> : <Check size={16} />} title={unhealthyWorkloads > 0 ? `${unhealthyWorkloads} workloads are not fully ready` : "All discovered workloads are ready"} detail="Compared with desired replicas" tone={unhealthyWorkloads > 0 ? "warning" : "good"} /></div></section>
    </div>
    <section className="cluster-panel"><PanelHeading title="Top CPU-requesting namespaces" action="View namespaces" onAction={() => onNavigate("namespaces")} /><NamespaceBars snapshot={snapshot} /></section>
  </div>;
}

function CapacityRow({ label, value, color }: { label: string; value: string; color: string }) {
  return <div className="capacity-row"><div><span>{label}</span><strong>{value}</strong></div><div className="capacity-track"><span className={color} style={{ width: value }} /></div></div>;
}

function AttentionRow({ icon, title, detail, tone, onClick }: { icon: React.ReactNode; title: string; detail: string; tone: string; onClick?: () => void }) {
  const content = <><span className="attention-icon">{icon}</span><div><strong>{title}</strong><small>{detail}</small></div></>;
  return onClick
    ? <button type="button" className={`attention-row interactive ${tone}`} onClick={onClick}>{content}</button>
    : <div className={`attention-row ${tone}`}>{content}</div>;
}

function PanelHeading({ title, action, onAction }: { title: string; action?: string; onAction?: () => void }) {
  return <div className="cluster-panel-heading"><h2>{title}</h2>{action && <button type="button" onClick={onAction}>{action}<ArrowRight size={14} /></button>}</div>;
}

function NamespaceBars({ snapshot }: { snapshot: ClusterSnapshot }) {
  const namespaces = [...snapshot.namespaces].sort((left, right) => right.requests.cpuMilli - left.requests.cpuMilli).slice(0, 3);
  const totalCPU = snapshot.namespaces.reduce((total, namespace) => total + namespace.requests.cpuMilli, 0);
  return <div className="namespace-bars">{namespaces.map((namespace) => {
    const percentage = totalCPU > 0 ? Math.round(namespace.requests.cpuMilli / totalCPU * 100) : 0;
    return <div className="namespace-bar" key={namespace.name}><div><span className="namespace-name">{namespace.name}{isSystemNamespace(namespace.name) && <small className="system-namespace-label">System</small>}</span><strong>{formatCPU(namespace.requests.cpuMilli)}<small>{percentage}% of total</small></strong></div><div className="capacity-track"><span style={{ width: `${percentage}%` }} /></div></div>;
  })}<div className="panel-footnote standalone">Showing the top {Math.min(3, snapshot.namespaces.length)} of {snapshot.namespaces.length} namespaces by requested CPU.</div></div>;
}

function ClusterWorkloads({ snapshot, filter, onFilterChange, onRefresh, isRefreshing, manualRefreshEnabled }: { snapshot: ClusterSnapshot; filter: WorkloadFilter; onFilterChange: (filter: WorkloadFilter) => void; onRefresh: () => void; isRefreshing: boolean; manualRefreshEnabled: boolean }) {
  const workloads = snapshot.workloads.filter((workload) => {
    if (filter === "incomplete") return workload.missingRequests;
    if (filter === "incomplete-application") return workload.missingRequests && !isSystemNamespace(workload.namespace);
    if (filter === "incomplete-system") return workload.missingRequests && isSystemNamespace(workload.namespace);
    return true;
  });
  return <div className="cluster-screen-content"><div className="table-toolbar"><div className="fake-search"><Search size={15} /><span>{workloads.length} of {snapshot.workloads.length} workloads</span></div><div className="select-wrap workload-filter"><select value={filter} onChange={(event) => onFilterChange(event.target.value as WorkloadFilter)} aria-label="Filter workloads"><option value="all">All workloads</option><option value="incomplete">All incomplete requests</option><option value="incomplete-application">Incomplete application workloads</option><option value="incomplete-system">Incomplete system workloads</option></select><ChevronDown size={14} /></div>{manualRefreshEnabled && <button type="button" className="refresh-button" onClick={onRefresh} disabled={isRefreshing} title="Refresh workloads"><RefreshCw className={isRefreshing ? "spin" : ""} size={15} /></button>}</div><section className="cluster-panel table-panel"><div className="table-summary"><span>{workloads.length} workloads shown</span><span>Declared resource requests</span></div><div className="cluster-table-wrap"><table className="cluster-table"><thead><tr><th>Workload</th><th>Kind</th><th>Namespace</th><th>Ready</th><th>Requests</th></tr></thead><tbody>{workloads.map((workload) => <tr key={workload.uid || `${workload.kind}/${workload.namespace}/${workload.name}`}><td><strong>{workload.name}</strong><small>{workload.missingRequests ? "CPU or memory request missing" : `${workload.containers.length} containers collected`}</small></td><td>{workload.kind}</td><td><span className="namespace-tag">{workload.namespace}</span></td><td><span className={workload.readyReplicas >= workload.desiredReplicas ? "status-good" : "status-warning"}>{workload.readyReplicas >= workload.desiredReplicas ? <Check size={13} /> : <AlertCircle size={13} />} {workload.readyReplicas} / {workload.desiredReplicas}</span></td><td className="cost-cell">{formatCPU(workload.requests.cpuMilli)} / {formatBytes(workload.requests.memoryBytes)}</td></tr>)}</tbody></table></div></section></div>;
}

function isSystemNamespace(namespace: string) {
  return namespace === "kube-system" || namespace === "kube-public" || namespace === "kube-node-lease" || namespace === "local-path-storage";
}

function ClusterResources({ snapshot }: { snapshot: ClusterSnapshot }) {
  const categoryCounts = snapshot.resources.reduce<Record<string, number>>((counts, resource) => {
    counts[resource.category] = (counts[resource.category] ?? 0) + 1;
    return counts;
  }, {});
  const failedBatch = snapshot.resources.filter((resource) => resource.category === "Batch" && resource.status === "Failed").length;
  return <div className="cluster-screen-content">
    <div className="cluster-metrics">
      <DemoMetric label="Resources" value={String(snapshot.resources.length)} detail="Supporting objects in scope" />
      <DemoMetric label="Batch" value={String(categoryCounts.Batch ?? 0)} detail="Jobs and CronJobs" tone={failedBatch > 0 ? "warning" : "good"} />
      <DemoMetric label="Configuration" value={String(categoryCounts.Configuration ?? 0)} detail="ConfigMaps and Secrets" />
      <DemoMetric label="Infrastructure" value={String((categoryCounts.Networking ?? 0) + (categoryCounts.Storage ?? 0) + (categoryCounts.Policy ?? 0))} detail="Network, storage, and policy" />
    </div>
    <section className="cluster-panel table-panel">
      <div className="table-summary"><span>{snapshot.resources.length} supporting resources discovered</span><span>Secret values are never collected</span></div>
      <div className="cluster-table-wrap"><table className="cluster-table"><thead><tr><th>Resource</th><th>Kind</th><th>Category</th><th>Namespace</th><th>Status</th><th>Details</th></tr></thead><tbody>
        {snapshot.resources.map((resource) => <tr key={resource.uid || `${resource.kind}/${resource.namespace}/${resource.name}`}>
          <td><strong>{resource.name}</strong><small>{resource.requests.cpuMilli > 0 || resource.requests.memoryBytes > 0 ? `${formatCPU(resource.requests.cpuMilli)} · ${formatBytes(resource.requests.memoryBytes)}` : "Metadata only"}</small></td>
          <td>{resource.kind}</td>
          <td><span className="namespace-tag">{resource.category}</span></td>
          <td>{resource.namespace || "Cluster"}</td>
          <td><span className={resource.status === "Failed" ? "status-warning" : "status-good"}>{resource.status === "Failed" ? <AlertCircle size={13} /> : <Check size={13} />} {resource.status}</span></td>
          <td><small>{formatResourceAttributes(resource.attributes)}</small></td>
        </tr>)}
      </tbody></table></div>
    </section>
  </div>;
}

function ClusterCostExplorer({ snapshot }: { snapshot: ClusterSnapshot }) {
  return <div className="cluster-screen-content"><div className="cluster-metrics"><DemoMetric label="Requested monthly cost" value="Unavailable" detail="Pricing is not configured" /><DemoMetric label="CPU requested" value={formatCPU(snapshot.summary.requests.cpuMilli)} detail={`Across ${snapshot.summary.workloadCount} workloads`} /><DemoMetric label="Memory requested" value={formatBytes(snapshot.summary.requests.memoryBytes)} detail="Declared workload requests" /><DemoMetric label="Incomplete requests" value={String(snapshot.summary.missingRequestWorkloads)} detail="Require review before pricing" tone={snapshot.summary.missingRequestWorkloads > 0 ? "warning" : "good"} /></div><section className="cluster-panel"><PanelHeading title="Pricing not configured" /><div className="panel-footnote"><CircleDollarSign size={14} /> Live resources are collected. A pricing selection must be added before requested costs can be calculated.</div></section><section className="cluster-panel"><PanelHeading title="CPU requests by namespace" /><NamespaceBars snapshot={snapshot} /></section></div>;
}

function CostLine({ icon, label, value, width }: { icon: React.ReactNode; label: string; value: string; width: string }) {
  return <div className="cost-line"><div><span className="cost-line-icon">{icon}</span><span>{label}</span><strong>{value}</strong></div><div className="capacity-track"><span style={{ width }} /></div></div>;
}

function ClusterNamespaces({ snapshot }: { snapshot: ClusterSnapshot }) {
  return <div className="cluster-screen-content"><section className="cluster-panel table-panel"><div className="table-summary"><span>{snapshot.namespaces.length} namespaces in scope</span><span>Live workload inventory</span></div><div className="cluster-table-wrap"><table className="cluster-table"><thead><tr><th>Namespace</th><th>Workloads</th><th>Pods</th><th>CPU requests</th><th>Memory requests</th></tr></thead><tbody>{snapshot.namespaces.map((namespace) => <tr key={namespace.name}><td><strong>{namespace.name}</strong><small>{namespace.missingRequests > 0 ? `${namespace.missingRequests} incomplete workloads` : "Resource requests available"}</small></td><td>{namespace.workloadCount}</td><td>{namespace.podCount}</td><td>{formatCPU(namespace.requests.cpuMilli)}</td><td className="cost-cell">{formatBytes(namespace.requests.memoryBytes)}</td></tr>)}</tbody></table></div></section></div>;
}

function ClusterNodes({ snapshot }: { snapshot: ClusterSnapshot }) {
  const instanceTypes = new Set(snapshot.nodes.map((node) => node.instanceType).filter(Boolean));
  const usableNodes = snapshot.nodes.filter((node) => node.ready && node.schedulable);
  const usableRequests = usableNodes.reduce((total, node) => addResourceValues(total, node.requests), emptyResourceValues());
  const usableAllocatable = usableNodes.reduce((total, node) => addResourceValues(total, node.allocatable), emptyResourceValues());
  return <div className="cluster-screen-content"><div className="cluster-metrics"><DemoMetric label="Usable nodes" value={`${usableNodes.length} / ${snapshot.summary.nodeCount}`} detail={usableNodes.length === snapshot.summary.nodeCount ? "All nodes ready and schedulable" : "NotReady or cordoned nodes excluded"} tone={usableNodes.length === snapshot.summary.nodeCount ? "good" : "warning"} /><DemoMetric label="Usable CPU" value={formatCPU(usableAllocatable.cpuMilli)} detail={`${resourcePercent(usableRequests.cpuMilli, usableAllocatable.cpuMilli)} requested`} /><DemoMetric label="Usable memory" value={formatBytes(usableAllocatable.memoryBytes)} detail={`${resourcePercent(usableRequests.memoryBytes, usableAllocatable.memoryBytes)} requested`} /><DemoMetric label="Instance types" value={String(instanceTypes.size)} detail={Array.from(instanceTypes).slice(0, 2).join(", ") || "Not reported"} /></div><section className="cluster-panel table-panel"><div className="table-summary"><span>Node capacity</span><span>Read-only inventory</span></div><div className="cluster-table-wrap"><table className="cluster-table"><thead><tr><th>Node</th><th>Status</th><th>Scheduling</th><th>Role</th><th>CPU</th><th>Memory</th></tr></thead><tbody>{snapshot.nodes.map((node) => <tr key={node.uid || node.name}><td><strong>{node.name}</strong><small>{node.zone || node.instanceType || "Zone not reported"}</small></td><td><span className={node.ready ? "status-good" : "status-warning"}>{node.ready ? <Check size={13} /> : <AlertCircle size={13} />} {node.ready ? "Ready" : "Not ready"}</span></td><td><span className={node.schedulable ? "status-good" : "status-warning"}>{node.schedulable ? <Check size={13} /> : <AlertCircle size={13} />} {node.schedulable ? "Schedulable" : "Cordoned"}</span></td><td>{node.role}</td><td>{resourcePercent(node.requests.cpuMilli, node.allocatable.cpuMilli)} requested</td><td>{resourcePercent(node.requests.memoryBytes, node.allocatable.memoryBytes)} requested</td></tr>)}</tbody></table></div></section></div>;
}

function emptyResourceValues() {
  return { cpuMilli: 0, memoryBytes: 0, storageBytes: 0, gpuUnits: 0 };
}

function addResourceValues(left: ReturnType<typeof emptyResourceValues>, right: ReturnType<typeof emptyResourceValues>) {
  return { cpuMilli: left.cpuMilli + right.cpuMilli, memoryBytes: left.memoryBytes + right.memoryBytes, storageBytes: left.storageBytes + right.storageBytes, gpuUnits: left.gpuUnits + right.gpuUnits };
}

function resourcePercent(requested: number, allocatable: number) {
  return allocatable > 0 ? `${Math.round(requested / allocatable * 100)}%` : "N/A";
}

function formatCPU(cpuMilli: number) {
  return cpuMilli >= 1000 ? `${(cpuMilli / 1000).toFixed(cpuMilli % 1000 === 0 ? 0 : 1)} cores` : `${cpuMilli}m`;
}

function formatBytes(bytes: number) {
  if (bytes === 0) return "0 GiB";
  const gibibytes = bytes / (1024 ** 3);
  return gibibytes >= 1 ? `${gibibytes.toFixed(gibibytes >= 10 ? 0 : 1)} GiB` : `${Math.round(bytes / (1024 ** 2))} MiB`;
}

function formatResourceAttributes(attributes: Record<string, string>) {
  const entries = Object.entries(attributes ?? {});
  return entries.length > 0 ? entries.map(([name, value]) => `${name}: ${value}`).join(" · ") : "No additional details";
}

function OptimizationPanel() {
  return (
    <section className="optimization-workspace">
      <div className="optimization-content">
        <div className="optimization-visual" aria-hidden="true">
          <div className="optimization-icon"><Lightbulb size={48} /></div>
        </div>
        <h2>Optimization features</h2>
        <p>Discover recommendations to optimize your Kubernetes workloads and reduce costs.</p>
        <div className="empty-state">
          <div className="empty-visual" aria-hidden="true">
            <div className="visual-node visual-main"><Lightbulb size={27} /></div>
            <div className="visual-node visual-one"><Gauge size={18} /></div>
            <div className="visual-node visual-two"><Layers3 size={18} /></div>
            <div className="visual-node visual-three"><ServerCog size={18} /></div>
            <span className="connector connector-one" /><span className="connector connector-two" /><span className="connector connector-three" />
          </div>
          <h3>Optimization recommendations coming soon</h3>
          <p>Get actionable insights to optimize your Kubernetes clusters for better performance and reduced costs.</p>
          <div className="empty-capabilities">
            <span><Check size={14} /> Resource optimization</span>
            <span><Check size={14} /> Cost reduction tips</span>
            <span><Check size={14} /> Performance insights</span>
          </div>
        </div>
      </div>
    </section>
  );
}

function Overview({ result }: { result: ManifestResult }) {
  return (
    <div className="overview-view">
      <section className="total-band">
        <div><span>Estimated monthly cost</span><strong>{formatMoney(result.monthlyTotal, result.currency)}</strong><small>30-day projection from resource requests</small></div>
        <div className="cost-pulse"><CircleDollarSign size={28} /></div>
      </section>

      <section className="metric-row">
        <div><span>Hourly</span><strong>{formatMoney(result.hourlyTotal, result.currency)}</strong></div>
        <div><span>Daily</span><strong>{formatMoney(result.dailyTotal, result.currency)}</strong></div>
        <div><span>Replicas</span><strong>{result.workload.replicas}</strong></div>
      </section>

      <section className="details-section">
        <div className="section-header"><h3>Workload profile</h3><span>{result.workload.resources.length} container{result.workload.resources.length === 1 ? "" : "s"}</span></div>
        <dl className="detail-list">
          <div><dt>Kind</dt><dd>Deployment</dd></div>
          <div><dt>Namespace</dt><dd>{result.workload.namespace || "default"}</dd></div>
          <div><dt>Provider</dt><dd>{result.pricing.provider.toUpperCase()} / {result.pricing.region}</dd></div>
          <div><dt>Worker node</dt><dd>{result.pricing.instanceType}</dd></div>
        </dl>
      </section>

      <section className="details-section">
        <div className="section-header"><h3>Container cost share</h3><span>USD / hour</span></div>
        <div className="cost-bars">
          {result.workload.resources.map((resource, index) => {
            const share = result.hourlyTotal ? (resource.hourlyCost / result.hourlyTotal) * 100 : 0;
            return (
              <div className="cost-bar-row" key={`${resource.name}-${index}`}>
                <div><strong>{resource.name}</strong><span>{formatMoney(resource.hourlyCost, result.currency)}</span></div>
                <div className="bar-track"><span style={{ width: `${Math.max(share, 2)}%` }} /></div>
              </div>
            );
          })}
        </div>
      </section>
    </div>
  );
}

function Resources({ result }: { result: ManifestResult }) {
  return (
    <div className="resource-view">
      <div className="section-header"><h3>Requested resources</h3><span>Per pod</span></div>
      <div className="resource-table-wrap">
        <table className="resource-table">
          <thead><tr><th>Container</th><th>CPU</th><th>Memory</th><th>Storage</th><th>GPU</th><th>Hourly</th></tr></thead>
          <tbody>
            {result.workload.resources.map((resource, index) => (
              <tr key={`${resource.name}-${index}`}>
                <td><span className="container-icon"><Boxes size={14} /></span>{resource.name}</td>
                <td>{resource.cpuCores} cores</td>
                <td>{resource.memoryGB.toFixed(2)} GiB</td>
                <td>{resource.storageGB.toFixed(2)} GiB</td>
                <td>{resource.gpuUnits}</td>
                <td>{formatMoney(resource.hourlyCost, result.currency)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function App() {
  return (
    <ClusterProvider>
      <EstimationHistoryProvider>
        <AppContent />
      </EstimationHistoryProvider>
    </ClusterProvider>
  );
}

export default App;
import {
  AlertCircle,
  ArrowRight,
  Boxes,
  Check,
  ChevronDown,
  CircleDollarSign,
  Clock,
  Cloud,
  Code2,
  FileCode2,
  Gauge,
  KeyRound,
  Layers3,
  Lightbulb,
  LoaderCircle,
  Network,
  PlugZap,
  RotateCcw,
  ServerCog,
  ShieldCheck,
  SlidersHorizontal,
  UploadCloud,
  X,
  Zap,
} from "lucide-react";
import { ChangeEvent, DragEvent, useEffect, useRef, useState } from "react";
import { estimateManifest, ManifestResult } from "./backend";
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
        {activeSection === "cluster" && isConnected() && <ClusterManagementPanel />}
        <div className="app-main">
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

function ClusterManagementPanel() {
  return (
    <aside className="cluster-sidebar" aria-label="Cluster management options">
      <span className="sidebar-label">CLUSTER</span>
      <div className="sidebar-placeholder">
        <Lightbulb size={18} />
        <p>Management options coming soon</p>
      </div>
    </aside>
  );
}

function ClusterConnection() {
  const { isConnected, clusterConnection } = useCluster();
  const [isModalOpen, setIsModalOpen] = useState(false);

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
      <section className="cluster-management-view">
        <div className="management-header">
          <div>
            <span className="step-label">CLUSTER</span>
            <h1>{clusterConnection?.name || "Connected Cluster"}</h1>
            <p>Manage and monitor your connected Kubernetes cluster</p>
          </div>
          <button
            type="button"
            className="icon-button"
            onClick={() => setIsModalOpen(true)}
            title="Edit cluster connection"
            aria-label="Edit cluster connection"
          >
            <Gauge size={17} />
          </button>
        </div>

        <div className="empty-state">
          <div className="empty-visual" aria-hidden="true">
            <div className="visual-node visual-main"><Network size={27} /></div>
            <div className="visual-node visual-one"><Boxes size={18} /></div>
            <div className="visual-node visual-two"><Layers3 size={18} /></div>
            <div className="visual-node visual-three"><ServerCog size={18} /></div>
            <span className="connector connector-one" /><span className="connector connector-two" /><span className="connector connector-three" />
          </div>
          <h3>Cluster features coming soon</h3>
          <p>Additional cluster management features will be available in future releases. For now, you can use this cluster for accurate cost estimation in the Manifest section.</p>
          <div className="empty-capabilities">
            <span><Check size={14} /> Live workload inspection</span>
            <span><Check size={14} /> Real-time metrics</span>
            <span><Check size={14} /> Cluster insights</span>
          </div>
        </div>
      </section>
      <ConnectionModal isOpen={isModalOpen} onClose={() => setIsModalOpen(false)} />
    </>
  );
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
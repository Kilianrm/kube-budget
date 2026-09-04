import {
  AlertCircle,
  ArrowRight,
  Boxes,
  Check,
  ChevronDown,
  CircleDollarSign,
  Cloud,
  Code2,
  FileCode2,
  Gauge,
  KeyRound,
  Layers3,
  LoaderCircle,
  Network,
  PlugZap,
  RotateCcw,
  ServerCog,
  ShieldCheck,
  SlidersHorizontal,
  UploadCloud,
} from "lucide-react";
import { ChangeEvent, DragEvent, useRef, useState } from "react";
import { estimateManifest, ManifestResult } from "./backend";

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
`;

const regionInstances: Record<string, string[]> = {
  "us-east-1": ["m6i.large", "m6i.xlarge", "c6i.large", "t3.medium"],
  "eu-west-1": ["m6i.large", "m6i.xlarge", "c6i.large", "t3.medium"],
};

type ResultTab = "overview" | "resources" | "source";
type AppSection = "estimate" | "cluster";

function formatMoney(value: number, currency = "USD") {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
    minimumFractionDigits: value < 1 ? 4 : 2,
    maximumFractionDigits: 4,
  }).format(value);
}

function App() {
  const [activeSection, setActiveSection] = useState<AppSection>("estimate");
  const fileInput = useRef<HTMLInputElement>(null);
  const [fileName, setFileName] = useState("");
  const [manifest, setManifest] = useState("");
  const [region, setRegion] = useState("us-east-1");
  const [instanceType, setInstanceType] = useState("m6i.large");
  const [result, setResult] = useState<ManifestResult | null>(null);
  const [activeTab, setActiveTab] = useState<ResultTab>("overview");
  const [isDragging, setIsDragging] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");

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

    setIsLoading(true);
    setError("");
    try {
      const estimate = await estimateManifest({
        documents: [{ name: fileName || "untitled.yaml", content: manifest }],
        provider: "aws",
        region,
        instanceType,
      });
      setResult(estimate);
      setActiveTab("overview");
    } catch (reason) {
      setResult(null);
      setError(reason instanceof Error ? reason.message : String(reason));
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <main className="app-shell">
      <header className="topbar">
        <div className="brand">
          <div className={`brand-mark ${activeSection === "cluster" ? "cluster" : ""}`} aria-hidden="true">
            {activeSection === "estimate" ? <Gauge size={21} /> : <Network size={21} />}
          </div>
          <div>
            <strong>KubeBudget</strong>
            <span>{activeSection === "estimate" ? "Manifest workspace" : "Cluster workspace"}</span>
          </div>
        </div>
        <nav className="primary-nav" aria-label="Main sections">
          <button
            type="button"
            className={activeSection === "estimate" ? "active estimate" : ""}
            onClick={() => setActiveSection("estimate")}
            aria-current={activeSection === "estimate" ? "page" : undefined}
          >
            <CircleDollarSign size={16} /> Cost estimation
          </button>
          <button
            type="button"
            className={activeSection === "cluster" ? "active cluster" : ""}
            onClick={() => setActiveSection("cluster")}
            aria-current={activeSection === "cluster" ? "page" : undefined}
          >
            <Network size={16} /> Connect cluster
          </button>
        </nav>
        <div className="runtime-status"><span className="status-dot" /> Local desktop</div>
      </header>

      {activeSection === "estimate" ? <section className="workspace">
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

          <div className="pricing-section">
            <div className="section-title"><Cloud size={16} /><span>Pricing assumptions</span></div>
            <div className="control-grid">
              <label className="select-field">
                <span>Provider</span>
                <div className="select-wrap"><select value="aws" disabled><option>AWS</option></select><ChevronDown size={15} /></div>
              </label>
              <label className="select-field">
                <span>Region</span>
                <div className="select-wrap">
                  <select value={region} onChange={(event) => { setRegion(event.target.value); setInstanceType("m6i.large"); setResult(null); }}>
                    {Object.keys(regionInstances).map((value) => <option key={value}>{value}</option>)}
                  </select><ChevronDown size={15} />
                </div>
              </label>
              <label className="select-field full-width">
                <span>Worker instance</span>
                <div className="select-wrap">
                  <select value={instanceType} onChange={(event) => { setInstanceType(event.target.value); setResult(null); }}>
                    {regionInstances[region].map((value) => <option key={value}>{value}</option>)}
                  </select><ChevronDown size={15} />
                </div>
              </label>
            </div>
          </div>

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
            {result && <div className="valid-badge"><Check size={14} /> Valid deployment</div>}
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
      </section> : <ClusterConnection />}
    </main>
  );
}

function ClusterConnection() {
  const [source, setSource] = useState<"kubeconfig" | "manual">("kubeconfig");
  const [connectionName, setConnectionName] = useState("Local development");
  const [kubeconfigPath, setKubeconfigPath] = useState("~/.kube/config");
  const [context, setContext] = useState("docker-desktop");
  const [namespace, setNamespace] = useState("All namespaces");
  const [notice, setNotice] = useState("");

  return (
    <section className="cluster-workspace">
      <div className="cluster-intro">
        <div>
          <span className="cluster-eyebrow">KUBERNETES ACCESS</span>
          <h1>Connect a cluster</h1>
          <p>Configure the local credentials and scope KubeBudget will use to inspect live workloads.</p>
        </div>
        <div className="cluster-state"><span /> Not connected</div>
      </div>

      <div className="cluster-layout">
        <form className="connection-form" onSubmit={(event) => { event.preventDefault(); setNotice("Cluster connectivity is not available in this UI-only version."); }}>
          <div className="form-section-heading">
            <div className="cluster-icon"><KeyRound size={18} /></div>
            <div><span>01 / CREDENTIALS</span><h2>Connection source</h2></div>
          </div>

          <div className="source-control" role="group" aria-label="Connection source">
            <button type="button" className={source === "kubeconfig" ? "active" : ""} onClick={() => { setSource("kubeconfig"); setNotice(""); }}>
              <Code2 size={16} /><span><strong>Kubeconfig</strong><small>Use a local configuration file</small></span>
            </button>
            <button type="button" className={source === "manual" ? "active" : ""} onClick={() => { setSource("manual"); setNotice(""); }}>
              <SlidersHorizontal size={16} /><span><strong>Manual</strong><small>Enter API server details</small></span>
            </button>
          </div>

          <div className="cluster-fields">
            <label className="cluster-field full-width">
              <span>Connection name</span>
              <input value={connectionName} onChange={(event) => setConnectionName(event.target.value)} placeholder="Production cluster" />
            </label>

            {source === "kubeconfig" ? <>
              <label className="cluster-field full-width">
                <span>Kubeconfig path</span>
                <div className="path-input"><input value={kubeconfigPath} onChange={(event) => setKubeconfigPath(event.target.value)} /><button type="button" title="Choose kubeconfig file" aria-label="Choose kubeconfig file"><FileCode2 size={17} /></button></div>
              </label>
              <label className="cluster-field full-width">
                <span>Context</span>
                <div className="select-wrap cluster-select"><select value={context} onChange={(event) => setContext(event.target.value)}><option>docker-desktop</option><option>minikube</option><option>kind-local</option></select><ChevronDown size={15} /></div>
              </label>
            </> : <>
              <label className="cluster-field full-width">
                <span>API server URL</span>
                <input type="url" placeholder="https://kubernetes.example.com:6443" />
              </label>
              <label className="cluster-field">
                <span>Bearer token</span>
                <input type="password" placeholder="Token" autoComplete="off" />
              </label>
              <label className="cluster-field">
                <span>CA certificate</span>
                <input placeholder="Certificate path" />
              </label>
            </>}
          </div>

          <div className="form-divider" />
          <div className="form-section-heading compact">
            <div className="cluster-icon"><Network size={18} /></div>
            <div><span>02 / SCOPE</span><h2>Workload access</h2></div>
          </div>
          <label className="cluster-field full-width">
            <span>Namespace scope</span>
            <div className="select-wrap cluster-select"><select value={namespace} onChange={(event) => setNamespace(event.target.value)}><option>All namespaces</option><option>default</option><option>production</option><option>kube-system</option></select><ChevronDown size={15} /></div>
          </label>

          <label className="permission-row">
            <input type="checkbox" defaultChecked />
            <span><strong>Read-only access</strong><small>Only workload metadata and resource requests will be inspected.</small></span>
          </label>

          {notice && <div className="cluster-notice" role="status"><AlertCircle size={16} /><span>{notice}</span></div>}
          <button className="connect-button" type="submit"><PlugZap size={18} /><span>Connect cluster</span><ArrowRight size={18} /></button>
        </form>

        <aside className="connection-preview">
          <div className="preview-header"><span>CONNECTION PREVIEW</span><ServerCog size={18} /></div>
          <div className="cluster-orbit" aria-hidden="true">
            <div className="orbit-ring outer" /><div className="orbit-ring inner" />
            <div className="orbit-core"><Network size={28} /></div>
            <span className="orbit-node node-one" /><span className="orbit-node node-two" /><span className="orbit-node node-three" />
          </div>
          <div className="preview-name"><strong>{connectionName || "Unnamed cluster"}</strong><span>{source === "kubeconfig" ? context : "Manual credentials"}</span></div>
          <dl className="preview-details">
            <div><dt>Credential source</dt><dd>{source === "kubeconfig" ? "Kubeconfig" : "Manual"}</dd></div>
            <div><dt>Namespace scope</dt><dd>{namespace}</dd></div>
            <div><dt>Access mode</dt><dd>Read only</dd></div>
          </dl>
          <div className="security-note"><ShieldCheck size={18} /><div><strong>Local credentials</strong><span>Connection details stay on this device and are never sent to a remote KubeBudget service.</span></div></div>
        </aside>
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
          <div><dt>Provider</dt><dd>AWS / {result.pricing.region}</dd></div>
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

export default App;
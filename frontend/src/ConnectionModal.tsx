import {
  AlertCircle,
  ArrowRight,
  Check,
  ChevronDown,
  Code2,
  FileCode2,
  KeyRound,
  LogOut,
  Network,
  PlugZap,
  ShieldCheck,
  SlidersHorizontal,
  X,
} from "lucide-react";
import { useState } from "react";
import { ClusterConnection, useCluster } from "./ClusterContext";

interface ConnectionModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export function ConnectionModal({ isOpen, onClose }: ConnectionModalProps) {
  const { clusterConnection, isConnecting, connectionError, connect, disconnect } = useCluster();
  const [source, setSource] = useState<"kubeconfig" | "manual">("kubeconfig");
  const [connectionName, setConnectionName] = useState("Local development");
  const [kubeconfigPath, setKubeconfigPath] = useState("~/.kube/config");
  const [context, setContext] = useState("docker-desktop");
  const [namespace, setNamespace] = useState("default");
  const [apiServerUrl, setApiServerUrl] = useState("");
  const [bearerToken, setBearerToken] = useState("");
  const [caCertificate, setCaCertificate] = useState("");
  const [readOnly, setReadOnly] = useState(true);
  const [notice, setNotice] = useState("");

  const handleConnect = async (e: React.FormEvent) => {
    e.preventDefault();
    setNotice("");

    try {
      await connect({
        name: connectionName,
        source,
        kubeconfigPath: source === "kubeconfig" ? kubeconfigPath : undefined,
        context: source === "kubeconfig" ? context : undefined,
        apiServerUrl: source === "manual" ? apiServerUrl : undefined,
        namespace,
        readOnly,
        connectedAt: Date.now(),
      });

      // Reset form
      setConnectionName("Local development");
      setKubeconfigPath("~/.kube/config");
      setContext("docker-desktop");
      setApiServerUrl("");
      setBearerToken("");
      setCaCertificate("");
      setReadOnly(true);
      setSource("kubeconfig");

      // Close modal after successful connection
      setTimeout(onClose, 300);
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "Failed to connect to cluster");
    }
  };

  const handleDisconnect = () => {
    disconnect();
    setNotice("Disconnected from cluster");
    setTimeout(() => {
      setNotice("");
      onClose();
    }, 1000);
  };

  if (!isOpen) return null;

  return (
    <>
      <div className="modal-backdrop" onClick={onClose} />
      <div className="modal-dialog">
        <div className="modal-header">
          <div>
            <h2>{clusterConnection ? "Connected cluster" : "Connect to a cluster"}</h2>
            <p>
              {clusterConnection
                ? "View and manage your cluster connection"
                : "Configure the credentials KubeBudget will use to inspect live workloads"}
            </p>
          </div>
          <button type="button" className="modal-close-button" onClick={onClose} aria-label="Close modal">
            <X size={20} />
          </button>
        </div>

        {clusterConnection ? (
          <div className="modal-content">
            <div className="connection-details">
              <div className="detail-row">
                <span className="detail-label">Connection name</span>
                <span className="detail-value">{clusterConnection.name}</span>
              </div>
              <div className="detail-row">
                <span className="detail-label">Credential source</span>
                <span className="detail-value">{clusterConnection.source === "kubeconfig" ? "Kubeconfig" : "Manual credentials"}</span>
              </div>
              {clusterConnection.source === "kubeconfig" && (
                <>
                  <div className="detail-row">
                    <span className="detail-label">Context</span>
                    <span className="detail-value">{clusterConnection.context}</span>
                  </div>
                </>
              )}
              <div className="detail-row">
                <span className="detail-label">Namespace scope</span>
                <span className="detail-value">{clusterConnection.namespace}</span>
              </div>
              <div className="detail-row">
                <span className="detail-label">Access mode</span>
                <span className="detail-value">{clusterConnection.readOnly ? "Read only" : "Read/Write"}</span>
              </div>
              <div className="detail-row">
                <span className="detail-label">Connected since</span>
                <span className="detail-value">
                  {new Date(clusterConnection.connectedAt).toLocaleTimeString()}
                </span>
              </div>
            </div>

            <div className="modal-footer">
              <button
                type="button"
                className="disconnect-button"
                onClick={handleDisconnect}
              >
                <LogOut size={16} />
                Disconnect cluster
              </button>
              <button type="button" className="secondary-button" onClick={onClose}>
                Close
              </button>
            </div>
          </div>
        ) : (
          <form className="modal-content" onSubmit={handleConnect}>
            <div className="form-section">
              <div className="form-section-heading">
                <div className="cluster-icon">
                  <KeyRound size={18} />
                </div>
                <div>
                  <span>01 / CREDENTIALS</span>
                  <h3>Connection source</h3>
                </div>
              </div>

              <div className="source-control" role="group" aria-label="Connection source">
                <button
                  type="button"
                  className={source === "kubeconfig" ? "active" : ""}
                  onClick={() => {
                    setSource("kubeconfig");
                    setNotice("");
                  }}
                >
                  <Code2 size={16} />
                  <span>
                    <strong>Kubeconfig</strong>
                    <small>Use a local configuration file</small>
                  </span>
                </button>
                <button
                  type="button"
                  className={source === "manual" ? "active" : ""}
                  onClick={() => {
                    setSource("manual");
                    setNotice("");
                  }}
                >
                  <SlidersHorizontal size={16} />
                  <span>
                    <strong>Manual</strong>
                    <small>Enter API server details</small>
                  </span>
                </button>
              </div>

              <div className="form-fields">
                <label className="form-field full-width">
                  <span>Connection name</span>
                  <input
                    value={connectionName}
                    onChange={(e) => setConnectionName(e.target.value)}
                    placeholder="Production cluster"
                    required
                  />
                </label>

                {source === "kubeconfig" ? (
                  <>
                    <label className="form-field full-width">
                      <span>Kubeconfig path</span>
                      <div className="path-input">
                        <input
                          value={kubeconfigPath}
                          onChange={(e) => setKubeconfigPath(e.target.value)}
                          placeholder="~/.kube/config"
                          required
                        />
                        <button type="button" title="Choose kubeconfig file" aria-label="Choose kubeconfig file">
                          <FileCode2 size={17} />
                        </button>
                      </div>
                    </label>
                    <label className="form-field full-width">
                      <span>Context</span>
                      <div className="select-wrap">
                        <select
                          value={context}
                          onChange={(e) => setContext(e.target.value)}
                          required
                        >
                          <option>docker-desktop</option>
                          <option>minikube</option>
                          <option>kind-local</option>
                        </select>
                        <ChevronDown size={15} />
                      </div>
                    </label>
                  </>
                ) : (
                  <>
                    <label className="form-field full-width">
                      <span>API server URL</span>
                      <input
                        type="url"
                        placeholder="https://kubernetes.example.com:6443"
                        value={apiServerUrl}
                        onChange={(e) => setApiServerUrl(e.target.value)}
                        required
                      />
                    </label>
                    <label className="form-field">
                      <span>Bearer token</span>
                      <input
                        type="password"
                        placeholder="Token"
                        autoComplete="off"
                        value={bearerToken}
                        onChange={(e) => setBearerToken(e.target.value)}
                        required
                      />
                    </label>
                    <label className="form-field">
                      <span>CA certificate</span>
                      <input
                        placeholder="Certificate path"
                        value={caCertificate}
                        onChange={(e) => setCaCertificate(e.target.value)}
                      />
                    </label>
                  </>
                )}
              </div>
            </div>

            <div className="form-divider" />

            <div className="form-section">
              <div className="form-section-heading compact">
                <div className="cluster-icon">
                  <Network size={18} />
                </div>
                <div>
                  <span>02 / SCOPE</span>
                  <h3>Workload access</h3>
                </div>
              </div>

              <label className="form-field full-width">
                <span>Namespace scope</span>
                <div className="select-wrap">
                  <select value={namespace} onChange={(e) => setNamespace(e.target.value)} required>
                    <option>All namespaces</option>
                    <option>default</option>
                    <option>production</option>
                    <option>kube-system</option>
                  </select>
                  <ChevronDown size={15} />
                </div>
              </label>

              <label className="permission-row">
                <input
                  type="checkbox"
                  checked={readOnly}
                  onChange={(e) => setReadOnly(e.target.checked)}
                />
                <span>
                  <strong>Read-only access</strong>
                  <small>Only workload metadata and resource requests will be inspected.</small>
                </span>
              </label>
            </div>

            {notice && connectionError && (
              <div className="modal-notice error" role="status">
                <AlertCircle size={16} />
                <span>{notice || connectionError}</span>
              </div>
            )}

            <div className="modal-footer">
              <button
                type="button"
                className="secondary-button"
                onClick={onClose}
                disabled={isConnecting}
              >
                Cancel
              </button>
              <button
                type="submit"
                className="primary-button"
                disabled={isConnecting}
              >
                {isConnecting ? (
                  <>
                    <div className="spinner" />
                    Connecting...
                  </>
                ) : (
                  <>
                    <PlugZap size={16} />
                    Connect cluster
                  </>
                )}
              </button>
            </div>
          </form>
        )}
      </div>
    </>
  );
}

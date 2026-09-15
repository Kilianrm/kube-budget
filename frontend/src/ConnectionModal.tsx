import {
  AlertCircle,
  ArrowRight,
  Check,
  CheckCircle2,
  ChevronDown,
  Circle,
  Code2,
  FileCode2,
  KeyRound,
  LoaderCircle,
  LogOut,
  Network,
  PlugZap,
  ShieldCheck,
  X,
} from "lucide-react";
import { useState } from "react";
import { useEffect, useRef } from "react";
import { listKubeconfigContexts } from "./backend";
import { ClusterConnection, useCluster } from "./ClusterContext";

interface ConnectionModalProps {
  isOpen: boolean;
  onClose: () => void;
}

const connectionSteps = [
  "Read kubeconfig and selected context",
  "Verify access to the Kubernetes API",
  "Load cluster identity and version",
];

export function ConnectionModal({ isOpen, onClose }: ConnectionModalProps) {
  const { clusterConnection, isConnecting, connectionError, connect, confirmConnection, disconnect } = useCluster();
  const source = "kubeconfig" as const;
  const [connectionName, setConnectionName] = useState("Local development");
  const [kubeconfigPath, setKubeconfigPath] = useState("~/.kube/config");
  const [context, setContext] = useState("");
  const [contexts, setContexts] = useState<Array<{ name: string; server: string }>>([]);
  const [namespace, setNamespace] = useState("All namespaces");
  const [readOnly, setReadOnly] = useState(true);
  const [notice, setNotice] = useState("");
  const [noticeKind, setNoticeKind] = useState<"info" | "success" | "error">("info");
  const [completedSteps, setCompletedSteps] = useState(0);
  const [pendingConnection, setPendingConnection] = useState<ClusterConnection | null>(null);
  const [showSuccessResult, setShowSuccessResult] = useState(false);
  const [isDisconnecting, setIsDisconnecting] = useState(false);
  const commitTimer = useRef<number | null>(null);
  const disconnectNoticeTimer = useRef<number | null>(null);
  const modalDetailsRef = useRef<HTMLDivElement>(null);
  const modalFormRef = useRef<HTMLFormElement>(null);

  const displayedConnection = pendingConnection ?? clusterConnection;

  function clearCommitTimer() {
    if (commitTimer.current !== null) {
      window.clearTimeout(commitTimer.current);
      commitTimer.current = null;
    }
  }

  function clearDisconnectNoticeTimer() {
    if (disconnectNoticeTimer.current !== null) {
      window.clearTimeout(disconnectNoticeTimer.current);
      disconnectNoticeTimer.current = null;
    }
  }

  const handleClose = () => {
    clearCommitTimer();
    clearDisconnectNoticeTimer();
    if (isDisconnecting) {
      disconnect();
    }
    if (pendingConnection) {
      confirmConnection(pendingConnection);
      setPendingConnection(null);
    }
    setIsDisconnecting(false);
    setShowSuccessResult(false);
    setNotice("");
    onClose();
  };

  useEffect(() => {
    if (!isOpen) return;

    listKubeconfigContexts(kubeconfigPath)
      .then((availableContexts) => {
        setContexts(availableContexts);
        setContext((current) => current || availableContexts[0]?.name || "");
        setNotice(availableContexts.length ? "" : "No contexts were found in this kubeconfig");
      })
      .catch((error) => {
        setContexts([]);
        setContext("");
        setNotice(error instanceof Error ? error.message : "Unable to read kubeconfig");
      });
  }, [isOpen, kubeconfigPath]);

  useEffect(() => {
    if (isConnecting || isDisconnecting) {
      (modalDetailsRef.current ?? modalFormRef.current)?.scrollTo({ top: 0, behavior: "auto" });
    }
  }, [isConnecting, isDisconnecting]);

  const handleConnect = async (e: React.FormEvent) => {
    e.preventDefault();
    clearDisconnectNoticeTimer();
    setNotice("");
    setNoticeKind("info");
    setCompletedSteps(0);
    setNotice("Verifying the selected cluster...");
    const minimumCheckTime = new Promise((resolve) => setTimeout(resolve, 1000));
    const stepProgress = window.setInterval(() => {
      setCompletedSteps((current) => Math.min(current + 1, connectionSteps.length - 1));
    }, 650);

    try {
          const [connectedCluster] = await Promise.all([
          connect({
          name: connectionName,
          source,
          kubeconfigPath,
          context,
          namespace,
          readOnly,
          connectedAt: Date.now(),
        }),
        minimumCheckTime,
        ]);

      window.clearInterval(stepProgress);
      setCompletedSteps(connectionSteps.length);
      setNoticeKind("success");
      setNotice("Connection verified. Review the cluster details below.");
      setPendingConnection(connectedCluster);
      setShowSuccessResult(true);
      clearCommitTimer();
      commitTimer.current = window.setTimeout(() => {
        confirmConnection(connectedCluster);
        setPendingConnection(null);
        commitTimer.current = null;
      }, 900);

      // Reset form
      setConnectionName("Local development");
      setKubeconfigPath("~/.kube/config");
      setReadOnly(true);
    } catch (error) {
      window.clearInterval(stepProgress);
      await minimumCheckTime;
      setNoticeKind("error");
      setNotice(error instanceof Error ? error.message : "Failed to connect to cluster");
    }
  };

  const handleDisconnect = () => {
    clearCommitTimer();
    clearDisconnectNoticeTimer();
    setIsDisconnecting(true);
    setNoticeKind("error");
    setNotice("Disconnecting cluster...");
    disconnectNoticeTimer.current = window.setTimeout(() => {
      disconnect();
      setPendingConnection(null);
      setShowSuccessResult(false);
      setIsDisconnecting(false);
      setNotice("");
      disconnectNoticeTimer.current = null;
    }, 1200);
  };

  if (!isOpen) return null;

  return (
    <>
      <div className="modal-backdrop" onClick={handleClose} />
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
          <button type="button" className="modal-close-button" onClick={handleClose} aria-label="Close modal">
            <X size={20} />
          </button>
        </div>

        {displayedConnection ? (
          <div className="modal-content" ref={modalDetailsRef}>
            {isDisconnecting && (
              <div className="modal-notice error" role="status" aria-live="polite">
                <LoaderCircle size={16} className="spin" />
                <span>{notice}</span>
              </div>
            )}

            {showSuccessResult && !isDisconnecting && (
              <div className="connection-result" role="status">
                <div className="connection-result-heading">
                  <div className="connection-result-icon">
                    <CheckCircle2 size={24} />
                  </div>
                  <div>
                    <span className="connection-result-eyebrow">CONNECTION COMPLETE</span>
                    <h3>Cluster is ready to use</h3>
                    <p>{notice || "The cluster connection was verified successfully."}</p>
                  </div>
                </div>

                <div className="connection-steps" aria-label="Connection verification steps">
                  {connectionSteps.map((step, index) => (
                    <div className="connection-step" key={step}>
                      <div className="connection-step-marker">
                        {index < completedSteps ? <Check size={13} /> : <Circle size={11} />}
                      </div>
                      <span>{step}</span>
                    </div>
                  ))}
                </div>

                <div className="connection-data-heading">
                  <span>VERIFIED CLUSTER DATA</span>
                  <small>Returned by the Kubernetes API</small>
                </div>
              </div>
            )}

            <div className="connection-details">
              <div className="detail-row">
                <span className="detail-label">Connection name</span>
                  <span className="detail-value">{displayedConnection.name}</span>
              </div>
              <div className="detail-row">
                <span className="detail-label">Credential source</span>
                  <span className="detail-value">{displayedConnection.source === "kubeconfig" ? "Kubeconfig" : "Manual credentials"}</span>
              </div>
              {displayedConnection.source === "kubeconfig" && (
                <>
                  <div className="detail-row">
                    <span className="detail-label">Context</span>
                    <span className="detail-value">{displayedConnection.context}</span>
                  </div>
                  <div className="detail-row">
                    <span className="detail-label">API server</span>
                    <span className="detail-value">{displayedConnection.server}</span>
                  </div>
                  <div className="detail-row">
                    <span className="detail-label">Kubernetes version</span>
                    <span className="detail-value">{displayedConnection.version}</span>
                  </div>
                </>
              )}
              <div className="detail-row">
                <span className="detail-label">Namespace scope</span>
                <span className="detail-value">{displayedConnection.namespace}</span>
              </div>
              <div className="detail-row">
                <span className="detail-label">Access mode</span>
                <span className="detail-value">{displayedConnection.readOnly ? "Read only" : "Read/Write"}</span>
              </div>
              <div className="detail-row">
                <span className="detail-label">Connected since</span>
                <span className="detail-value">
                  {new Date(displayedConnection.connectedAt).toLocaleTimeString()}
                </span>
              </div>
            </div>

            <div className="modal-footer">
              <button
                type="button"
                className="disconnect-button"
                onClick={handleDisconnect}
                disabled={isDisconnecting}
              >
                <LogOut size={16} />
                Disconnect cluster
              </button>
            </div>
          </div>
        ) : (
          <form className="modal-content" ref={modalFormRef} onSubmit={handleConnect}>
            {isConnecting ? (
              <div className="connection-progress" role="status" aria-live="polite">
                <div className="connection-progress-heading">
                  <LoaderCircle size={17} className="spin" />
                  <div>
                    <strong>Verifying cluster connection</strong>
                    <span>We are checking access and reading cluster metadata.</span>
                  </div>
                </div>
                <div className="connection-steps">
                  {connectionSteps.map((step, index) => (
                    <div className="connection-step" key={step}>
                      <div className="connection-step-marker">
                        {index < completedSteps ? <Check size={13} /> : <Circle size={11} />}
                      </div>
                      <span>{step}</span>
                    </div>
                  ))}
                </div>
              </div>
            ) : notice && (
              <div className={`modal-notice ${noticeKind}`} role="status">
                <AlertCircle size={16} />
                <span>{notice || connectionError}</span>
              </div>
            )}

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
                <div className="active">
                  <Code2 size={16} />
                  <span>
                    <strong>Kubeconfig</strong>
                    <small>Use credentials from your kubeconfig file</small>
                  </span>
                </div>
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
                        <select value={context} onChange={(e) => setContext(e.target.value)} required>
                          <option value="" disabled>Select a context</option>
                          {contexts.map((availableContext) => (
                            <option key={availableContext.name} value={availableContext.name}>
                              {availableContext.name}
                            </option>
                          ))}
                        </select>
                        <ChevronDown size={15} />
                      </div>
                    </label>
                </>
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

            <div className="modal-footer">
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

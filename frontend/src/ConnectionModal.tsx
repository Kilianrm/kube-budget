import {
  AlertCircle,
  ArrowRight,
  Check,
  CheckCircle2,
  ChevronDown,
  Circle,
  Cloud,
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
import { ClusterContext, listEKSClusters, listKubeconfigContexts } from "./backend";
import { ClusterConnection, defaultRefreshIntervalMs, useCluster } from "./ClusterContext";

interface ConnectionModalProps {
  isOpen: boolean;
  onClose: () => void;
}

const connectionSteps = [
  "Prepare the selected connection",
  "Verify access to the Kubernetes API",
  "Load cluster identity and version",
];

export function ConnectionModal({ isOpen, onClose }: ConnectionModalProps) {
  const { clusterConnection, isConnecting, connectionError, connect, confirmConnection, updateConnection, disconnect } = useCluster();
  const [source, setSource] = useState<"kubeconfig" | "eks">("kubeconfig");
  const [kubeconfigPath, setKubeconfigPath] = useState("~/.kube/config");
  const [context, setContext] = useState("");
  const [clusterName, setClusterName] = useState("");
  const [region, setRegion] = useState("us-east-1");
  const [profile, setProfile] = useState("default");
  const [roleArn, setRoleArn] = useState("");
  const [eksClusters, setEksClusters] = useState<string[]>([]);
  const [isLoadingEksClusters, setIsLoadingEksClusters] = useState(false);
  const [contexts, setContexts] = useState<ClusterContext[]>([]);
  const [namespace, setNamespace] = useState("All namespaces");
  const [readOnly, setReadOnly] = useState(true);
  const [refreshIntervalMs, setRefreshIntervalMs] = useState<number | null>(defaultRefreshIntervalMs);
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

  function handleSourceChange(nextSource: "kubeconfig" | "eks") {
    setSource(nextSource);
    setContext("");
    setNotice("");
  }

  function connectionDisplayName() {
    if (source === "eks") return clusterName;

    const selectedContext = contexts.find((availableContext) => availableContext.name === context);
    const clusterIdentifier = selectedContext?.cluster ?? "";
    if (context.startsWith("eks/")) {
      return context.split("/")[1] || context;
    }
    if (clusterIdentifier.includes(":eks:") && clusterIdentifier.includes(":cluster/")) {
      return clusterIdentifier.split(":cluster/")[1] || context;
    }
    if (selectedContext?.server.includes(".eks.amazonaws.com") && clusterIdentifier) {
      return clusterIdentifier;
    }
    return context;
  }

  function selectedKubeconfigProvider() {
    if (source !== "kubeconfig") return undefined;
    const selectedContext = contexts.find((availableContext) => availableContext.name === context);
    if (!selectedContext?.server.includes(".eks.amazonaws.com")) return undefined;
    const clusterName = selectedContext.cluster.startsWith("arn:")
      ? selectedContext.cluster.split(":cluster/")[1] || connectionDisplayName()
      : connectionDisplayName();
    const region = selectedContext.server.match(/\.([a-z0-9-]+)\.eks\.amazonaws\.com/)?.[1] ?? "";
    return { provider: "aws-eks" as const, clusterName, region, profile: "default", roleArn: "" };
  }

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

    if (source === "eks") return;

    listKubeconfigContexts(kubeconfigPath)
      .then((availableContexts) => {
        setContexts(availableContexts);
        setContext((current) => availableContexts.some((availableContext) => availableContext.name === current) ? current : "");
        setNotice(availableContexts.length ? "" : "No contexts were found in this kubeconfig");
      })
      .catch((error) => {
        setContexts([]);
        setContext("");
        setNotice(error instanceof Error ? error.message : "Unable to read kubeconfig");
      });
  }, [isOpen, kubeconfigPath, source]);

  useEffect(() => {
    if (!isOpen || source !== "eks" || !region.trim()) return;

    let cancelled = false;
    setIsLoadingEksClusters(true);
    listEKSClusters(region, profile)
      .then((availableClusters) => {
        if (cancelled) return;
        setEksClusters(availableClusters);
        setClusterName((current) => availableClusters.includes(current) ? current : "");
        setNotice(availableClusters.length ? "" : "No EKS clusters were found in this region");
      })
      .catch((error) => {
        if (cancelled) return;
        setEksClusters([]);
        setClusterName("");
        setNotice(error instanceof Error ? error.message : "Unable to discover EKS clusters");
      })
      .finally(() => {
        if (!cancelled) setIsLoadingEksClusters(false);
      });

    return () => {
      cancelled = true;
    };
  }, [isOpen, profile, region, source]);

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
          name: connectionDisplayName(),
          provider: source === "eks" ? "aws-eks" : undefined,
          ...selectedKubeconfigProvider(),
          source,
          kubeconfigPath,
          context,
          clusterName: source === "eks" ? clusterName : undefined,
          region: source === "eks" ? region : undefined,
          profile: source === "eks" ? profile : undefined,
          roleArn: source === "eks" ? roleArn : undefined,
          namespace,
          readOnly,
          refreshIntervalMs,
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
      setKubeconfigPath("~/.kube/config");
      setReadOnly(true);
      setRefreshIntervalMs(defaultRefreshIntervalMs);
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
                  <span className="detail-value">{displayedConnection.source === "eks" ? "Amazon EKS" : displayedConnection.source === "kubeconfig" ? "Kubeconfig" : "Manual credentials"}</span>
              </div>
              {displayedConnection.source !== "manual" && (
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
              <label className="form-field full-width">
                <span>Automatic refresh</span>
                <div className="select-wrap">
                  <select
                    value={displayedConnection.refreshIntervalMs ?? ""}
                    onChange={(event) => updateConnection({ refreshIntervalMs: event.target.value ? Number(event.target.value) : null })}
                    disabled={isDisconnecting}
                  >
                    <option value="">Off (manual refresh only)</option>
                    <option value="10000">Every 10 seconds</option>
                    <option value="30000">Every 30 seconds</option>
                    <option value="60000">Every minute</option>
                    <option value="300000">Every 5 minutes</option>
                    <option value="600000">Every 10 minutes</option>
                  </select>
                  <ChevronDown size={15} />
                </div>
              </label>
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
                <button type="button" className={source === "kubeconfig" ? "active" : ""} onClick={() => handleSourceChange("kubeconfig")}>
                  <Code2 size={16} />
                  <span>
                    <strong>Existing kubeconfig</strong>
                    <small>Use any existing Kubernetes context</small>
                  </span>
                </button>
                <button type="button" className={source === "eks" ? "active" : ""} onClick={() => handleSourceChange("eks")}>
                  <Cloud size={16} />
                  <span>
                    <strong>Discover Amazon EKS</strong>
                    <small>Find an AWS cluster and configure its context</small>
                  </span>
                </button>
              </div>

              <div className="form-fields">
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
                ) : (
                  <>
                    <label className="form-field full-width">
                      <span>EKS cluster</span>
                      <div className="select-wrap">
                        <select value={clusterName} onChange={(e) => setClusterName(e.target.value)} required disabled={isLoadingEksClusters}>
                          <option value="" disabled>{isLoadingEksClusters ? "Discovering clusters..." : "Select an EKS cluster"}</option>
                          {eksClusters.map((cluster) => (
                            <option key={cluster} value={cluster}>{cluster}</option>
                          ))}
                        </select>
                        <ChevronDown size={15} />
                      </div>
                    </label>
                    <label className="form-field">
                      <span>AWS region</span>
                      <input value={region} onChange={(e) => setRegion(e.target.value)} placeholder="eu-west-1" required />
                    </label>
                    <label className="form-field">
                      <span>AWS profile</span>
                      <input value={profile} onChange={(e) => setProfile(e.target.value)} placeholder="default" />
                    </label>
                    <label className="form-field full-width">
                      <span>Assume role ARN (optional)</span>
                      <input value={roleArn} onChange={(e) => setRoleArn(e.target.value)} placeholder="arn:aws:iam::123456789012:role/KubeBudgetViewer" />
                    </label>
                    <p className="form-hint">Requires AWS CLI v2 and an active AWS SSO or IAM session.</p>
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

              <label className="form-field full-width">
                <span>Automatic refresh</span>
                <div className="select-wrap">
                  <select value={refreshIntervalMs ?? ""} onChange={(event) => setRefreshIntervalMs(event.target.value ? Number(event.target.value) : null)}>
                    <option value="">Off (manual refresh only)</option>
                    <option value="10000">Every 10 seconds</option>
                    <option value="30000">Every 30 seconds</option>
                    <option value="60000">Every minute</option>
                    <option value="300000">Every 5 minutes</option>
                    <option value="600000">Every 10 minutes</option>
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

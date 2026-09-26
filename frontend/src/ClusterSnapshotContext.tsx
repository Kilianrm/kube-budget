import { createContext, ReactNode, useCallback, useContext, useEffect, useRef, useState } from "react";
import { getClusterSnapshot, ClusterSnapshot } from "./backend";
import { useCluster } from "./ClusterContext";

// The live cluster snapshot, loaded when a cluster connects and refreshed in
// the background at the connection's interval, whichever section is open. The
// Cluster section reads it, so it shows data the moment it is opened.
interface ClusterSnapshotContextType {
  snapshot: ClusterSnapshot | null;
  isRefreshing: boolean;
  error: string;
  refresh: () => Promise<void>;
}

const ClusterSnapshotContext = createContext<ClusterSnapshotContextType | undefined>(undefined);

export function ClusterSnapshotProvider({ children }: { children: ReactNode }) {
  const { isConnected, clusterConnection } = useCluster();
  const [snapshot, setSnapshot] = useState<ClusterSnapshot | null>(null);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [error, setError] = useState("");
  const inFlight = useRef<Promise<void> | null>(null);

  const refresh = useCallback((): Promise<void> => {
    if (!clusterConnection?.context) {
      setError("The connected cluster does not have a kubeconfig context.");
      return Promise.resolve();
    }
    // A manual refresh during a background one waits for it instead of starting another.
    if (inFlight.current) return inFlight.current;

    setIsRefreshing(true);
    setError("");
    const request = getClusterSnapshot(
      clusterConnection.kubeconfigPath ?? "",
      clusterConnection.context,
      clusterConnection.namespace,
      clusterConnection.provider === "aws-eks" ? {
        name: "aws-eks",
        clusterName: clusterConnection.clusterName ?? clusterConnection.name,
        region: clusterConnection.region ?? "",
        profile: clusterConnection.profile ?? "default",
        roleArn: clusterConnection.roleArn ?? "",
      } : undefined,
    )
      .then((next) => setSnapshot(next))
      .catch((reason) => setError(reason instanceof Error ? reason.message : String(reason)))
      .finally(() => {
        inFlight.current = null;
        setIsRefreshing(false);
      });
    inFlight.current = request;
    return request;
  }, [clusterConnection?.context, clusterConnection?.kubeconfigPath, clusterConnection?.namespace, clusterConnection?.provider, clusterConnection?.clusterName, clusterConnection?.name, clusterConnection?.region, clusterConnection?.profile, clusterConnection?.roleArn]);

  // A new connection invalidates the old snapshot; load the new one right away.
  useEffect(() => {
    setSnapshot(null);
    setError("");
    if (isConnected()) void refresh();
  }, [isConnected, refresh]);

  const intervalMs = clusterConnection?.refreshIntervalMs;
  useEffect(() => {
    if (!intervalMs || !isConnected()) return;
    const intervalId = window.setInterval(() => {
      if (document.visibilityState === "visible") void refresh();
    }, intervalMs);
    return () => window.clearInterval(intervalId);
  }, [intervalMs, isConnected, refresh]);

  return (
    <ClusterSnapshotContext.Provider value={{ snapshot, isRefreshing, error, refresh }}>
      {children}
    </ClusterSnapshotContext.Provider>
  );
}

export function useClusterSnapshot() {
  const context = useContext(ClusterSnapshotContext);
  if (!context) {
    throw new Error("useClusterSnapshot must be used within a ClusterSnapshotProvider");
  }
  return context;
}

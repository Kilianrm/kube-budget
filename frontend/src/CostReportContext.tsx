import { createContext, ReactNode, useCallback, useContext, useEffect, useRef, useState } from "react";
import { getCostReport, CostReportResult, OptimizationResult } from "./backend";
import { useCluster } from "./ClusterContext";
import { prefetchCostHistory } from "./costHistoryCache";
import { useClusterSnapshot } from "./ClusterSnapshotContext";

// One cost report per connected cluster, shared by every section. The Cost
// section and the Manifest section's cluster impact read the same report, so
// they can never show different numbers for the same cluster.
interface CostReportContextType {
  report: CostReportResult | null;
  isLoading: boolean;
  error: string;
  /** Collects and prices the cluster again; resolves to the new report, or null on failure. */
  refresh: () => Promise<CostReportResult | null>;
  /** Replaces the optimization plan after a dismissal or an applied mark. */
  setOptimization: (optimization: OptimizationResult) => void;
}

const CostReportContext = createContext<CostReportContextType | undefined>(undefined);

export function CostReportProvider({ children }: { children: ReactNode }) {
  const { isConnected, clusterConnection } = useCluster();
  const { snapshot } = useClusterSnapshot();
  // A cluster the price lists do not cover has no report to refresh.
  const priceable = snapshot?.platform?.supported !== false;
  const [report, setReport] = useState<CostReportResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");
  const inFlight = useRef<Promise<CostReportResult | null> | null>(null);

  const refresh = useCallback((): Promise<CostReportResult | null> => {
    if (!clusterConnection?.context) {
      setError("The connected cluster does not have a kubeconfig context.");
      return Promise.resolve(null);
    }
    // Concurrent callers share one collection instead of starting another.
    if (inFlight.current) return inFlight.current;

    setIsLoading(true);
    setError("");
    const request = getCostReport(
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
      { region: clusterConnection.region ?? "" },
    )
      .then((next) => {
        setReport(next);
        prefetchCostHistory(next.clusterId, next.report.totals?.provisioned?.hourly ?? 0, String(next.report.generatedAt));
        return next;
      })
      .catch((reason) => {
        setError(reason instanceof Error ? reason.message : String(reason));
        return null;
      })
      .finally(() => {
        inFlight.current = null;
        setIsLoading(false);
      });
    inFlight.current = request;
    return request;
  }, [clusterConnection?.context, clusterConnection?.kubeconfigPath, clusterConnection?.namespace, clusterConnection?.provider, clusterConnection?.clusterName, clusterConnection?.name, clusterConnection?.region, clusterConnection?.profile, clusterConnection?.roleArn]);

  // A new connection invalidates the old report; load the new one right away.
  useEffect(() => {
    setReport(null);
    if (isConnected()) void refresh();
  }, [isConnected, refresh]);

  // Refresh in the background on the connection's automatic refresh, like the
  // cluster snapshot: both ticks start together, so the backend collects the
  // cluster once for both. The backend decides which reports go to history.
  const intervalMs = clusterConnection?.refreshIntervalMs;
  useEffect(() => {
    if (!intervalMs || !isConnected() || !priceable) return;
    const intervalId = window.setInterval(() => {
      if (document.visibilityState === "visible") void refresh();
    }, intervalMs);
    return () => window.clearInterval(intervalId);
  }, [intervalMs, isConnected, refresh, priceable]);

  const setOptimization = useCallback((optimization: OptimizationResult) => {
    setReport((current) => (current ? ({ ...current, optimization } as CostReportResult) : current));
  }, []);

  return (
    <CostReportContext.Provider value={{ report, isLoading, error, refresh, setOptimization }}>
      {children}
    </CostReportContext.Provider>
  );
}

export function useCostReport() {
  const context = useContext(CostReportContext);
  if (!context) {
    throw new Error("useCostReport must be used within a CostReportProvider");
  }
  return context;
}

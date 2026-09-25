import { EstimateManifest } from "../wailsjs/go/wails/ManifestAdapter";
import {
  GetClusterSnapshot,
	GetWorkloadYAML,
  GetCostReport,
  GetCostTrend,
  ListKubeconfigContexts,
  ListEKSClusters,
  PrepareEKSConnection,
  TestConnection,
} from "../wailsjs/go/wails/ClusterAdapter";
import { cluster, costmodel, costseries, optimize, wails } from "../wailsjs/go/models";

export interface ManifestRequest {
  documents: Array<{
    name: string;
    content: string;
  }>;
  provider?: string;
  region?: string;
  instanceType?: string;
  useClusterData?: boolean;
  clusterInfo?: {
    name: string;
    context?: string;
    namespace: string;
  };
}

export type ManifestResult = wails.ManifestResult;
export type ClusterContext = wails.KubeconfigContext;
export type ClusterConnectionResult = wails.ClusterConnectionResult;
export type ClusterSnapshot = cluster.Snapshot;
export type CostReport = costmodel.CostReport;
export type CostLineItem = costmodel.LineItem;
export type CostProjection = costmodel.Projection;
export type CostReportResult = wails.CostReportResult;
export type CostRecommendation = optimize.Recommendation;
export type CostTrend = costseries.Series;
export type CostTrendPoint = costseries.Point;

declare global {
  interface Window {
    go?: {
      wails?: {
        ManifestAdapter?: unknown;
        ClusterAdapter?: unknown;
      };
    };
  }
}

export async function estimateManifest(request: ManifestRequest): Promise<ManifestResult> {
  if (!window.go?.wails?.ManifestAdapter) {
    throw new Error("The Wails desktop runtime is unavailable. Start the dashboard with `wails dev`.");
  }

  return EstimateManifest(new wails.ManifestRequest(request));
}

export async function listKubeconfigContexts(kubeconfigPath: string): Promise<ClusterContext[]> {
  if (!window.go?.wails?.ClusterAdapter) {
    throw new Error("The Wails desktop runtime is unavailable. Start the dashboard with `wails dev`.");
  }

  return ListKubeconfigContexts(new wails.KubeconfigContextsRequest({ kubeconfigPath }));
}

export async function listEKSClusters(region: string, profile: string): Promise<string[]> {
  if (!window.go?.wails?.ClusterAdapter) {
    throw new Error("The Wails desktop runtime is unavailable. Start the dashboard with `wails dev`.");
  }

  return ListEKSClusters(new wails.EKSClustersRequest({ region, profile }));
}

export async function testClusterConnection(
  kubeconfigPath: string,
  context: string,
): Promise<ClusterConnectionResult> {
  if (!window.go?.wails?.ClusterAdapter) {
    throw new Error("The Wails desktop runtime is unavailable. Start the dashboard with `wails dev`.");
  }

  return TestConnection(new wails.ClusterConnectionRequest({ kubeconfigPath, context }));
}

export async function prepareEKSConnection(request: {
  kubeconfigPath: string;
  clusterName: string;
  region: string;
  profile: string;
  roleArn: string;
}): Promise<{ kubeconfigPath: string; context: string }> {
  if (!window.go?.wails?.ClusterAdapter) {
    throw new Error("The Wails desktop runtime is unavailable. Start the dashboard with `wails dev`.");
  }

  return PrepareEKSConnection(request);
}

export async function getClusterSnapshot(
  kubeconfigPath: string,
  context: string,
  namespace: string,
  provider?: { name: string; clusterName: string; region: string; profile: string; roleArn: string },
): Promise<ClusterSnapshot> {
  if (!window.go?.wails?.ClusterAdapter) {
    throw new Error("The Wails desktop runtime is unavailable. Start the dashboard with `wails dev`.");
  }

  return GetClusterSnapshot(new wails.ClusterSnapshotRequest({
    kubeconfigPath,
    context,
    namespace,
    provider: provider?.name ?? "",
    clusterName: provider?.clusterName ?? "",
    region: provider?.region ?? "",
    profile: provider?.profile ?? "",
    roleArn: provider?.roleArn ?? "",
  }));
}

export async function getCostReport(
  kubeconfigPath: string,
  context: string,
  namespace: string,
  provider?: { name: string; clusterName: string; region: string; profile: string; roleArn: string },
  pricing?: { provider?: string; region?: string; instanceType?: string },
): Promise<CostReportResult> {
  if (!window.go?.wails?.ClusterAdapter) {
    throw new Error("The Wails desktop runtime is unavailable. Start the dashboard with `wails dev`.");
  }

  return GetCostReport(new wails.CostReportRequest({
    kubeconfigPath,
    context,
    namespace,
    provider: provider?.name ?? "",
    clusterName: provider?.clusterName ?? "",
    region: provider?.region ?? "",
    profile: provider?.profile ?? "",
    roleArn: provider?.roleArn ?? "",
    pricingProvider: pricing?.provider ?? "",
    pricingRegion: pricing?.region ?? "",
    instanceType: pricing?.instanceType ?? "",
  }));
}

export async function getCostTrend(clusterId: string, days: number, bucket: "hour" | "day"): Promise<CostTrend> {
  if (!window.go?.wails?.ClusterAdapter) {
    throw new Error("The Wails desktop runtime is unavailable. Start the dashboard with `wails dev`.");
  }

  return GetCostTrend(new wails.CostTrendRequest({ clusterId, days, bucket }));
}

export async function getWorkloadYAML(
  kubeconfigPath: string,
  context: string,
  kind: string,
  namespace: string,
  name: string,
): Promise<string> {
  if (!window.go?.wails?.ClusterAdapter) {
    throw new Error("The Wails desktop runtime is unavailable. Start the dashboard with `wails dev`.");
  }

  return GetWorkloadYAML(new wails.WorkloadYAMLRequest({ kubeconfigPath, context, kind, namespace, name }));
}
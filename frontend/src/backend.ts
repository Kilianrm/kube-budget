import { EstimateManifest } from "../wailsjs/go/wails/ManifestAdapter";
import {
  GetClusterSnapshot,
	GetWorkloadYAML,
  ClaimRecommendation,
  DismissRecommendation,
  GetCostForecast,
  GetCostReport,
  GetCostTrend,
  MarkRecommendationApplied,
  RestoreRecommendation,
  ReopenRecommendation,
  SetCostBudget,
  SimulateManifest,
  ListKubeconfigContexts,
  ListEKSClusters,
  PrepareEKSConnection,
  TestConnection,
} from "../wailsjs/go/wails/ClusterAdapter";
import { cluster, costmodel, costseries, optimize, wails, whatif } from "../wailsjs/go/models";

export interface ManifestRequest {
  documents: Array<{
    name: string;
    content: string;
  }>;
  provider?: string;
  region?: string;
  instanceType?: string;
}

export type ManifestResult = wails.ManifestResult;
export type ClusterContext = wails.KubeconfigContext;
export type ClusterConnectionResult = wails.ClusterConnectionResult;
export type ClusterSnapshot = cluster.Snapshot;
export type CostReport = costmodel.CostReport;
export type CostLineItem = costmodel.LineItem;
export type CostAllocationRow = costmodel.AllocationRow;
export type CostProjection = costmodel.Projection;
export type CostReportResult = wails.CostReportResult;
export type OptimizationResult = wails.OptimizationResult;
export type SimulationResult = wails.SimulationResult;
export type SimulationImpact = whatif.Result;
export type OptimizationRecommendation = optimize.Recommendation;
export type OptimizationItem = optimize.Item;
export type AppliedRecommendation = wails.AppliedRecommendation;
export type RecommendationEvent = wails.RecommendationEvent;
export type DoneResource = wails.DoneResource;
export type CostTrend = wails.CostTrendResult;
export type CostSeries = costseries.Series;
export type CostDriver = costseries.Driver;
export type CostForecast = wails.CostForecastResult;
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

export async function getCostForecast(clusterId: string, runRateHourly: number): Promise<CostForecast> {
  if (!window.go?.wails?.ClusterAdapter) {
    throw new Error("The Wails desktop runtime is unavailable. Start the dashboard with `wails dev`.");
  }

  return GetCostForecast(new wails.CostForecastRequest({ clusterId, runRateHourly }));
}

export async function setCostBudget(clusterId: string, monthlyUSD: number): Promise<void> {
  if (!window.go?.wails?.ClusterAdapter) {
    throw new Error("The Wails desktop runtime is unavailable. Start the dashboard with `wails dev`.");
  }

  return SetCostBudget(new wails.CostBudgetRequest({ clusterId, monthlyUSD }));
}

function recommendationAction(call: (request: wails.RecommendationRequest) => Promise<wails.OptimizationResult>) {
  return async (clusterId: string, id: string): Promise<OptimizationResult> => {
    if (!window.go?.wails?.ClusterAdapter) {
      throw new Error("The Wails desktop runtime is unavailable. Start the dashboard with `wails dev`.");
    }
    return call(new wails.RecommendationRequest({ clusterId, id }));
  };
}

export const dismissRecommendation = recommendationAction(DismissRecommendation);
export const restoreRecommendation = recommendationAction(RestoreRecommendation);
export const reopenRecommendation = recommendationAction(ReopenRecommendation);
export const markRecommendationApplied = recommendationAction(MarkRecommendationApplied);
export const claimRecommendation = recommendationAction(ClaimRecommendation);

/** Prices a manifest at a connected cluster's rates and simulates deploying it. */
export async function simulateManifest(clusterId: string, document: { name: string; content: string }): Promise<SimulationResult> {
  if (!window.go?.wails?.ClusterAdapter) {
    throw new Error("The Wails desktop runtime is unavailable. Start the dashboard with `wails dev`.");
  }

  return SimulateManifest(new wails.SimulationRequest({ clusterId, document }));
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
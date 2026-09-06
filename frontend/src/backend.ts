import { EstimateManifest } from "../wailsjs/go/wails/ManifestAdapter";
import {
  ListKubeconfigContexts,
  TestConnection,
} from "../wailsjs/go/wails/ClusterAdapter";
import { wails } from "../wailsjs/go/models";

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

export async function testClusterConnection(
  kubeconfigPath: string,
  context: string,
): Promise<ClusterConnectionResult> {
  if (!window.go?.wails?.ClusterAdapter) {
    throw new Error("The Wails desktop runtime is unavailable. Start the dashboard with `wails dev`.");
  }

  return TestConnection(new wails.ClusterConnectionRequest({ kubeconfigPath, context }));
}
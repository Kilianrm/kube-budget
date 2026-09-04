import { EstimateManifest } from "../wailsjs/go/wails/ManifestAdapter";
import { wails } from "../wailsjs/go/models";

export interface ManifestRequest {
  documents: Array<{
    name: string;
    content: string;
  }>;
  provider: string;
  region: string;
  instanceType: string;
}

export type ManifestResult = wails.ManifestResult;

declare global {
  interface Window {
    go?: {
      wails?: {
        ManifestAdapter?: unknown;
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
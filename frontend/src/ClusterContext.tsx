import React, { createContext, useContext, useState, useCallback, ReactNode } from "react";
import { prepareEKSConnection, testClusterConnection } from "./backend";

export const defaultRefreshIntervalMs = 30_000;

export interface ClusterConnection {
  name: string;
  source: "kubeconfig" | "eks" | "manual";
  provider?: "aws-eks";
  kubeconfigPath?: string;
  context?: string;
  clusterName?: string;
  region?: string;
  profile?: string;
  roleArn?: string;
  apiServerUrl?: string;
  namespace: string;
  readOnly: boolean;
  refreshIntervalMs: number | null;
  connectedAt: number;
  server?: string;
  version?: string;
}

interface ClusterContextType {
  clusterConnection: ClusterConnection | null;
  isConnecting: boolean;
  connectionError: string | null;
  connect: (connection: ClusterConnection) => Promise<ClusterConnection>;
  confirmConnection: (connection: ClusterConnection) => void;
  updateConnection: (updates: Partial<Pick<ClusterConnection, "refreshIntervalMs">>) => void;
  disconnect: () => void;
  isConnected: () => boolean;
}

const ClusterContext = createContext<ClusterContextType | undefined>(undefined);

export function ClusterProvider({ children }: { children: ReactNode }) {
  const [clusterConnection, setClusterConnection] = useState<ClusterConnection | null>(() => {
    // Try to restore connection from localStorage
    try {
      const stored = localStorage.getItem("cluster-connection");
      if (!stored) return null;
      const connection = JSON.parse(stored) as ClusterConnection;
      const normalizedConnection = normalizeEKSConnection(connection);
      return {
        ...normalizedConnection,
        refreshIntervalMs: normalizedConnection.refreshIntervalMs ?? defaultRefreshIntervalMs,
      };
    } catch {
      return null;
    }
  });

  const [isConnecting, setIsConnecting] = useState(false);
  const [connectionError, setConnectionError] = useState<string | null>(null);

  const connect = useCallback(async (connection: ClusterConnection) => {
    setIsConnecting(true);
    setConnectionError(null);
    try {
      let kubeconfigPath = connection.kubeconfigPath ?? "";
      let context = connection.context ?? "";
      if (connection.source === "eks") {
        const prepared = await prepareEKSConnection({
          kubeconfigPath,
          clusterName: connection.clusterName ?? "",
          region: connection.region ?? "",
          profile: connection.profile ?? "",
          roleArn: connection.roleArn ?? "",
        });
        kubeconfigPath = prepared.kubeconfigPath;
        context = prepared.context;
      }
      const result = await testClusterConnection(kubeconfigPath, context);

      const connectedCluster: ClusterConnection = {
        ...normalizeEKSConnection(connection),
        kubeconfigPath,
        context,
        connectedAt: result.connectedAt,
        server: result.server,
        version: result.version,
      };

      return connectedCluster;
    } catch (error) {
      const message = error instanceof Error ? error.message : "Failed to connect to cluster";
      setConnectionError(message);
      throw error;
    } finally {
      setIsConnecting(false);
    }
  }, []);

  const confirmConnection = useCallback((connection: ClusterConnection) => {
    setClusterConnection(connection);
    localStorage.setItem("cluster-connection", JSON.stringify(connection));
    setConnectionError(null);
  }, []);

  const updateConnection = useCallback((updates: Partial<Pick<ClusterConnection, "refreshIntervalMs">>) => {
    setClusterConnection((current) => {
      if (!current) return current;
      const updated = { ...current, ...updates };
      localStorage.setItem("cluster-connection", JSON.stringify(updated));
      return updated;
    });
  }, []);

  const disconnect = useCallback(() => {
    setClusterConnection(null);
    localStorage.removeItem("cluster-connection");
    setConnectionError(null);
  }, []);

  const isConnected = useCallback(() => {
    return clusterConnection !== null;
  }, [clusterConnection]);

  return (
    <ClusterContext.Provider
      value={{
        clusterConnection,
        isConnecting,
        connectionError,
        connect,
        confirmConnection,
        updateConnection,
        disconnect,
        isConnected,
      }}
    >
      {children}
    </ClusterContext.Provider>
  );
}

function normalizeEKSConnection(connection: ClusterConnection): ClusterConnection {
  if (connection.provider !== "aws-eks" && connection.source !== "eks") return connection;

  const contextParts = (connection.context ?? "").match(/^eks\/([^/]+)\/([^/]+)$/);
  return {
    ...connection,
    provider: "aws-eks",
    clusterName: connection.clusterName || contextParts?.[1],
    region: connection.region || contextParts?.[2],
  };
}

export function useCluster() {
  const context = useContext(ClusterContext);
  if (context === undefined) {
    throw new Error("useCluster must be used within a ClusterProvider");
  }
  return context;
}

import React, { createContext, useContext, useState, useCallback, ReactNode } from "react";
import { testClusterConnection } from "./backend";

export interface ClusterConnection {
  name: string;
  source: "kubeconfig" | "manual";
  kubeconfigPath?: string;
  context?: string;
  apiServerUrl?: string;
  namespace: string;
  readOnly: boolean;
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
  disconnect: () => void;
  isConnected: () => boolean;
}

const ClusterContext = createContext<ClusterContextType | undefined>(undefined);

export function ClusterProvider({ children }: { children: ReactNode }) {
  const [clusterConnection, setClusterConnection] = useState<ClusterConnection | null>(() => {
    // Try to restore connection from localStorage
    try {
      const stored = localStorage.getItem("cluster-connection");
      return stored ? JSON.parse(stored) : null;
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
      const result = await testClusterConnection(connection.kubeconfigPath ?? "", connection.context ?? "");

      const connectedCluster: ClusterConnection = {
        ...connection,
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
        disconnect,
        isConnected,
      }}
    >
      {children}
    </ClusterContext.Provider>
  );
}

export function useCluster() {
  const context = useContext(ClusterContext);
  if (context === undefined) {
    throw new Error("useCluster must be used within a ClusterProvider");
  }
  return context;
}

import React, { createContext, useContext, useState, useCallback, ReactNode } from "react";

export interface ClusterConnection {
  name: string;
  source: "kubeconfig" | "manual";
  kubeconfigPath?: string;
  context?: string;
  apiServerUrl?: string;
  namespace: string;
  readOnly: boolean;
  connectedAt: number;
}

interface ClusterContextType {
  clusterConnection: ClusterConnection | null;
  isConnecting: boolean;
  connectionError: string | null;
  connect: (connection: ClusterConnection) => Promise<void>;
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
      // In a real implementation, you would validate the connection here
      // For now, we'll simulate a successful connection after a short delay
      await new Promise((resolve) => setTimeout(resolve, 500));

      const connectedCluster: ClusterConnection = {
        ...connection,
        connectedAt: Date.now(),
      };

      setClusterConnection(connectedCluster);
      localStorage.setItem("cluster-connection", JSON.stringify(connectedCluster));
    } catch (error) {
      const message = error instanceof Error ? error.message : "Failed to connect to cluster";
      setConnectionError(message);
      throw error;
    } finally {
      setIsConnecting(false);
    }
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

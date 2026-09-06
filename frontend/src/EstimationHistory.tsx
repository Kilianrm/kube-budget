import React, { createContext, useContext, useEffect, useState, useCallback, ReactNode } from "react";
import { ManifestResult } from "./backend";

export interface SavedEstimation {
  id: string;
  name: string;
  timestamp: number;
  estimationMode: "manual" | "cluster";
  provider?: string;
  region?: string;
  instanceType?: string;
  clusterName?: string;
  monthlyCost: number;
  currency: string;
  manifest: string;
  fullResult: ManifestResult;
}

interface EstimationHistoryContextType {
  estimations: SavedEstimation[];
  saveEstimation: (estimation: Omit<SavedEstimation, "id" | "timestamp">) => void;
  deleteEstimation: (id: string) => void;
  getEstimation: (id: string) => SavedEstimation | undefined;
  clearAll: () => void;
}

const EstimationHistoryContext = createContext<EstimationHistoryContextType | undefined>(undefined);

export function EstimationHistoryProvider({ children }: { children: ReactNode }) {
  const [estimations, setEstimations] = useState<SavedEstimation[]>(() => {
    try {
      const stored = localStorage.getItem("estimation-history");
      return stored ? JSON.parse(stored) : [];
    } catch {
      return [];
    }
  });

  useEffect(() => {
    localStorage.setItem("estimation-history", JSON.stringify(estimations));
  }, [estimations]);

  const saveEstimation = useCallback((estimation: Omit<SavedEstimation, "id" | "timestamp">) => {
      const newEstimation: SavedEstimation = {
        ...estimation,
        id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
        timestamp: Date.now(),
      };

      setEstimations((current) => [newEstimation, ...current].slice(0, 50));
    }, []);

  const deleteEstimation = useCallback((id: string) => {
    setEstimations((current) => current.filter((estimation) => estimation.id !== id));
  }, []);

  const getEstimation = useCallback(
    (id: string) => estimations.find((e) => e.id === id),
    [estimations]
  );

  const clearAll = useCallback(() => {
    setEstimations([]);
  }, []);

  return (
    <EstimationHistoryContext.Provider value={{ estimations, saveEstimation, deleteEstimation, getEstimation, clearAll }}>
      {children}
    </EstimationHistoryContext.Provider>
  );
}

export function useEstimationHistory() {
  const context = useContext(EstimationHistoryContext);
  if (!context) {
    throw new Error("useEstimationHistory must be used within EstimationHistoryProvider");
  }
  return context;
}

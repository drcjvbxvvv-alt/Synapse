import { create } from 'zustand';
import type { Cluster } from '../types';

// ── Cluster Store ────────────────────────────────────────────────────────────
//
// Single source of truth for the currently selected cluster across the app.
// ClusterSelector writes here; any component that needs the active cluster ID
// can read from this store instead of parsing URL params independently.

interface ClusterState {
  activeClusterId: string | null;
  clusters: Cluster[];

  // Actions
  setActiveClusterId: (id: string | null) => void;
  setClusters: (clusters: Cluster[]) => void;
}

export const useClusterStore = create<ClusterState>((set) => ({
  activeClusterId: null,
  clusters: [],

  setActiveClusterId: (id) => set({ activeClusterId: id }),
  setClusters: (clusters) => set({ clusters }),
}));

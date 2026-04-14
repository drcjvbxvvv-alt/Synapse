import { create } from 'zustand';
import type { Cluster } from '../types';

// ── Cluster Store ────────────────────────────────────────────────────────────
//
// Single source of truth for the currently selected cluster across the app.
// ClusterSelector writes here; any component that needs the active cluster ID
// can read from this store instead of parsing URL params independently.
//
// ALWAYS use the exported selector constants rather than inline arrow functions
// or bare useClusterStore() — stable selector references prevent re-renders
// when unrelated state fields (e.g. future pipelineRuns) are added to the store.
//
//   ✅  const clusterId = useClusterStore(selectActiveClusterId);
//   ❌  const { activeClusterId } = useClusterStore();   // subscribes to entire store

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

// ── Selectors ────────────────────────────────────────────────────────────────
// Stable function references — pass these to useClusterStore() to subscribe
// only to the slice you need and avoid spurious re-renders.

export const selectActiveClusterId  = (s: ClusterState) => s.activeClusterId;
export const selectClusters         = (s: ClusterState) => s.clusters;
export const selectSetActiveClusterId = (s: ClusterState) => s.setActiveClusterId;
export const selectSetClusters      = (s: ClusterState) => s.setClusters;

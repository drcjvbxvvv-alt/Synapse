import { create } from 'zustand';
import { persist } from 'zustand/middleware';

// ── UI Store ─────────────────────────────────────────────────────────────────
//
// Persistent UI preferences. Survives page refresh via localStorage.
// Intentionally scoped to layout preferences only — not server data.

interface UIState {
  sidebarCollapsed: boolean;

  // Actions
  setSidebarCollapsed: (collapsed: boolean) => void;
  toggleSidebar: () => void;
}

export const useUIStore = create<UIState>()(
  persist(
    (set) => ({
      sidebarCollapsed: false,

      setSidebarCollapsed: (collapsed) => set({ sidebarCollapsed: collapsed }),
      toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
    }),
    {
      name: 'synapse-ui',
      partialize: (state) => ({ sidebarCollapsed: state.sidebarCollapsed }),
    },
  ),
);

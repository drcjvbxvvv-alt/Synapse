import { create } from 'zustand';
import type { User } from '../types';

// ── Session Store ───────────────────────────────────────────────────────────
//
// Reactive wrapper over tokenManager's module-level variables.
// Components that need to react to auth state changes should subscribe here
// instead of polling tokenManager directly.
//
// NOTE: accessToken itself is NOT stored here (it lives in tokenManager's
// module scope for XSS safety). This store only tracks the reactive
// session metadata: who is logged in and whether the session is valid.

interface SessionState {
  user: User | null;
  isLoggedIn: boolean;

  // Actions
  setSession: (user: User, expiresAt: number) => void;
  clearSession: () => void;
}

export const useSessionStore = create<SessionState>((set) => ({
  user: null,
  isLoggedIn: false,

  setSession: (user, _expiresAt) => {
    set({ user, isLoggedIn: true });
  },

  clearSession: () => {
    set({ user: null, isLoggedIn: false });
  },
}));

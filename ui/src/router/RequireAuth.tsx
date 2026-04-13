import React from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { tokenManager } from '../services/authService';

const ENTRY_KEY = 'synapse_app_entered';

interface RequireAuthProps {
  children: React.ReactNode;
}

export const RequireAuth: React.FC<RequireAuthProps> = ({ children }) => {
  const location = useLocation();

  if (!tokenManager.isLoggedIn()) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  // Enforce root-entry policy: users must enter the app through "/" first.
  // Direct deep-link access (e.g. bookmarked /clusters/10/network) is blocked
  // and redirected to "/" so the full app shell initialises correctly.
  // sessionStorage is tab-scoped, so opening a new tab always requires
  // re-entry through "/".
  const hasEntered = !!sessionStorage.getItem(ENTRY_KEY);

  if (!hasEntered) {
    if (location.pathname === '/') {
      sessionStorage.setItem(ENTRY_KEY, '1');
    } else {
      return <Navigate to="/" replace />;
    }
  }

  return <>{children}</>;
};

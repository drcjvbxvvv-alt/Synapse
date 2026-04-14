import { useEffect, useRef } from 'react';

/**
 * Runs callback on an interval, but skips execution while the tab is hidden.
 * Equivalent to React Query's refetchIntervalInBackground: false.
 *
 * @param callback - Function to call each tick (captured via ref — always fresh)
 * @param delay    - Interval in ms; pass null to pause the timer entirely
 */
export function useVisibilityInterval(callback: () => void, delay: number | null): void {
  const savedCallback = useRef(callback);
  savedCallback.current = callback;

  useEffect(() => {
    if (delay === null) return;
    const id = setInterval(() => {
      if (!document.hidden) {
        savedCallback.current();
      }
    }, delay);
    return () => clearInterval(id);
  }, [delay]);
}

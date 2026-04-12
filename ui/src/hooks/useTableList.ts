import { useState, useMemo } from 'react';

// ─── Types ─────────────────────────────────────────────────────────────────

export interface UseTableListOptions<T> {
  /** Raw data from useQuery */
  data: T[] | undefined;
  /** Loading state from useQuery */
  isLoading: boolean;
  /** refetch function from useQuery */
  refetch: () => void;
  /**
   * Keys of T to match against search text (case-insensitive substring).
   * Mutually exclusive with searchFn.
   */
  searchKeys?: (keyof T)[];
  /**
   * Custom search predicate. Takes priority over searchKeys.
   */
  searchFn?: (item: T, searchText: string) => boolean;
}

export interface UseTableListReturn<T> {
  /** Debounce-free search text state */
  searchText: string;
  setSearchText: (text: string) => void;
  /** Data filtered by searchText */
  filtered: T[];
  isLoading: boolean;
  refetch: () => void;
}

// ─── Hook ──────────────────────────────────────────────────────────────────

/**
 * useTableList — shared state + client-side filtering for list pages.
 *
 * Usage:
 *   const { searchText, setSearchText, filtered, isLoading, refetch } = useTableList({
 *     data,
 *     isLoading,
 *     refetch,
 *     searchKeys: ['name', 'namespace'],
 *   });
 */
export function useTableList<T>({
  data,
  isLoading,
  refetch,
  searchKeys,
  searchFn,
}: UseTableListOptions<T>): UseTableListReturn<T> {
  const [searchText, setSearchText] = useState('');

  const filtered = useMemo(() => {
    const items = data ?? [];
    if (!searchText.trim()) return items;
    const lower = searchText.toLowerCase();

    if (searchFn) {
      return items.filter((item) => searchFn(item, lower));
    }

    if (searchKeys && searchKeys.length > 0) {
      return items.filter((item) =>
        searchKeys.some((key) => {
          const val = item[key];
          return typeof val === 'string' && val.toLowerCase().includes(lower);
        }),
      );
    }

    return items;
  }, [data, searchText, searchKeys, searchFn]);

  return { searchText, setSearchText, filtered, isLoading, refetch };
}

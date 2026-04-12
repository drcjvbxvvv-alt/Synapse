import { useState, useCallback } from 'react';

export interface MultiSearchCondition {
  field: string;
  value: string;
}

export interface UseMultiSearchReturn {
  conditions: MultiSearchCondition[];
  currentField: string;
  currentValue: string;
  setCurrentField: (field: string) => void;
  setCurrentValue: (value: string) => void;
  addCondition: () => void;
  removeCondition: (index: number) => void;
  clearAll: () => void;
}

/**
 * Generic multi-condition search state hook.
 * Works with any field type — pair with applyMultiSearch() for filtering.
 */
export function useMultiSearch(initialField: string): UseMultiSearchReturn {
  const [conditions, setConditions] = useState<MultiSearchCondition[]>([]);
  const [currentField, setCurrentField] = useState(initialField);
  const [currentValue, setCurrentValue] = useState('');

  const addCondition = useCallback(() => {
    const trimmed = currentValue.trim();
    if (!trimmed) return;
    setConditions(prev => [...prev, { field: currentField, value: trimmed }]);
    setCurrentValue('');
  }, [currentField, currentValue]);

  const removeCondition = useCallback((index: number) => {
    setConditions(prev => prev.filter((_, i) => i !== index));
  }, []);

  const clearAll = useCallback(() => {
    setConditions([]);
    setCurrentValue('');
  }, []);

  return {
    conditions,
    currentField,
    currentValue,
    setCurrentField,
    setCurrentValue,
    addCondition,
    removeCondition,
    clearAll,
  };
}

/**
 * Applies multi-condition filtering to a list.
 *
 * Conditions are grouped by field; within each group at least one value
 * must match (OR); all field groups must match (AND).
 *
 * @param items        The full list to filter.
 * @param conditions   Active search conditions from useMultiSearch.
 * @param getFieldValue  Maps (item, fieldName) → the string to search against.
 */
export function applyMultiSearch<T>(
  items: T[],
  conditions: MultiSearchCondition[],
  getFieldValue: (item: T, field: string) => string,
): T[] {
  if (conditions.length === 0) return items;

  const grouped = conditions.reduce((acc, c) => {
    (acc[c.field] = acc[c.field] ?? []).push(c.value.toLowerCase());
    return acc;
  }, {} as Record<string, string[]>);

  return items.filter(item =>
    Object.entries(grouped).every(([field, values]) => {
      const fieldVal = getFieldValue(item, field).toLowerCase();
      return values.some(v => fieldVal.includes(v));
    }),
  );
}

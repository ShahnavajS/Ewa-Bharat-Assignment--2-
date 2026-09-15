import { useState, useEffect, useCallback } from 'react';
import { getWindows } from '../api/windows';
import { Window, ApiError } from '../types';

export function useWindows() {
  const [windows, setWindows] = useState<Window[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<ApiError | null>(null);

  const fetchWindows = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await getWindows();
      setWindows(data || []);
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        setError(err);
      } else {
        setError(new ApiError('Failed to load display windows', 'UNKNOWN_ERROR', 500));
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchWindows();
  }, [fetchWindows]);

  return {
    windows,
    loading,
    error,
    refetch: fetchWindows,
  };
}

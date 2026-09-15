import { useState, useEffect, useCallback } from 'react';
import { getMedia } from '../api/media';
import { MediaItem, ApiError } from '../types';

export function useMedia() {
  const [media, setMedia] = useState<MediaItem[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<ApiError | null>(null);

  const fetchMedia = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await getMedia();
      setMedia(data || []);
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        setError(err);
      } else {
        setError(new ApiError('Failed to load media library', 'UNKNOWN_ERROR', 500));
      }
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchMedia();
  }, [fetchMedia]);

  return {
    media,
    loading,
    error,
    refetch: fetchMedia,
  };
}

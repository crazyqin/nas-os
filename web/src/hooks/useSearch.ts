import { useState, useCallback, useEffect } from 'react';

interface SearchResult {
  id: string;
  type: 'file' | 'user' | 'setting' | 'api' | 'container' | 'share';
  title: string;
  description: string;
  path?: string;
  icon?: string;
  metadata?: Record<string, string | number>;
}

interface SearchResponse {
  results: SearchResult[];
  total: number;
  query: string;
  type?: string;
}

interface UseSearchResult {
  results: SearchResult[];
  query: string;
  loading: boolean;
  error: Error | null;
  search: (q: string, type?: string) => Promise<void>;
  clear: () => void;
}

export function useSearch(): UseSearchResult {
  const [results, setResults] = useState<SearchResult[]>([]);
  const [query, setQuery] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const search = useCallback(async (q: string, type?: string) => {
    if (!q.trim()) {
      setResults([]);
      setQuery('');
      return;
    }

    setLoading(true);
    setError(null);
    setQuery(q);

    try {
      const url = new URL('/api/v1/search', window.location.origin);
      url.searchParams.set('q', q);
      if (type) {
        url.searchParams.set('type', type);
      }

      const response = await fetch(url.toString());
      if (!response.ok) {
        throw new Error(`HTTP error: ${response.status}`);
      }

      const data: SearchResponse = await response.json();
      setResults(data.results);
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Search failed'));
      setResults([]);
    } finally {
      setLoading(false);
    }
  }, []);

  const clear = useCallback(() => {
    setResults([]);
    setQuery('');
    setError(null);
  }, []);

  return {
    results,
    query,
    loading,
    error,
    search,
    clear
  };
}

// Quick search for autocomplete/suggestions
interface UseQuickSearchResult {
  suggestions: SearchResult[];
  loading: boolean;
  quickSearch: (q: string) => Promise<void>;
}

export function useQuickSearch(): UseQuickSearchResult {
  const [suggestions, setSuggestions] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);

  const quickSearch = useCallback(async (q: string) => {
    if (!q.trim() || q.length < 2) {
      setSuggestions([]);
      return;
    }

    setLoading(true);

    try {
      const url = new URL('/api/v1/search/quick', window.location.origin);
      url.searchParams.set('q', q);

      const response = await fetch(url.toString());
      if (!response.ok) {
        throw new Error(`HTTP error: ${response.status}`);
      }

      const data: SearchResponse = await response.json();
      setSuggestions(data.results.slice(0, 5)); // Limit to 5 suggestions
    } catch {
      setSuggestions([]);
    } finally {
      setLoading(false);
    }
  }, []);

  // Debounce quick search
  useEffect(() => {
    const timeout = setTimeout(() => {
      // quickSearch would be called here with debounced query
    }, 300);
    return () => clearTimeout(timeout);
  }, []);

  return {
    suggestions,
    loading,
    quickSearch
  };
}
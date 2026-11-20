import { useFocusEffect } from "expo-router";
import { useCallback, useRef, useState } from "react";

import { errorMessage } from "@/lib/errors";
import { onRefreshRequest } from "@/lib/refresh";

/** The most the server returns per request (see the transaction list in the API docs). */
const MAX_PAGE_SIZE = 500;

interface PagedState<T> {
  items?: T[];
  /** True while the last page was full, so there may be more. */
  hasMore: boolean;
  loading: boolean;
  loadingMore: boolean;
  error?: string;
}

/**
 * Like useLoader, but for long lists the server hands out in pages. The first
 * page loads when the screen comes into view; loadMore appends the next one.
 * A reload fetches as many entries as are already shown (up to the server's
 * maximum), so the list does not shrink back after returning from an edit.
 * `fetchPage` must be wrapped in useCallback, otherwise it would reload on every render.
 */
export function usePagedLoader<T extends { id: string }>(
  fetchPage: (limit: number, offset: number) => Promise<T[]>,
  pageSize = 100,
) {
  const [state, setState] = useState<PagedState<T>>({ hasMore: false, loading: true, loadingMore: false });
  // The reload callback must keep its identity (useFocusEffect reruns on a new one), so it reads the shown count from a ref.
  const shown = useRef(0);
  // One request at a time, so a reload and "load more" cannot overwrite each other.
  const busy = useRef(false);

  const reload = useCallback(async () => {
    if (busy.current) {
      return;
    }
    busy.current = true;
    setState((previous) => ({ ...previous, loading: true, error: undefined }));
    try {
      const limit = Math.min(Math.max(shown.current, pageSize), MAX_PAGE_SIZE);
      const items = await fetchPage(limit, 0);
      shown.current = items.length;
      setState({ items, hasMore: items.length === limit, loading: false, loadingMore: false });
    } catch (error) {
      setState((previous) => ({ ...previous, error: errorMessage(error), loading: false }));
    } finally {
      busy.current = false;
    }
  }, [fetchPage, pageSize]);

  const loadMore = useCallback(async () => {
    if (busy.current) {
      return;
    }
    busy.current = true;
    setState((previous) => ({ ...previous, loadingMore: true, error: undefined }));
    try {
      const page = await fetchPage(pageSize, shown.current);
      setState((previous) => {
        // New bookings made meanwhile shift the offsets, so the same entry can arrive twice.
        const known = new Set(previous.items?.map((item) => item.id));
        const items = [...(previous.items ?? []), ...page.filter((item) => !known.has(item.id))];
        shown.current = items.length;
        return { items, hasMore: page.length === pageSize, loading: false, loadingMore: false };
      });
    } catch (error) {
      setState((previous) => ({ ...previous, error: errorMessage(error), loadingMore: false }));
    } finally {
      busy.current = false;
    }
  }, [fetchPage, pageSize]);

  useFocusEffect(
    useCallback(() => {
      void reload();
      return onRefreshRequest(() => void reload());
    }, [reload]),
  );

  return { ...state, reload, loadMore };
}

import { useFocusEffect } from "expo-router";
import { useCallback, useState } from "react";

import { errorMessage } from "@/lib/errors";
import { onRefreshRequest } from "@/lib/refresh";

interface LoaderState<T> {
  data?: T;
  error?: string;
  loading: boolean;
}

/**
 * Loads data whenever the screen comes into view, so a list is fresh again
 * after returning from a screen that changed it. While the screen is in view,
 * it also reloads on requestRefresh (refresh button, app back in front), so
 * hidden screens further down the stack do not send needless requests.
 * `load` must be wrapped in useCallback, otherwise it would reload on every render.
 */
export function useLoader<T>(load: () => Promise<T>) {
  const [state, setState] = useState<LoaderState<T>>({ loading: true });

  const reload = useCallback(async () => {
    setState((previous) => ({ ...previous, loading: true, error: undefined }));
    try {
      setState({ data: await load(), loading: false });
    } catch (error) {
      setState((previous) => ({ ...previous, error: errorMessage(error), loading: false }));
    }
  }, [load]);

  useFocusEffect(
    useCallback(() => {
      void reload();
      return onRefreshRequest(() => void reload());
    }, [reload]),
  );

  return { ...state, reload };
}

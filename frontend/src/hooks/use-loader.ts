import { useFocusEffect } from "expo-router";
import { useCallback, useState } from "react";

import { errorMessage } from "@/lib/errors";

interface LoaderState<T> {
  data?: T;
  error?: string;
  loading: boolean;
}

/**
 * Loads data whenever the screen comes into view, so a list is fresh again
 * after returning from a screen that changed it. `load` must be wrapped in
 * useCallback, otherwise it would reload on every render.
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
    }, [reload]),
  );

  return { ...state, reload };
}

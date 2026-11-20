import { useEffect } from "react";
import { AppState, Platform } from "react-native";

import { requestRefresh } from "@/lib/refresh";

/**
 * Reloads the visible screen when the user comes back to the app: the app
 * returns from the background, or on the web the tab becomes visible again or
 * the window gets the focus (e.g. two windows side by side).
 */
export function useRefreshOnReturn() {
  useEffect(() => {
    const subscription = AppState.addEventListener("change", (state) => {
      if (state === "active") {
        requestRefresh();
      }
    });
    if (Platform.OS !== "web") {
      return () => subscription.remove();
    }
    window.addEventListener("focus", requestRefresh);
    return () => {
      subscription.remove();
      window.removeEventListener("focus", requestRefresh);
    };
  }, []);
}

import type { ReactNode } from "react";
import { ScrollView } from "react-native";

/** Scrollable page with the app background and a readable maximum width on large screens. */
export function Screen({ children }: { children: ReactNode }) {
  return (
    <ScrollView
      className="flex-1 bg-background"
      contentContainerClassName="w-full max-w-xl self-center gap-4 p-4"
      keyboardShouldPersistTaps="handled">
      {children}
    </ScrollView>
  );
}

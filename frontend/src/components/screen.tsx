import { useIsFocused } from "expo-router";
import type { ReactNode } from "react";
import { ScrollView, View } from "react-native";

import { cn } from "@/lib/utils";

/**
 * Scrollable page with the app background and a readable maximum width on
 * large screens. `floating` is a button that stays in place above the content,
 * such as "Add transaction"; the content gets room at the end so the button
 * never covers the last row.
 */
export function Screen({ children, floating }: { children: ReactNode; floating?: ReactNode }) {
  // On the web, screens that are not shown (other tabs, screens further back)
  // stay in the page. The navigator marks them "inert", which not every tool
  // understands; hiding them keeps them out of reach of screen readers and
  // automated tests alike, and unlike display:none it keeps the scroll position.
  const focused = useIsFocused();
  return (
    <View className={cn("flex-1 bg-background", !focused && "web:invisible")}>
      <ScrollView
        className="flex-1"
        contentContainerClassName={cn("w-full max-w-xl self-center gap-6 px-4 pt-4", floating ? "pb-28" : "pb-10")}
        keyboardShouldPersistTaps="handled">
        {children}
      </ScrollView>
      {floating && (
        <View pointerEvents="box-none" className="absolute bottom-5 left-0 right-0 w-full max-w-xl items-end self-center px-4">
          {floating}
        </View>
      )}
    </View>
  );
}

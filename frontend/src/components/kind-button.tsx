import type { ReactNode } from "react";
import { Pressable, View } from "react-native";

import { Text } from "@/components/ui/text";
import { cn } from "@/lib/utils";

/**
 * A segmented control: a row of options of which exactly one is chosen, such
 * as expense/income. Wrap the KindButtons in a Segmented.
 */
export function Segmented({ children, className }: { children: ReactNode; className?: string }) {
  return <View className={cn("flex-row gap-1 rounded-xl bg-muted p-1", className)}>{children}</View>;
}

/** One option of a Segmented control. */
export function KindButton({ label, active, onPress }: { label: string; active: boolean; onPress: () => void }) {
  return (
    <Pressable
      role="button"
      aria-selected={active}
      onPress={onPress}
      className={cn(
        "h-10 flex-1 items-center justify-center rounded-lg px-2",
        active ? "bg-card shadow-sm shadow-black/10" : "active:bg-background/60 web:hover:bg-background/40",
      )}>
      <Text className={cn("text-sm font-semibold", active ? "text-foreground" : "text-muted-foreground")} numberOfLines={1}>
        {label}
      </Text>
    </Pressable>
  );
}

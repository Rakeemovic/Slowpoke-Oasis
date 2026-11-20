import type { ReactNode } from "react";
import { View } from "react-native";

import { ThemeToggle } from "@/components/theme-toggle";
import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";
import { requestRefresh } from "@/lib/refresh";

/**
 * Header buttons for screens that show shared data: refresh, the scheme
 * toggle and any extra buttons of the screen. Refresh picks up bookings that
 * other household members made while the screen was open.
 */
export function HeaderActions({ children }: { children?: ReactNode }) {
  return (
    <View className="flex-row">
      <Button variant="ghost" size="sm" onPress={requestRefresh}>
        <Text>Refresh</Text>
      </Button>
      <ThemeToggle />
      {children}
    </View>
  );
}

import { RefreshCw } from "lucide-react-native";
import type { ReactNode } from "react";
import { View } from "react-native";

import { ThemeToggle } from "@/components/theme-toggle";
import { Button } from "@/components/ui/button";
import { Icon } from "@/components/ui/icon";
import { requestRefresh } from "@/lib/refresh";

/**
 * Header buttons for screens that show shared data: refresh, the scheme
 * toggle and any extra buttons of the screen. Refresh picks up bookings that
 * other household members made while the screen was open. The buttons show
 * icons only, so each has a name for screen readers.
 */
export function HeaderActions({ children }: { children?: ReactNode }) {
  return (
    <View className="flex-row items-center">
      <HeaderIconButton icon={RefreshCw} label="Refresh" onPress={requestRefresh} />
      <ThemeToggle />
      {children}
    </View>
  );
}

/** A round icon button for the navigation header, large enough to hit with a thumb. */
export function HeaderIconButton({
  icon,
  label,
  onPress,
}: {
  icon: React.ComponentProps<typeof Icon>["as"];
  label: string;
  onPress?: () => void;
}) {
  return (
    <Button variant="ghost" size="icon" onPress={onPress} aria-label={label}>
      <Icon as={icon} size={20} />
    </Button>
  );
}

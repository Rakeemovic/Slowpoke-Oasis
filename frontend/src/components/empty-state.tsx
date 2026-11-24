import type { LucideIcon } from "lucide-react-native";
import type { ReactNode } from "react";
import { View } from "react-native";

import { IconTile } from "@/components/icon-tile";
import { Text } from "@/components/ui/text";
import type { Hue } from "@/lib/hues";

const EMPTY_HUE: Hue = { tile: "bg-accent", icon: "text-accent-foreground", fill: "bg-primary" };

/**
 * Shown instead of an empty list: says what would be here and how to get
 * there, so a new user is never left in front of a blank screen.
 */
export function EmptyState({
  icon,
  title,
  text,
  action,
}: {
  icon: LucideIcon;
  title: string;
  text?: string;
  action?: ReactNode;
}) {
  return (
    <View className="items-center gap-3 rounded-2xl border border-dashed border-border px-6 py-8">
      <IconTile icon={icon} hue={EMPTY_HUE} size="lg" />
      <View className="items-center gap-1">
        <Text className="font-heading text-base">{title}</Text>
        {text && (
          <Text variant="muted" className="text-center">
            {text}
          </Text>
        )}
      </View>
      {action}
    </View>
  );
}

import { Check, type LucideIcon } from "lucide-react-native";
import type { ReactNode } from "react";
import { Pressable, View } from "react-native";

import { Icon } from "@/components/ui/icon";
import { Text } from "@/components/ui/text";
import type { Hue } from "@/lib/hues";
import { cn } from "@/lib/utils";

export interface Choice {
  id: string;
  label: string;
  icon?: LucideIcon;
  hue?: Hue;
}

/**
 * A wrap of chips to pick one item, such as a category or a target ledger.
 * The chosen chip is filled and carries a check mark, so the choice does not
 * depend on colour alone. `extra` adds a chip at the end, e.g. "New category".
 */
export function ChoicePicker({
  choices,
  selected,
  onSelect,
  emptyText,
  extra,
}: {
  choices: Choice[];
  selected?: string;
  onSelect: (id: string) => void;
  emptyText: string;
  extra?: ReactNode;
}) {
  return (
    <View className="gap-2">
      {choices.length === 0 && <Text variant="muted">{emptyText}</Text>}
      <View className="flex-row flex-wrap gap-2">
        {choices.map((c) => {
          const active = c.id === selected;
          return (
            <Pressable
              key={c.id}
              role="button"
              aria-selected={active}
              aria-label={c.label}
              onPress={() => onSelect(c.id)}
              className={cn(
                "h-10 flex-row items-center gap-1.5 rounded-full border px-3.5",
                active ? "border-primary bg-primary" : "border-border bg-card active:bg-accent web:hover:bg-accent/60",
              )}>
              {active ? (
                <Icon as={Check} size={16} className="text-primary-foreground" />
              ) : (
                c.icon && <Icon as={c.icon} size={16} className={c.hue?.icon ?? "text-muted-foreground"} />
              )}
              <Text className={cn("text-sm font-medium", active && "text-primary-foreground")}>{c.label}</Text>
            </Pressable>
          );
        })}
        {extra}
      </View>
    </View>
  );
}

/** A dashed chip that starts something new, placed at the end of a ChoicePicker. */
export function NewChoiceChip({ label, icon, onPress }: { label: string; icon: LucideIcon; onPress: () => void }) {
  return (
    <Pressable
      role="button"
      onPress={onPress}
      className="h-10 flex-row items-center gap-1.5 rounded-full border border-dashed border-input px-3.5 active:bg-accent web:hover:bg-accent/60">
      <Icon as={icon} size={16} className="text-muted-foreground" />
      <Text className="text-sm font-medium text-muted-foreground">{label}</Text>
    </Pressable>
  );
}

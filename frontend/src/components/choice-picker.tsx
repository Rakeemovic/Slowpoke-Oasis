import { View } from "react-native";

import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";

export interface Choice {
  id: string;
  label: string;
}

/** A row of buttons to pick one item, such as a category or a target ledger. */
export function ChoicePicker({
  choices,
  selected,
  onSelect,
  emptyText,
}: {
  choices: Choice[];
  selected?: string;
  onSelect: (id: string) => void;
  emptyText: string;
}) {
  if (choices.length === 0) {
    return <Text variant="muted">{emptyText}</Text>;
  }
  return (
    <View className="flex-row flex-wrap gap-2">
      {choices.map((c) => (
        <Button key={c.id} size="sm" variant={c.id === selected ? "default" : "outline"} onPress={() => onSelect(c.id)}>
          <Text>{c.label}</Text>
        </Button>
      ))}
    </View>
  );
}

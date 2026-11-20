import { ActivityIndicator, View } from "react-native";

import type { Pot } from "@/api/types";
import { ChoicePicker } from "@/components/choice-picker";
import { ErrorMessage } from "@/components/error-message";
import { Label } from "@/components/ui/label";
import { defaultPot, potLabel, selectablePots } from "@/lib/pots";

/**
 * Lets the user pick a pot. With only the default pot there is nothing to
 * choose, so the picker stays out of the way. Nothing selected means the
 * default pot.
 */
export function PotPicker({
  label,
  pots,
  selected,
  onSelect,
  loading,
  error,
  onRetry,
}: {
  label: string;
  pots?: Pot[];
  selected?: string;
  onSelect: (id: string) => void;
  loading: boolean;
  error?: string;
  onRetry: () => void;
}) {
  if (!pots) {
    return (
      <View className="gap-2">
        {loading && <ActivityIndicator />}
        <ErrorMessage message={error} onRetry={onRetry} />
      </View>
    );
  }
  const choices = selectablePots(pots, selected);
  if (choices.length < 2) {
    return null;
  }
  return (
    <View className="gap-2">
      <Label>{label}</Label>
      <ChoicePicker
        choices={choices.map((p) => ({ id: p.id, label: potLabel(p) }))}
        selected={selected ?? defaultPot(pots)?.id}
        onSelect={onSelect}
        emptyText=""
      />
    </View>
  );
}

import { View } from "react-native";

import { KindButton } from "@/components/kind-button";
import { Text } from "@/components/ui/text";
import { multiplierLabel, roundUpMultipliers } from "@/lib/round-ups";

/**
 * Switches the round-up of a pot: off, or the multiplier (rule T10). The
 * hint says what it does and, when another pot collects now, that this pot
 * takes over from it, since a ledger has only one collecting pot.
 */
export function RoundUpChoice({
  value,
  onChange,
  replaces,
}: {
  value: number;
  onChange: (multiplier: number) => void;
  /** The name of the pot that collects now, if it is another one. */
  replaces?: string;
}) {
  return (
    <View className="gap-1.5">
      <Text className="text-sm font-medium">Round-ups</Text>
      <View className="flex-row gap-2">
        {roundUpMultipliers.map((m) => (
          <KindButton key={m} label={multiplierLabel(m)} active={value === m} onPress={() => onChange(m)} />
        ))}
      </View>
      <Text variant="muted" className="text-sm">
        Every expense from Main is rounded up to the next whole euro, times the factor, and the difference moves into
        this pot. Payments of recurring payments and whole euro amounts are not rounded up. It starts with today’s
        expenses.
      </Text>
      {value > 0 && replaces && (
        <Text variant="muted" className="text-sm">{`This replaces “${replaces}”, which collects the round-ups now.`}</Text>
      )}
    </View>
  );
}

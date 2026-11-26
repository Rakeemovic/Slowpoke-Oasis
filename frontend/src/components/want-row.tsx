import { Pressable, View } from "react-native";

import type { Category, Transaction } from "@/api/types";
import { Amount } from "@/components/amount";
import { IconTile } from "@/components/icon-tile";
import { Switch } from "@/components/ui/switch";
import { Text } from "@/components/ui/text";
import { formatDate } from "@/lib/dates";
import { hueFor, NEUTRAL_HUE } from "@/lib/hues";
import { categoryIcon } from "@/lib/icons";
import { transactionTitle } from "@/lib/transactions";

/** One want of the week with a switch for the spontaneous mark; the whole row toggles. */
export function WantRow({
  tx,
  categories,
  disabled,
  onChange,
}: {
  tx: Transaction;
  categories: Category[];
  disabled: boolean;
  onChange: (spontaneous: boolean) => void;
}) {
  const title = transactionTitle(tx, categories);
  const category = categories.find((c) => c.id === tx.category_id);
  const details = [formatDate(tx.booked_on), tx.note].filter(Boolean).join(" · ");
  return (
    <Pressable className="flex-row items-center gap-3 p-4" disabled={disabled} onPress={() => onChange(!tx.is_spontaneous)}>
      <IconTile icon={categoryIcon(title)} hue={category ? hueFor(category.id) : NEUTRAL_HUE} />
      <View className="flex-1 gap-0.5">
        <Text className="font-medium" numberOfLines={1}>
          {title}
        </Text>
        <Text variant="muted" className="text-sm" numberOfLines={1}>
          {details}
        </Text>
      </View>
      {/* Wants are expenses; here only the size of the amount matters. */}
      <Amount cents={Math.abs(tx.amount_cents)} />
      <Switch
        checked={tx.is_spontaneous}
        disabled={disabled}
        onCheckedChange={onChange}
        aria-label={`Spontaneous: ${details}`}
      />
    </Pressable>
  );
}

import { useCallback } from "react";
import { ActivityIndicator, View } from "react-native";

import { ledgers } from "@/api/endpoints";
import { ErrorMessage } from "@/components/error-message";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { today } from "@/lib/dates";
import { formatCents } from "@/lib/money";
import { cn } from "@/lib/utils";

/**
 * Shows what a ledger holds today: large the money available in the default
 * pot, small the total of all pots. Loads itself, so every place that shows a
 * ledger can drop it in. "large" is for the ledger screen, "compact" for lists.
 */
export function LedgerBalance({ ledgerId, size = "compact" }: { ledgerId: string; size?: "compact" | "large" }) {
  const { data, error, loading, reload } = useLoader(
    useCallback(() => ledgers.balance(ledgerId, today()), [ledgerId]),
  );

  if (loading && !data) {
    return <ActivityIndicator />;
  }
  if (!data) {
    return <ErrorMessage message={error} onRetry={reload} />;
  }
  const large = size === "large";
  return (
    <View className={cn(large ? "gap-1" : "items-end")}>
      {large && <Text variant="muted">Available today</Text>}
      <Text
        className={cn(
          "font-semibold",
          large ? "text-3xl" : "text-lg",
          data.available_cents < 0 && "text-destructive",
        )}>
        {formatCents(data.available_cents)}
      </Text>
      <Text variant="muted" className="text-sm">
        {formatCents(data.total_cents)} in all pots
      </Text>
    </View>
  );
}

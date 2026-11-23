import { useCallback, useEffect } from "react";
import { ActivityIndicator, View } from "react-native";

import { ledgers } from "@/api/endpoints";
import type { Balance } from "@/api/types";
import { ErrorMessage } from "@/components/error-message";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { today } from "@/lib/dates";
import { formatCents } from "@/lib/money";
import { cn } from "@/lib/utils";

/**
 * Shows what a ledger holds today: large the estimated account balance (all
 * pots minus due payments that are not booked yet), small the money in the
 * default pot. Loads itself, so every place that shows a ledger can drop it
 * in. "large" is for the ledger screen, "compact" for lists. A new `version`
 * loads the balance again, e.g. after a skipped payment.
 */
export function LedgerBalance({
  ledgerId,
  size = "compact",
  version = 0,
}: {
  ledgerId: string;
  size?: "compact" | "large";
  version?: number;
}) {
  const { data, error, loading, reload } = useLoader(
    useCallback(() => ledgers.balance(ledgerId, today()), [ledgerId]),
  );
  // The first load comes from useLoader; later versions load again in place.
  useEffect(() => {
    if (version > 0) {
      void reload();
    }
  }, [version, reload]);

  if (loading && !data) {
    return <ActivityIndicator />;
  }
  if (!data) {
    return <ErrorMessage message={error} onRetry={reload} />;
  }
  const large = size === "large";
  const estimated = formatCents(data.estimated_cents);
  return (
    <View className={cn(large ? "gap-1" : "items-end")}>
      {large && <Text variant="muted">Estimated account balance</Text>}
      <Text
        aria-label={`Estimated account balance: ${estimated}`}
        className={cn(
          "font-semibold",
          large ? "text-3xl" : "text-lg",
          data.estimated_cents < 0 && "text-destructive",
        )}>
        {estimated}
      </Text>
      {data.unbooked_count > 0 && (
        <Text variant="muted" className="text-sm">
          {unbookedHint(data)}
        </Text>
      )}
      <Text variant="muted" className="text-sm">
        {`${formatCents(data.available_cents)} in ${data.default_pot_name}`}
      </Text>
    </View>
  );
}

/** "incl. 2 unbooked payments (−45,00 €)" */
function unbookedHint(b: Balance): string {
  const payments = b.unbooked_count === 1 ? "payment" : "payments";
  return `incl. ${b.unbooked_count} unbooked ${payments} (${formatCents(-b.unbooked_cents)})`;
}

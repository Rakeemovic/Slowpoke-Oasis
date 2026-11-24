import { BadgeCheck, Landmark } from "lucide-react-native";
import { useCallback, useEffect } from "react";
import { View } from "react-native";

import { ledgers } from "@/api/endpoints";
import type { Balance } from "@/api/types";
import { Amount } from "@/components/amount";
import { ErrorMessage } from "@/components/error-message";
import { Icon } from "@/components/ui/icon";
import { Skeleton } from "@/components/ui/skeleton";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { formatDate, today } from "@/lib/dates";
import { formatCents } from "@/lib/money";

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
  const { data, error, loading, reload } = useLoader(useCallback(() => ledgers.balance(ledgerId, today()), [ledgerId]));
  // The first load comes from useLoader; later versions load again in place.
  useEffect(() => {
    if (version > 0) {
      void reload();
    }
  }, [version, reload]);

  if (loading && !data) {
    // A placeholder of the same size keeps the layout from jumping when the numbers arrive.
    return size === "large" ? (
      <View className="gap-2">
        <Skeleton className="h-3 w-40" />
        <Skeleton className="h-10 w-48" />
        <Skeleton className="h-4 w-32" />
      </View>
    ) : (
      <Skeleton className="h-6 w-20" />
    );
  }
  if (!data) {
    return <ErrorMessage message={error} onRetry={reload} />;
  }
  const estimated = formatCents(data.estimated_cents);
  const label = `Estimated account balance: ${estimated}`;
  if (size === "compact") {
    return (
      <View className="items-end">
        <Amount
          cents={data.estimated_cents}
          aria-label={label}
          tone={data.estimated_cents < 0 ? "negative" : "default"}
          className="font-display text-lg"
        />
        <Text variant="muted" className="text-xs">
          {`${formatCents(data.available_cents)} in ${data.default_pot_name}`}
        </Text>
      </View>
    );
  }
  return (
    <View className="gap-1">
      <Text variant="overline">Estimated account balance</Text>
      <Amount
        cents={data.estimated_cents}
        size="xl"
        animate
        aria-label={label}
        tone={data.estimated_cents < 0 ? "negative" : "default"}
      />
      <View className="flex-row flex-wrap gap-x-3 gap-y-1">
        <Text variant="muted">{`${formatCents(data.available_cents)} in ${data.default_pot_name}`}</Text>
        {data.unbooked_count > 0 && <Text className="text-sm font-medium text-warning">{unbookedHint(data)}</Text>}
      </View>
      {data.reconciled && (
        <View className="mt-1 flex-row items-center gap-1.5">
          <Icon as={BadgeCheck} size={14} className="text-positive" />
          <Text variant="muted" className="text-xs">
            {`Reconciled on ${formatDate(data.reconciled.reconciled_on)}`}
          </Text>
        </View>
      )}
      {data.separate_accounts.length > 0 && <SeparateAccounts accounts={data.separate_accounts} />}
    </View>
  );
}

/**
 * The real sub-accounts at the bank, one line each. Their money is not part
 * of the estimated balance above, so it is listed on its own.
 */
function SeparateAccounts({ accounts }: { accounts: Balance["separate_accounts"] }) {
  return (
    <View className="mt-3 gap-1.5 rounded-xl bg-muted/60 p-3" aria-label="Separate accounts">
      <Text variant="muted" className="text-xs">
        Separate accounts, not included above
      </Text>
      {accounts.map((a) => (
        <View key={a.pot_id} className="flex-row items-center justify-between gap-3">
          <View className="flex-1 flex-row items-center gap-2">
            <Icon as={Landmark} size={14} className="text-muted-foreground" />
            <Text className="text-sm" numberOfLines={1}>
              {a.name}
            </Text>
          </View>
          <Amount cents={a.balance_cents} size="sm" tone={a.balance_cents < 0 ? "negative" : "default"} />
        </View>
      ))}
    </View>
  );
}

/** "incl. 2 unbooked payments (−45,00 €)" */
function unbookedHint(b: Balance): string {
  const payments = b.unbooked_count === 1 ? "payment" : "payments";
  return `incl. ${b.unbooked_count} unbooked ${payments} (${formatCents(-b.unbooked_cents)})`;
}

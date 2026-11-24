import { ChevronDown, ChevronUp } from "lucide-react-native";
import { useCallback, useEffect, useState } from "react";
import { Pressable, View } from "react-native";

import { ledgers } from "@/api/endpoints";
import type { SafeToSpend as SafeToSpendData } from "@/api/types";
import { Amount } from "@/components/amount";
import { ErrorMessage } from "@/components/error-message";
import { Icon } from "@/components/ui/icon";
import { Skeleton } from "@/components/ui/skeleton";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { formatDate, today } from "@/lib/dates";
import { formatCents } from "@/lib/money";
import { cn } from "@/lib/utils";

/**
 * Shows what the main pot can spend freely until the end of the payday
 * period (rule L8), with an amount per day. Pressing it shows how the amount
 * comes about. A new `version` loads it again, like LedgerBalance.
 */
export function SafeToSpend({ ledgerId, version = 0 }: { ledgerId: string; version?: number }) {
  const { data, error, loading, reload } = useLoader(useCallback(() => ledgers.safeToSpend(ledgerId, today()), [ledgerId]));
  const [open, setOpen] = useState(false);
  useEffect(() => {
    if (version > 0) {
      void reload();
    }
  }, [version, reload]);

  if (loading && !data) {
    return <Skeleton className="h-12 w-full" />;
  }
  if (!data) {
    return <ErrorMessage message={error} onRetry={reload} />;
  }
  const overPlanned = data.amount_cents < 0;
  return (
    <View className="gap-2">
      <Pressable
        className="gap-1.5 active:opacity-70"
        onPress={() => setOpen(!open)}
        role="button"
        aria-expanded={open}
        aria-label={`Safe to spend: ${formatCents(data.amount_cents)}`}>
        <View className="flex-row items-center justify-between">
          <Text variant="overline">Safe to spend</Text>
          <Icon as={open ? ChevronUp : ChevronDown} size={18} className="text-muted-foreground" />
        </View>
        {overPlanned ? (
          <Text className="font-display text-2xl text-destructive">{`Over-planned by ${formatCents(-data.amount_cents)}`}</Text>
        ) : (
          <View className="flex-row flex-wrap items-baseline gap-x-2">
            <Amount cents={data.amount_cents} size="lg" className="text-positive" />
            <Text variant="muted">{`until ${formatDate(data.period_end)} · ${formatCents(data.per_day_cents)} per day`}</Text>
          </View>
        )}
      </Pressable>
      {open && <Breakdown data={data} />}
    </View>
  );
}

function Breakdown({ data }: { data: SafeToSpendData }) {
  const payments = data.recurring_count === 1 ? "payment" : "payments";
  const rows: { label: string; cents: number; total?: boolean }[] = [
    { label: "Main pot today", cents: data.main_pot_cents },
    { label: `Open ${payments} (${data.recurring_count})`, cents: -data.recurring_cents },
    { label: "Reserve rates to save", cents: -data.reserve_rates_cents },
    { label: "Savings goals to save", cents: -data.savings_cents },
    { label: "Budget left", cents: -data.budget_left_cents },
    { label: "Safe to spend", cents: data.amount_cents, total: true },
  ];
  return (
    <View className="gap-1.5 rounded-xl bg-muted/60 p-3" aria-label="Safe to spend breakdown">
      {rows.map((row) => (
        <View key={row.label} className={cn("flex-row justify-between gap-3", row.total && "mt-1 border-t border-border pt-2")}>
          <Text className={cn("text-sm", row.total && "font-semibold")}>{row.label}</Text>
          <Amount
            cents={row.cents}
            size="sm"
            className={cn(!row.total && "font-normal")}
            tone={row.total && row.cents < 0 ? "negative" : "default"}
          />
        </View>
      ))}
    </View>
  );
}

import { Link } from "expo-router";
import { useCallback, useState } from "react";
import { View } from "react-native";

import { recurring } from "@/api/endpoints";
import type { Occurrence, ReserveRate } from "@/api/types";
import { ErrorMessage } from "@/components/error-message";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { today } from "@/lib/dates";
import { errorMessage } from "@/lib/errors";
import { formatCents } from "@/lib/money";
import { describeOccurrence } from "@/lib/recurring";
import { describeRate } from "@/lib/reserves";
import { cn } from "@/lib/utils";

/**
 * The open occurrences of a ledger's recurring payments up to the end of the
 * current period. "Book" opens a prefilled booking, "Skip" closes one without
 * booking (rule R7). A skipped row stays while the ledger is open, so the
 * skip can be undone. Below them, the rates the reserves should receive in
 * this period (rule R14); "Transfer" opens a prefilled move into the reserve.
 * Without anything due or to save, the card is not shown.
 */
export function DuePayments({ ledgerId, onChanged }: { ledgerId: string; onChanged?: () => void }) {
  const { data, error, reload } = useLoader(
    useCallback(async () => {
      const date = today();
      const [due, rates] = await Promise.all([recurring.due(ledgerId, date), recurring.reserveRates(ledgerId, date)]);
      return { due, rates: rates.filter((r) => r.rate_cents > 0) };
    }, [ledgerId]),
  );
  // Occurrences skipped here, so they can be undone. If someone else undid
  // the skip meanwhile, the loaded list has the row again; it is shown once.
  const [skipped, setSkipped] = useState<Occurrence[]>([]);
  const skippedKeys = new Set(skipped.map(key));
  const rows = [...(data?.due ?? []).filter((o) => !skippedKeys.has(key(o))), ...skipped].sort((a, b) =>
    a.due_on.localeCompare(b.due_on),
  );

  if (error) {
    return <ErrorMessage message={error} onRetry={reload} />;
  }
  const rates = data?.rates ?? [];
  if (rows.length === 0 && rates.length === 0) {
    return null;
  }

  function markSkipped(o: Occurrence, isSkipped: boolean) {
    setSkipped((list) => (isSkipped ? [...list, o] : list.filter((s) => key(s) !== key(o))));
  }

  return (
    <View className="gap-2">
      <Text variant="muted">Due payments</Text>
      <Card className="py-2">
        <CardContent className="gap-0 px-0">
          {rows.map((o, index) => (
            <DueRow
              key={key(o)}
              ledgerId={ledgerId}
              occurrence={o}
              first={index === 0}
              skipped={skippedKeys.has(key(o))}
              onSkipped={(isSkipped) => {
                markSkipped(o, isSkipped);
                // A skip changes the estimated balance shown above.
                onChanged?.();
              }}
            />
          ))}
          {rates.map((rate, index) => (
            <RateRow key={rate.recurring_payment_id} ledgerId={ledgerId} rate={rate} first={rows.length === 0 && index === 0} />
          ))}
        </CardContent>
      </Card>
    </View>
  );
}

function key(o: Occurrence): string {
  return `${o.recurring_payment_id}/${o.due_on}`;
}

function DueRow({
  ledgerId,
  occurrence: o,
  first,
  skipped,
  onSkipped,
}: {
  ledgerId: string;
  occurrence: Occurrence;
  first: boolean;
  skipped: boolean;
  onSkipped: (skipped: boolean) => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  async function toggleSkip() {
    setBusy(true);
    setError(undefined);
    try {
      if (skipped) {
        await recurring.undoSkip(ledgerId, o.recurring_payment_id, o.due_on);
      } else {
        await recurring.skip(ledgerId, o.recurring_payment_id, o.due_on);
      }
      onSkipped(!skipped);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <View className={cn("gap-2 px-4 py-3", !first && "border-t border-border")}>
      <View className="flex-row items-center gap-3">
        <View className="flex-1 gap-0.5">
          <Text className="font-medium">{o.name}</Text>
          <Text variant="muted" className={cn(o.status === "overdue" && !skipped && "text-destructive")}>
            {skipped ? "Skipped" : describeOccurrence(o)}
          </Text>
        </View>
        <Text className="font-semibold">{formatCents(o.amount_cents)}</Text>
      </View>
      <View className="flex-row gap-2">
        {!skipped && (
          <Link
            href={{
              pathname: "/ledgers/[ledgerId]/new",
              params: { ledgerId, recurringPaymentId: o.recurring_payment_id, dueOn: o.due_on },
            }}
            asChild>
            <Button size="sm" aria-label={`Book ${o.name}`}>
              <Text>Book</Text>
            </Button>
          </Link>
        )}
        <Button size="sm" variant="outline" onPress={toggleSkip} disabled={busy} aria-label={`${skipped ? "Undo skip of" : "Skip"} ${o.name}`}>
          <Text>{skipped ? "Undo skip" : "Skip"}</Text>
        </Button>
      </View>
      <ErrorMessage message={error} />
    </View>
  );
}

function RateRow({ ledgerId, rate, first }: { ledgerId: string; rate: ReserveRate; first: boolean }) {
  const done = rate.open_cents === 0;
  return (
    <View className={cn("gap-2 px-4 py-3", !first && "border-t border-border")}>
      <View className="flex-row items-center gap-3">
        <View className="flex-1 gap-0.5">
          <Text className="font-medium">Save for {rate.name}</Text>
          <Text variant="muted">{done ? "Saved for this period" : describeRate(rate)}</Text>
        </View>
        <Text className={cn("font-semibold", done && "text-muted-foreground")}>{formatCents(rate.open_cents)}</Text>
      </View>
      {!done && (
        <View className="flex-row gap-2">
          <Link
            href={{
              pathname: "/ledgers/[ledgerId]/move",
              params: { ledgerId, toPotId: rate.pot_id, amountCents: String(rate.open_cents) },
            }}
            asChild>
            <Button size="sm" aria-label={`Transfer to ${rate.name}`}>
              <Text>Transfer</Text>
            </Button>
          </Link>
        </View>
      )}
    </View>
  );
}

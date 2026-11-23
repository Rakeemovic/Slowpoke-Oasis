import { Link } from "expo-router";
import { useCallback, useState } from "react";
import { View } from "react-native";

import { recurring } from "@/api/endpoints";
import type { Occurrence } from "@/api/types";
import { ErrorMessage } from "@/components/error-message";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { today } from "@/lib/dates";
import { errorMessage } from "@/lib/errors";
import { formatCents } from "@/lib/money";
import { describeOccurrence } from "@/lib/recurring";
import { cn } from "@/lib/utils";

/**
 * The open occurrences of a ledger's recurring payments up to the end of the
 * current period. "Book" opens a prefilled booking, "Skip" closes one without
 * booking (rule R7). A skipped row stays while the ledger is open, so the
 * skip can be undone. Without anything due, the card is not shown.
 */
export function DuePayments({ ledgerId }: { ledgerId: string }) {
  const { data, error, reload } = useLoader(useCallback(() => recurring.due(ledgerId, today()), [ledgerId]));
  // Occurrences skipped here, so they can be undone. If someone else undid
  // the skip meanwhile, the loaded list has the row again; it is shown once.
  const [skipped, setSkipped] = useState<Occurrence[]>([]);
  const skippedKeys = new Set(skipped.map(key));
  const rows = [...(data ?? []).filter((o) => !skippedKeys.has(key(o))), ...skipped].sort((a, b) =>
    a.due_on.localeCompare(b.due_on),
  );

  if (error) {
    return <ErrorMessage message={error} onRetry={reload} />;
  }
  if (rows.length === 0) {
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
              onSkipped={(isSkipped) => markSkipped(o, isSkipped)}
            />
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

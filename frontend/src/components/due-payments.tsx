import { Link } from "expo-router";
import { useCallback, useState } from "react";
import { View } from "react-native";

import { ledgers, recurring } from "@/api/endpoints";
import type { Occurrence } from "@/api/types";
import { ErrorMessage } from "@/components/error-message";
import { CircleCheck, PiggyBank, Repeat } from "lucide-react-native";

import { Amount } from "@/components/amount";
import { IconTile } from "@/components/icon-tile";
import { ListSection } from "@/components/list";
import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { today } from "@/lib/dates";
import { errorMessage } from "@/lib/errors";
import { hueFor } from "@/lib/hues";
import { describeOccurrence } from "@/lib/recurring";
import { describeRate as describeGoalRate } from "@/lib/goals";
import { describeRate as describeReserveRate } from "@/lib/reserves";
import { cn } from "@/lib/utils";

/**
 * The open occurrences of a ledger's recurring payments up to the end of the
 * current period. "Book" opens a prefilled booking, "Skip" closes one without
 * booking (rule R7). A skipped row stays while the ledger is open, so the
 * skip can be undone. Below them, the rates the reserves (rule R14) and the
 * savings goals (rule T8) should receive in this period; "Transfer" opens a
 * prefilled move into their pot.
 * Without anything due or to save, the card is not shown.
 */
export function DuePayments({ ledgerId, onChanged }: { ledgerId: string; onChanged?: () => void }) {
  const { data, error, reload } = useLoader(
    useCallback(async () => {
      const date = today();
      const [due, rates, goals] = await Promise.all([
        recurring.due(ledgerId, date),
        recurring.reserveRates(ledgerId, date),
        ledgers.savingsGoals(ledgerId, date),
      ]);
      // Reserves and goals ask for a rate the same way; both have a pot to move money into.
      const saving: SaveRate[] = [
        ...rates
          .filter((r) => r.rate_cents > 0)
          .map((r) => ({
            key: r.recurring_payment_id,
            name: r.name,
            potId: r.pot_id,
            openCents: r.open_cents,
            detail: describeReserveRate(r),
          })),
        ...goals
          .filter((g) => g.rate_cents > 0)
          .map((g) => ({ key: g.pot_id, name: g.name, potId: g.pot_id, openCents: g.open_cents, detail: describeGoalRate(g) })),
      ];
      return { due, saving };
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
  const saving = data?.saving ?? [];
  if (rows.length === 0 && saving.length === 0) {
    return null;
  }

  function markSkipped(o: Occurrence, isSkipped: boolean) {
    setSkipped((list) => (isSkipped ? [...list, o] : list.filter((s) => key(s) !== key(o))));
  }

  return (
    <ListSection title="Due payments">
      {rows.map((o) => (
        <DueRow
          key={key(o)}
          ledgerId={ledgerId}
          occurrence={o}
          skipped={skippedKeys.has(key(o))}
          onSkipped={(isSkipped) => {
            markSkipped(o, isSkipped);
            // A skip changes the estimated balance shown above.
            onChanged?.();
          }}
        />
      ))}
      {saving.map((rate) => (
        <SaveRow key={rate.key} ledgerId={ledgerId} rate={rate} />
      ))}
    </ListSection>
  );
}

function key(o: Occurrence): string {
  return `${o.recurring_payment_id}/${o.due_on}`;
}

function DueRow({
  ledgerId,
  occurrence: o,
  skipped,
  onSkipped,
}: {
  ledgerId: string;
  occurrence: Occurrence;
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
    <View className={cn("gap-2 px-4 py-3", skipped && "opacity-60")}>
      <View className="flex-row items-center gap-3">
        <IconTile icon={Repeat} hue={hueFor(o.recurring_payment_id)} />
        <View className="flex-1 gap-0.5">
          <Text className="font-medium">{o.name}</Text>
          <Text variant="muted" className={cn(o.status === "overdue" && !skipped && "font-medium text-destructive")}>
            {skipped ? "Skipped" : describeOccurrence(o)}
          </Text>
        </View>
        <Amount cents={o.amount_cents} />
      </View>
      <View className="flex-row gap-2 pl-[52px]">
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
        <Button
          size="sm"
          variant="ghost"
          onPress={toggleSkip}
          disabled={busy}
          aria-label={`${skipped ? "Undo skip of" : "Skip"} ${o.name}`}>
          <Text>{skipped ? "Undo skip" : "Skip"}</Text>
        </Button>
      </View>
      <ErrorMessage message={error} />
    </View>
  );
}

/** What a reserve or a savings goal should receive in this period, in the shape the card shows. */
interface SaveRate {
  key: string;
  name: string;
  /** The pot the money goes into. */
  potId: string;
  openCents: number;
  detail: string;
}

function SaveRow({ ledgerId, rate }: { ledgerId: string; rate: SaveRate }) {
  const done = rate.openCents === 0;
  return (
    <View className="gap-2 px-4 py-3">
      <View className="flex-row items-center gap-3">
        <IconTile icon={done ? CircleCheck : PiggyBank} hue={hueFor(rate.potId)} />
        <View className="flex-1 gap-0.5">
          <Text className="font-medium">Save for {rate.name}</Text>
          <Text variant="muted">{done ? "Saved for this period" : rate.detail}</Text>
        </View>
        <Amount cents={rate.openCents} tone={done ? "muted" : "default"} />
      </View>
      {!done && (
        <View className="flex-row gap-2 pl-[52px]">
          <Link
            href={{
              pathname: "/ledgers/[ledgerId]/move",
              params: { ledgerId, toPotId: rate.potId, amountCents: String(rate.openCents) },
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

import { router, useLocalSearchParams } from "expo-router";
import { useCallback, useState } from "react";
import { ActivityIndicator, View } from "react-native";
import { toast } from "sonner-native";

import { ledgers, pots, recurring } from "@/api/endpoints";
import type { Pot, RecurringPayment, RecurringPaymentInput } from "@/api/types";
import { ConfirmPanel } from "@/components/confirm-panel";
import { ErrorMessage } from "@/components/error-message";
import { RecurringForm } from "@/components/recurring-form";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { today } from "@/lib/dates";
import { errorMessage } from "@/lib/errors";
import { formatCents } from "@/lib/money";
import { defaultPot } from "@/lib/pots";

/**
 * Owner only (rule R8): change a recurring payment, archive it or bring it
 * back. When its reserve is no longer needed (archived, or now monthly), the
 * screen offers to move what is left to the default pot and archive the
 * reserve; declined, the reserve stays an ordinary pot.
 */
export default function RecurringPaymentScreen() {
  const { ledgerId, paymentId } = useLocalSearchParams<{ ledgerId: string; paymentId: string }>();
  const { data, error, loading, reload } = useLoader(useCallback(() => recurring.list(ledgerId, today()), [ledgerId]));
  const payment = data?.find((p) => p.id === paymentId);
  const [leftover, setLeftover] = useState<Leftover>();

  if (leftover) {
    return (
      <Screen>
        <LeftoverOffer ledgerId={ledgerId} leftover={leftover} />
      </Screen>
    );
  }

  if (!data || !payment) {
    return (
      <Screen>
        {loading && <ActivityIndicator />}
        <ErrorMessage message={error ?? (data ? "This recurring payment does not exist." : undefined)} onRetry={reload} />
      </Screen>
    );
  }

  async function save(values: RecurringPaymentInput) {
    await recurring.update(ledgerId, paymentId, values, today());
    await afterChange(ledgerId, payment!, values.interval_months > 1 && !payment!.archived, setLeftover);
  }

  return (
    <Screen>
      <RecurringForm ledgerId={ledgerId} initial={toInput(payment)} onSubmit={save} />
      {/* Rule R5: a new schedule must not turn past due dates into overdue ones. */}
      <Text variant="muted">
        Changing how often or on which day it is due counts from today: earlier due dates are not shown as overdue.
      </Text>
      <ArchiveSection ledgerId={ledgerId} payment={payment} onLeftover={setLeftover} />
    </Screen>
  );
}

function toInput(p: RecurringPayment): RecurringPaymentInput {
  return {
    name: p.name,
    amount_cents: p.amount_cents,
    category_id: p.category_id,
    pot_id: p.pot_id,
    interval_months: p.interval_months,
    due_day: p.due_day,
    first_month: p.first_month,
    last_month: p.last_month ?? "",
  };
}

function ArchiveSection({
  ledgerId,
  payment,
  onLeftover,
}: {
  ledgerId: string;
  payment: RecurringPayment;
  onLeftover: (leftover: Leftover) => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  async function setArchived(archived: boolean) {
    setBusy(true);
    setError(undefined);
    try {
      await recurring.update(ledgerId, payment.id, { archived }, today());
      await afterChange(ledgerId, payment, !archived && payment.interval_months > 1, onLeftover);
    } catch (e) {
      setError(errorMessage(e));
      setBusy(false);
    }
  }

  return (
    <View className="gap-2">
      {payment.archived ? (
        <>
          <Text variant="muted">This payment is archived. It shows no due dates.</Text>
          <Button variant="outline" onPress={() => setArchived(false)} disabled={busy}>
            <Text>Restore payment</Text>
          </Button>
        </>
      ) : (
        <Button variant="outline" onPress={() => setArchived(true)} disabled={busy}>
          <Text>Archive payment</Text>
        </Button>
      )}
      <ErrorMessage message={error} />
    </View>
  );
}

/** A reserve its payment no longer needs, and the pot its money can go back to. */
interface Leftover {
  reserve: Pot;
  defaultPotId: string;
}

/**
 * After a change, either goes back or, when the payment had an active reserve
 * that it no longer saves in, hands that reserve to onLeftover.
 */
async function afterChange(
  ledgerId: string,
  before: RecurringPayment,
  stillSaves: boolean,
  onLeftover: (leftover: Leftover) => void,
) {
  const hadReserve = before.reserve_pot_id && before.interval_months > 1 && !before.archived;
  if (hadReserve && !stillSaves) {
    const list = await pots.list(ledgerId).catch(() => []);
    const reserve = list.find((p) => p.id === before.reserve_pot_id && !p.archived);
    const defaultPotId = defaultPot(list)?.id;
    if (reserve && defaultPotId) {
      onLeftover({ reserve, defaultPotId });
      return;
    }
  }
  toast.success("Recurring payment saved");
  router.back();
}

function LeftoverOffer({ ledgerId, leftover }: { ledgerId: string; leftover: Leftover }) {
  const { reserve, defaultPotId } = leftover;
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const amount = formatCents(reserve.balance_cents);
  const message =
    reserve.balance_cents > 0
      ? `The payment no longer saves in its reserve “${reserve.name}”, which still holds ${amount}. Move it to the default pot and archive the reserve?`
      : `The payment no longer saves in its reserve “${reserve.name}”. Archive the reserve?`;

  async function clearReserve() {
    setBusy(true);
    setError(undefined);
    try {
      if (reserve.balance_cents > 0) {
        await ledgers.createTransaction(ledgerId, {
          type: "transfer",
          pot_id: reserve.id,
          counterpart_pot_id: defaultPotId,
          amount_cents: reserve.balance_cents,
          booked_on: today(),
        });
      }
      await pots.update(ledgerId, reserve.id, { archived: true }, today());
      toast.success("Reserve archived");
      router.back();
    } catch (e) {
      setError(errorMessage(e));
      setBusy(false);
    }
  }

  return (
    <>
      <Text variant="muted">Saved.</Text>
      <ConfirmPanel
        tone="default"
        message={message}
        confirmLabel={reserve.balance_cents > 0 ? `Move ${amount} and archive` : "Archive reserve"}
        busy={busy}
        onConfirm={clearReserve}
        onCancel={() => router.back()}
      />
      <Text variant="muted">Cancel keeps “{reserve.name}” as an ordinary pot.</Text>
      <ErrorMessage message={error} />
    </>
  );
}

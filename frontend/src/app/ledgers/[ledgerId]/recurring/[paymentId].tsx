import { router, useLocalSearchParams } from "expo-router";
import { useCallback, useState } from "react";
import { ActivityIndicator, View } from "react-native";

import { recurring } from "@/api/endpoints";
import type { RecurringPayment, RecurringPaymentInput } from "@/api/types";
import { ErrorMessage } from "@/components/error-message";
import { RecurringForm } from "@/components/recurring-form";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { today } from "@/lib/dates";
import { errorMessage } from "@/lib/errors";

/** Owner only (rule R8): change a recurring payment, archive it or bring it back. */
export default function RecurringPaymentScreen() {
  const { ledgerId, paymentId } = useLocalSearchParams<{ ledgerId: string; paymentId: string }>();
  const { data, error, loading, reload } = useLoader(useCallback(() => recurring.list(ledgerId, today()), [ledgerId]));
  const payment = data?.find((p) => p.id === paymentId);

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
    router.back();
  }

  return (
    <Screen>
      <RecurringForm ledgerId={ledgerId} initial={toInput(payment)} onSubmit={save} />
      {/* Rule R5: a new schedule must not turn past due dates into overdue ones. */}
      <Text variant="muted">
        Changing how often or on which day it is due counts from today: earlier due dates are not shown as overdue.
      </Text>
      <ArchiveSection ledgerId={ledgerId} payment={payment} />
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

function ArchiveSection({ ledgerId, payment }: { ledgerId: string; payment: RecurringPayment }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  async function setArchived(archived: boolean) {
    setBusy(true);
    setError(undefined);
    try {
      await recurring.update(ledgerId, payment.id, { archived }, today());
      router.back();
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

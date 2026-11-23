import { router, Stack, useLocalSearchParams } from "expo-router";
import { useCallback } from "react";
import { ActivityIndicator } from "react-native";

import { ledgers, recurring } from "@/api/endpoints";
import type { TransactionChange } from "@/api/types";
import { BookingForm } from "@/components/booking-form";
import { ErrorMessage } from "@/components/error-message";
import { Screen } from "@/components/screen";
import { useLoader } from "@/hooks/use-loader";
import { today } from "@/lib/dates";

/**
 * Books an income or expense. With recurringPaymentId and dueOn it books that
 * occurrence of a recurring payment instead: the form starts with the
 * payment's values and the due date, and everything can still be changed.
 */
export default function NewTransactionScreen() {
  const { ledgerId, recurringPaymentId, dueOn } = useLocalSearchParams<{
    ledgerId: string;
    recurringPaymentId?: string;
    dueOn?: string;
  }>();

  if (recurringPaymentId && dueOn) {
    return <BookOccurrence ledgerId={ledgerId} paymentId={recurringPaymentId} dueOn={dueOn} />;
  }

  async function create(values: TransactionChange) {
    await ledgers.createTransaction(ledgerId, { ...values, note: values.note || undefined });
    router.back();
  }

  return (
    <Screen>
      <BookingForm ledgerId={ledgerId} onSubmit={create} />
    </Screen>
  );
}

function BookOccurrence({ ledgerId, paymentId, dueOn }: { ledgerId: string; paymentId: string; dueOn: string }) {
  const { data, error, loading, reload } = useLoader(useCallback(() => recurring.list(ledgerId, today()), [ledgerId]));
  const payment = data?.find((p) => p.id === paymentId);

  if (!payment) {
    return (
      <Screen>
        {loading && <ActivityIndicator />}
        <ErrorMessage message={error ?? (data ? "This recurring payment does not exist." : undefined)} onRetry={reload} />
      </Screen>
    );
  }

  async function create(values: TransactionChange) {
    await ledgers.createTransaction(ledgerId, {
      ...values,
      note: values.note || undefined,
      recurring_payment_id: paymentId,
      recurring_due_on: dueOn,
    });
    router.back();
  }

  return (
    <Screen>
      <Stack.Screen options={{ title: `Book ${payment.name}` }} />
      <BookingForm
        ledgerId={ledgerId}
        expenseOnly
        initial={{
          type: "expense",
          category_id: payment.category_id,
          pot_id: payment.pot_id,
          amount_cents: payment.amount_cents,
          // A direct debit leaves the account on the due date, also when it is booked later.
          booked_on: dueOn,
          note: "",
        }}
        onSubmit={create}
      />
    </Screen>
  );
}

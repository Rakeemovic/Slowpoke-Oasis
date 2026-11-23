import { router, Stack, useLocalSearchParams } from "expo-router";
import { useCallback, useState } from "react";
import { ActivityIndicator } from "react-native";

import { ledgers, recurring } from "@/api/endpoints";
import type { NewTransaction, TransactionChange } from "@/api/types";
import { BookingForm } from "@/components/booking-form";
import { ConfirmPanel } from "@/components/confirm-panel";
import { ErrorMessage } from "@/components/error-message";
import { Screen } from "@/components/screen";
import { useLoader } from "@/hooks/use-loader";
import { today } from "@/lib/dates";
import { errorMessage } from "@/lib/errors";
import { formatCents } from "@/lib/money";
import { reserveShortfall } from "@/lib/reserves";

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
  const booking = useBookingWithTopUp(ledgerId);

  if (recurringPaymentId && dueOn) {
    return <BookOccurrence ledgerId={ledgerId} paymentId={recurringPaymentId} dueOn={dueOn} booking={booking} />;
  }

  return (
    <Screen>
      <BookingForm ledgerId={ledgerId} onSubmit={(values) => booking.book(withoutEmptyNote(values))} />
      <TopUpOffer booking={booking} />
    </Screen>
  );
}

function BookOccurrence({
  ledgerId,
  paymentId,
  dueOn,
  booking,
}: {
  ledgerId: string;
  paymentId: string;
  dueOn: string;
  booking: BookingWithTopUp;
}) {
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

  return (
    <Screen>
      <Stack.Screen options={{ title: `Book ${payment.name}` }} />
      <BookingForm
        ledgerId={ledgerId}
        expenseOnly
        initial={{
          type: "expense",
          category_id: payment.category_id,
          // A payment due every few months books from its reserve (rule R10).
          pot_id: payment.pot_id,
          amount_cents: payment.amount_cents,
          // A direct debit leaves the account on the due date, also when it is booked later.
          booked_on: dueOn,
          note: "",
        }}
        onSubmit={(values) =>
          booking.book({ ...withoutEmptyNote(values), recurring_payment_id: paymentId, recurring_due_on: dueOn })
        }
      />
      <TopUpOffer booking={booking} />
    </Screen>
  );
}

function withoutEmptyNote(values: TransactionChange): NewTransaction {
  return { ...values, note: values.note || undefined };
}

type BookingWithTopUp = ReturnType<typeof useBookingWithTopUp>;

/**
 * Books a transaction. If it is an expense from a reserve that holds too
 * little (rule R13), the booking fails with a hint and `offer` asks whether
 * to move the missing amount from the default pot first.
 */
function useBookingWithTopUp(ledgerId: string) {
  const [offer, setOffer] = useState<{ tx: NewTransaction; missingCents: number }>();

  async function book(tx: NewTransaction) {
    setOffer(undefined);
    try {
      await ledgers.createTransaction(ledgerId, tx);
    } catch (e) {
      const missingCents = await reserveShortfall(ledgerId, tx, e).catch(() => undefined);
      if (missingCents === undefined) {
        throw e;
      }
      setOffer({ tx, missingCents });
      // The form shows this and becomes usable again, e.g. to lower the amount.
      throw new Error(`The reserve lacks ${formatCents(missingCents)} for this payment.`);
    }
    router.back();
  }

  async function confirmTopUp() {
    await ledgers.createTransaction(ledgerId, { ...offer!.tx, top_up: true });
    router.back();
  }

  return { offer, book, confirmTopUp, dismiss: () => setOffer(undefined) };
}

function TopUpOffer({ booking }: { booking: BookingWithTopUp }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  if (!booking.offer) {
    return null;
  }
  const missing = formatCents(booking.offer.missingCents);

  async function confirm() {
    setBusy(true);
    setError(undefined);
    try {
      await booking.confirmTopUp();
    } catch (e) {
      setError(errorMessage(e));
      setBusy(false);
    }
  }

  return (
    <>
      <ConfirmPanel
        tone="default"
        message={`Move ${missing} from the default pot into the reserve and book the payment?`}
        confirmLabel={`Top up ${missing} and book`}
        busy={busy}
        onConfirm={confirm}
        onCancel={booking.dismiss}
      />
      <ErrorMessage message={error} />
    </>
  );
}

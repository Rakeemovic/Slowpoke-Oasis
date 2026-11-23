import { router, useLocalSearchParams } from "expo-router";
import { useCallback, useState } from "react";
import { ActivityIndicator, View } from "react-native";

import { auth, ledgers } from "@/api/endpoints";
import type { Category, NewPotTransfer, Transaction, TransactionChange } from "@/api/types";
import { BookingForm } from "@/components/booking-form";
import { ConfirmPanel } from "@/components/confirm-panel";
import { ErrorMessage } from "@/components/error-message";
import { MoveForm } from "@/components/move-form";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { errorMessage } from "@/lib/errors";
import { formatCents, formatSignedCents } from "@/lib/money";
import { isLinkedTransfer, transactionTitle } from "@/lib/transactions";

export default function TransactionScreen() {
  const { ledgerId, transactionId, kind } = useLocalSearchParams<{
    ledgerId: string;
    transactionId: string;
    kind?: string;
  }>();
  const load = useCallback(
    () => Promise.all([ledgers.transaction(ledgerId, transactionId), ledgers.categories(ledgerId), auth.me()]),
    [ledgerId, transactionId],
  );
  const { data, error, loading, reload } = useLoader(load);

  if (!data) {
    return (
      <Screen>
        {loading && <ActivityIndicator />}
        <ErrorMessage message={error} onRetry={reload} />
      </Screen>
    );
  }
  const [tx, categories, me] = data;
  // Only incomes and expenses have a form; transfers are shown as they are.
  const editable = tx.can_edit && (tx.type === "expense" || tx.type === "income");

  // A transfer between pots of this ledger has its own form.
  const movable = tx.can_edit && tx.type === "transfer";

  async function save(values: TransactionChange | NewPotTransfer) {
    await ledgers.updateTransaction(ledgerId, transactionId, values);
    router.back();
  }

  return (
    <Screen>
      {editable ? (
        <BookingForm
          ledgerId={ledgerId}
          initial={toChange(tx)}
          canMarkSpontaneous={tx.user_id === me.user_id}
          // A booking of a recurring payment stays an expense (rule R6).
          expenseOnly={tx.recurring_payment_id !== undefined}
          onSubmit={save}
        />
      ) : movable ? (
        <MoveForm ledgerId={ledgerId} initial={toMove(tx)} onSubmit={save} />
      ) : (
        <Details tx={tx} categories={categories} showAuthor={kind !== "personal"} />
      )}
      {tx.can_delete && <DeleteSection tx={tx} categories={categories} />}
    </Screen>
  );
}

function toMove(tx: Transaction): NewPotTransfer {
  return {
    type: "transfer",
    pot_id: tx.pot_id,
    counterpart_pot_id: tx.counterpart_pot_id ?? "",
    amount_cents: Math.abs(tx.amount_cents),
    booked_on: tx.booked_on,
    note: tx.note ?? "",
  };
}

/** The form wants a positive amount and plain strings; the API sends signed values. */
function toChange(tx: Transaction): TransactionChange {
  return {
    type: tx.type as TransactionChange["type"],
    category_id: tx.category_id ?? "",
    pot_id: tx.pot_id,
    amount_cents: Math.abs(tx.amount_cents),
    booked_on: tx.booked_on,
    note: tx.note ?? "",
    is_spontaneous: tx.is_spontaneous,
  };
}

/** Read-only view for bookings the user may not edit, such as transfers. */
function Details({ tx, categories, showAuthor }: { tx: Transaction; categories: Category[]; showAuthor: boolean }) {
  const isTransfer = tx.type !== "expense" && tx.type !== "income";
  return (
    <Card>
      <CardContent className="gap-1">
        <Text className="font-medium">{transactionTitle(tx, categories)}</Text>
        <Text className="text-2xl font-semibold">
          {isTransfer ? formatCents(Math.abs(tx.amount_cents)) : formatSignedCents(tx.amount_cents)}
        </Text>
        <Text variant="muted">{tx.booked_on}</Text>
        {showAuthor && <Text variant="muted">Booked by {tx.username ?? "a former member"}</Text>}
        {tx.note && <Text>{tx.note}</Text>}
        {tx.is_spontaneous && <Text variant="muted">⚡ Spontaneous</Text>}
        {isLinkedTransfer(tx) && (
          <Text variant="muted">Transfers between ledgers cannot be edited. Delete it and book it again instead.</Text>
        )}
      </CardContent>
    </Card>
  );
}

function DeleteSection({ tx, categories }: { tx: Transaction; categories: Category[] }) {
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  async function remove() {
    setBusy(true);
    setError(undefined);
    try {
      await ledgers.deleteTransaction(tx.ledger_id, tx.id);
      router.back();
    } catch (e) {
      setError(errorMessage(e));
      setBusy(false);
    }
  }

  // Name every consequence before an action that cannot be undone in the app yet.
  const linkedNote = isLinkedTransfer(tx) ? " The other side of the transfer is deleted too." : "";
  const message = `Delete “${transactionTitle(tx, categories)}”?${linkedNote} It is removed for good after 30 days.`;

  return (
    <View className="gap-2">
      {confirming ? (
        <ConfirmPanel
          message={message}
          confirmLabel="Delete"
          busy={busy}
          onConfirm={remove}
          onCancel={() => setConfirming(false)}
        />
      ) : (
        <Button variant="outline" onPress={() => setConfirming(true)}>
          <Text className="text-destructive">Delete</Text>
        </Button>
      )}
      <ErrorMessage message={error} />
    </View>
  );
}

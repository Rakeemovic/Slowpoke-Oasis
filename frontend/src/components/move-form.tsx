import { useCallback, useState } from "react";
import { ActivityIndicator, View } from "react-native";

import { pots } from "@/api/endpoints";
import type { NewPotTransfer } from "@/api/types";
import { ChoicePicker } from "@/components/choice-picker";
import { ErrorMessage } from "@/components/error-message";
import { AmountField, FormField } from "@/components/form-field";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { bookingProblem } from "@/lib/booking";
import { today } from "@/lib/dates";
import { errorMessage } from "@/lib/errors";
import { centsToAmountInput, parseAmountToCents } from "@/lib/money";
import { defaultPot, potLabel, selectablePots } from "@/lib/pots";

/**
 * The form for moving money from one pot to another in the same ledger, used
 * both for a new move and to edit an existing one. onSubmit saves the values
 * and throws on failure; the form shows the error. `initial` may prefill only
 * some fields; without a "from" pot, a prefilled move starts from the default
 * pot, e.g. a rate into a reserve.
 */
export function MoveForm({
  ledgerId,
  initial,
  onSubmit,
}: {
  ledgerId: string;
  initial?: Partial<NewPotTransfer>;
  onSubmit: (values: NewPotTransfer) => Promise<void>;
}) {
  const potList = useLoader(useCallback(() => pots.list(ledgerId), [ledgerId]));

  const [chosenFromId, setFromId] = useState<string | undefined>(initial?.pot_id);
  const fromId = chosenFromId ?? (initial ? defaultPot(potList.data ?? [])?.id : undefined);
  const [toId, setToId] = useState<string | undefined>(initial?.counterpart_pot_id);
  const [amount, setAmount] = useState(initial?.amount_cents ? centsToAmountInput(initial.amount_cents) : "");
  const [bookedOn, setBookedOn] = useState(initial?.booked_on ?? today);
  const [note, setNote] = useState(initial?.note ?? "");
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  async function submit() {
    const cents = parseAmountToCents(amount);
    const problem = bookingProblem(cents, bookedOn) ?? potProblem(fromId, toId);
    if (problem) {
      setError(problem);
      return;
    }
    setBusy(true);
    setError(undefined);
    try {
      await onSubmit({
        type: "transfer",
        pot_id: fromId!,
        counterpart_pot_id: toId!,
        amount_cents: cents!,
        booked_on: bookedOn,
        note: note.trim(),
      });
    } catch (e) {
      setError(errorMessage(e));
      setBusy(false);
    }
  }

  return (
    <>
      {potList.loading && !potList.data && <ActivityIndicator />}
      <ErrorMessage message={potList.error} onRetry={potList.reload} />
      {potList.data && (
        <>
          <PotChoice label="From" pots={potList.data} chosen={fromId} onSelect={setFromId} />
          <PotChoice label="To" pots={potList.data} chosen={toId} onSelect={setToId} />
        </>
      )}
      <AmountField label="Amount (€)" value={amount} onChangeText={setAmount} />
      <FormField label="Date (YYYY-MM-DD)" value={bookedOn} onChangeText={setBookedOn} autoCapitalize="none" />
      <FormField label="Note (optional)" value={note} onChangeText={setNote} />
      <ErrorMessage message={error} />
      <Button onPress={submit} disabled={busy}>
        <Text>{busy ? "Saving…" : "Save"}</Text>
      </Button>
    </>
  );
}

/** Rule X4: both pots are needed and they must differ. */
function potProblem(fromId?: string, toId?: string): string | undefined {
  if (!fromId || !toId) {
    return "Choose the pot the money leaves and the pot it arrives in.";
  }
  return fromId === toId ? "Choose two different pots." : undefined;
}

function PotChoice({
  label,
  pots: all,
  chosen,
  onSelect,
}: {
  label: string;
  pots: Parameters<typeof selectablePots>[0];
  chosen?: string;
  onSelect: (id: string) => void;
}) {
  return (
    <View className="gap-2">
      <Label>{label}</Label>
      <ChoicePicker
        choices={selectablePots(all, chosen).map((p) => ({ id: p.id, label: potLabel(p) }))}
        selected={chosen}
        onSelect={onSelect}
        emptyText="There are no pots yet."
      />
    </View>
  );
}

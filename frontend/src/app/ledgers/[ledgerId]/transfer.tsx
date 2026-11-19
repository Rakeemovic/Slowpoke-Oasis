import { router, useLocalSearchParams } from "expo-router";
import { useState } from "react";
import { ActivityIndicator, View } from "react-native";

import { transfers } from "@/api/endpoints";
import { ChoicePicker } from "@/components/choice-picker";
import { ErrorMessage } from "@/components/error-message";
import { FormField } from "@/components/form-field";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { bookingProblem } from "@/lib/booking";
import { today } from "@/lib/dates";
import { errorMessage } from "@/lib/errors";
import { loadMyLedgers } from "@/lib/ledgers";
import { parseAmountToCents } from "@/lib/money";

/**
 * Moves money from this ledger into another one, e.g. from the personal ledger
 * into a household. Both sides use the default pot (rule X14: two linked
 * bookings, deleted and restored together).
 */
export default function TransferScreen() {
  const { ledgerId } = useLocalSearchParams<{ ledgerId: string }>();
  const myLedgers = useLoader(loadMyLedgers);

  const [targetId, setTargetId] = useState<string>();
  const [amount, setAmount] = useState("");
  const [bookedOn, setBookedOn] = useState(today);
  const [note, setNote] = useState("");
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  const targets = (myLedgers.data ?? []).filter((l) => l.id !== ledgerId);

  async function submit() {
    const cents = parseAmountToCents(amount);
    const problem = bookingProblem(cents, bookedOn) ?? (targetId ? undefined : "Choose where the money goes.");
    if (problem) {
      setError(problem);
      return;
    }
    setBusy(true);
    setError(undefined);
    try {
      await transfers.create({
        from_ledger_id: ledgerId,
        to_ledger_id: targetId!,
        amount_cents: cents!,
        booked_on: bookedOn,
        note: note.trim() || undefined,
      });
      router.back();
    } catch (e) {
      setError(errorMessage(e));
      setBusy(false);
    }
  }

  return (
    <Screen>
      <View className="gap-2">
        <Label>To</Label>
        {myLedgers.loading && !myLedgers.data && <ActivityIndicator />}
        <ErrorMessage message={myLedgers.error} onRetry={myLedgers.reload} />
        {myLedgers.data && (
          <ChoicePicker
            choices={targets.map((l) => ({ id: l.id, label: l.name }))}
            selected={targetId}
            onSelect={setTargetId}
            emptyText="There is no other ledger yet. Create or join a household first."
          />
        )}
      </View>

      <FormField label="Amount (€)" value={amount} onChangeText={setAmount} keyboardType="decimal-pad" placeholder="0.00" />
      <FormField label="Date (YYYY-MM-DD)" value={bookedOn} onChangeText={setBookedOn} autoCapitalize="none" />
      <FormField label="Note (optional)" value={note} onChangeText={setNote} />

      <ErrorMessage message={error} />
      <Button onPress={submit} disabled={busy}>
        <Text>{busy ? "Saving…" : "Transfer"}</Text>
      </Button>
    </Screen>
  );
}

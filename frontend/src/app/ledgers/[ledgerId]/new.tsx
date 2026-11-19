import { router, useLocalSearchParams } from "expo-router";
import { useCallback, useState } from "react";
import { ActivityIndicator, View } from "react-native";

import { ledgers } from "@/api/endpoints";
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
import { parseAmountToCents } from "@/lib/money";

type Kind = "expense" | "income";

export default function NewTransactionScreen() {
  const { ledgerId } = useLocalSearchParams<{ ledgerId: string }>();
  const categories = useLoader(useCallback(() => ledgers.categories(ledgerId), [ledgerId]));

  const [kind, setKind] = useState<Kind>("expense");
  const [categoryId, setCategoryId] = useState<string>();
  const [amount, setAmount] = useState("");
  const [bookedOn, setBookedOn] = useState(today);
  const [note, setNote] = useState("");
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  // Rule K3: the category must match the type, so only matching ones are offered.
  const choices = (categories.data ?? []).filter((c) => !c.archived && c.is_income === (kind === "income"));

  function changeKind(next: Kind) {
    setKind(next);
    setCategoryId(undefined);
  }

  async function submit() {
    const cents = parseAmountToCents(amount);
    const problem = validate(cents, bookedOn, categoryId);
    if (problem) {
      setError(problem);
      return;
    }
    setBusy(true);
    setError(undefined);
    try {
      await ledgers.createTransaction(ledgerId, {
        type: kind,
        category_id: categoryId!,
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
      <View className="flex-row gap-2">
        <KindButton label="Expense" active={kind === "expense"} onPress={() => changeKind("expense")} />
        <KindButton label="Income" active={kind === "income"} onPress={() => changeKind("income")} />
      </View>

      <FormField label="Amount (€)" value={amount} onChangeText={setAmount} keyboardType="decimal-pad" placeholder="0.00" />
      <FormField label="Date (YYYY-MM-DD)" value={bookedOn} onChangeText={setBookedOn} autoCapitalize="none" />

      <View className="gap-2">
        <Label>Category</Label>
        {categories.loading && !categories.data && <ActivityIndicator />}
        <ErrorMessage message={categories.error} onRetry={categories.reload} />
        <ChoicePicker
          choices={choices.map((c) => ({ id: c.id, label: c.name }))}
          selected={categoryId}
          onSelect={setCategoryId}
          emptyText="No categories of this type yet. Add one below."
        />
        <AddCategory
          ledgerId={ledgerId}
          isIncome={kind === "income"}
          onAdded={(id) => {
            setCategoryId(id);
            void categories.reload();
          }}
        />
      </View>

      <FormField label="Note (optional)" value={note} onChangeText={setNote} />
      <ErrorMessage message={error} />
      <Button onPress={submit} disabled={busy}>
        <Text>{busy ? "Saving…" : "Save"}</Text>
      </Button>
    </Screen>
  );
}

/** Returns a message for the first invalid input, or undefined if all are fine. */
function validate(cents: number | null, bookedOn: string, categoryId?: string): string | undefined {
  return bookingProblem(cents, bookedOn) ?? (categoryId ? undefined : "Choose a category.");
}

function KindButton({ label, active, onPress }: { label: string; active: boolean; onPress: () => void }) {
  return (
    <Button className="flex-1" variant={active ? "default" : "outline"} onPress={onPress} aria-selected={active}>
      <Text>{label}</Text>
    </Button>
  );
}

function AddCategory({
  ledgerId,
  isIncome,
  onAdded,
}: {
  ledgerId: string;
  isIncome: boolean;
  onAdded: (id: string) => void;
}) {
  const [name, setName] = useState("");
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  async function submit() {
    setBusy(true);
    setError(undefined);
    try {
      const { id } = await ledgers.createCategory(ledgerId, name.trim(), isIncome);
      setName("");
      onAdded(id);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <View className="gap-2">
      <View className="flex-row items-end gap-2">
        <View className="flex-1">
          <FormField
            label={isIncome ? "New income category" : "New expense category"}
            value={name}
            onChangeText={setName}
            onSubmitEditing={submit}
          />
        </View>
        <Button variant="secondary" onPress={submit} disabled={busy || !name.trim()}>
          <Text>Add</Text>
        </Button>
      </View>
      <ErrorMessage message={error} />
    </View>
  );
}

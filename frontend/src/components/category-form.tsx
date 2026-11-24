import { useState } from "react";
import { View } from "react-native";

import type { Purpose } from "@/api/types";
import { ChoicePicker } from "@/components/choice-picker";
import { ErrorMessage } from "@/components/error-message";
import { FormField } from "@/components/form-field";
import { KindButton, Segmented } from "@/components/kind-button";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Text } from "@/components/ui/text";
import { purposeChoices, purposeFromChoice } from "@/lib/categories";
import { errorMessage } from "@/lib/errors";

/** What the form hands over. An empty purpose means none. */
export interface CategoryFormValues {
  name: string;
  is_income: boolean;
  purpose: Purpose | "";
}

/**
 * The form for a category, used both to create one and to edit one. Whether a
 * category is for income or expenses is only chosen when creating it, because
 * its bookings depend on it (rule K3). Only expense categories have a purpose
 * (rule K5). onSubmit saves the values and throws on failure; the form shows
 * the error.
 */
export function CategoryForm({
  initial,
  onSubmit,
}: {
  initial?: CategoryFormValues;
  onSubmit: (values: CategoryFormValues) => Promise<void>;
}) {
  const editing = initial !== undefined;
  const [name, setName] = useState(initial?.name ?? "");
  const [isIncome, setIsIncome] = useState(initial?.is_income ?? false);
  const [purpose, setPurpose] = useState<string>(initial?.purpose || "none");
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  async function submit() {
    if (!name.trim()) {
      setError("Enter a name.");
      return;
    }
    setBusy(true);
    setError(undefined);
    try {
      await onSubmit({ name: name.trim(), is_income: isIncome, purpose: isIncome ? "" : purposeFromChoice(purpose) });
    } catch (e) {
      setError(errorMessage(e));
      setBusy(false);
    }
  }

  return (
    <>
      {!editing && (
        <Segmented>
          <KindButton label="Expense" active={!isIncome} onPress={() => setIsIncome(false)} />
          <KindButton label="Income" active={isIncome} onPress={() => setIsIncome(true)} />
        </Segmented>
      )}
      <FormField label="Name" value={name} onChangeText={setName} />
      {!isIncome && (
        <View className="gap-2">
          <Label>Purpose (optional)</Label>
          <ChoicePicker choices={purposeChoices} selected={purpose} onSelect={setPurpose} emptyText="" />
          <Text variant="muted">Needs are what you must pay, wants what you choose to buy, savings what you put aside.</Text>
        </View>
      )}
      <ErrorMessage message={error} />
      <Button onPress={submit} disabled={busy}>
        <Text>{busy ? "Saving…" : "Save"}</Text>
      </Button>
    </>
  );
}

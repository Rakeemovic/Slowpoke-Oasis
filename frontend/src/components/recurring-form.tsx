import { useCallback, useState } from "react";
import { ActivityIndicator, View } from "react-native";

import { ledgers, pots } from "@/api/endpoints";
import type { IntervalMonths, RecurringPaymentInput } from "@/api/types";
import { ChoicePicker } from "@/components/choice-picker";
import { ErrorMessage } from "@/components/error-message";
import { FormField } from "@/components/form-field";
import { PotPicker } from "@/components/pot-picker";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { errorMessage } from "@/lib/errors";
import { centsToAmountInput, parseAmountToCents } from "@/lib/money";
import { currentMonth, intervalChoices, isMonth } from "@/lib/recurring";

/**
 * The form for a recurring payment, used both to create one and to edit one.
 * Recurring payments are expenses only (rule R2), so only active expense
 * categories are offered. onSubmit saves the values and throws on failure;
 * the form shows the error.
 */
export function RecurringForm({
  ledgerId,
  initial,
  onSubmit,
}: {
  ledgerId: string;
  initial?: RecurringPaymentInput;
  onSubmit: (values: RecurringPaymentInput) => Promise<void>;
}) {
  const categories = useLoader(useCallback(() => ledgers.categories(ledgerId), [ledgerId]));
  const potList = useLoader(useCallback(() => pots.list(ledgerId), [ledgerId]));

  const [name, setName] = useState(initial?.name ?? "");
  const [amount, setAmount] = useState(initial ? centsToAmountInput(initial.amount_cents) : "");
  const [categoryId, setCategoryId] = useState(initial?.category_id);
  // Without a choice, the payment uses the default pot.
  const [potId, setPotId] = useState(initial?.pot_id);
  const [intervalMonths, setIntervalMonths] = useState(String(initial?.interval_months ?? 1));
  const [dueDay, setDueDay] = useState(initial ? String(initial.due_day) : "");
  const [firstMonth, setFirstMonth] = useState(initial?.first_month ?? currentMonth());
  const [lastMonth, setLastMonth] = useState(initial?.last_month ?? "");
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  // An archived category stays visible while it is the one already chosen.
  const choices = (categories.data ?? []).filter((c) => !c.is_income && (!c.archived || c.id === categoryId));

  async function submit() {
    const values: RecurringPaymentInput = {
      name: name.trim(),
      amount_cents: parseAmountToCents(amount) ?? 0,
      category_id: categoryId ?? "",
      pot_id: potId,
      interval_months: Number(intervalMonths) as IntervalMonths,
      due_day: Number(dueDay),
      first_month: firstMonth.trim(),
      last_month: lastMonth.trim(),
    };
    const problem = validate(values);
    if (problem) {
      setError(problem);
      return;
    }
    setBusy(true);
    setError(undefined);
    try {
      await onSubmit(values);
    } catch (e) {
      setError(errorMessage(e));
      setBusy(false);
    }
  }

  return (
    <>
      <FormField label="Name" value={name} onChangeText={setName} placeholder="Rent" />
      <FormField label="Amount (€)" value={amount} onChangeText={setAmount} keyboardType="decimal-pad" placeholder="0.00" />

      <View className="gap-2">
        <Label>Category</Label>
        {categories.loading && !categories.data && <ActivityIndicator />}
        <ErrorMessage message={categories.error} onRetry={categories.reload} />
        <ChoicePicker
          choices={choices.map((c) => ({ id: c.id, label: c.name }))}
          selected={categoryId}
          onSelect={setCategoryId}
          emptyText="No expense categories yet. Add one under Categories."
        />
      </View>

      <PotPicker
        label="Pot"
        pots={potList.data}
        selected={potId}
        onSelect={setPotId}
        loading={potList.loading}
        error={potList.error}
        onRetry={potList.reload}
      />

      <View className="gap-2">
        <Label>How often</Label>
        <ChoicePicker choices={intervalChoices} selected={intervalMonths} onSelect={setIntervalMonths} emptyText="" />
      </View>
      <FormField label="Due day (1–31)" value={dueDay} onChangeText={setDueDay} keyboardType="number-pad" />
      <Text variant="muted">In months without this day, the payment is due on the last day of the month.</Text>
      <FormField label="First month (YYYY-MM)" value={firstMonth} onChangeText={setFirstMonth} autoCapitalize="none" />
      <FormField
        label="Last month (optional, YYYY-MM)"
        value={lastMonth}
        onChangeText={setLastMonth}
        autoCapitalize="none"
      />

      <ErrorMessage message={error} />
      <Button onPress={submit} disabled={busy}>
        <Text>{busy ? "Saving…" : "Save"}</Text>
      </Button>
    </>
  );
}

/** Returns a message for the first invalid input, or undefined if all are fine. */
function validate(v: RecurringPaymentInput): string | undefined {
  if (!v.name) {
    return "Enter a name.";
  }
  if (v.amount_cents <= 0) {
    return "Enter an amount greater than zero, e.g. 12.50.";
  }
  if (!v.category_id) {
    return "Choose a category.";
  }
  if (!Number.isInteger(v.due_day) || v.due_day < 1 || v.due_day > 31) {
    return "Enter a due day between 1 and 31.";
  }
  if (!isMonth(v.first_month) || (v.last_month && !isMonth(v.last_month))) {
    return "Enter months as YYYY-MM, e.g. 2026-12.";
  }
  if (v.last_month && v.last_month < v.first_month) {
    return "The last month must not be before the first month.";
  }
  return undefined;
}

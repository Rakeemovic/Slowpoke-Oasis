import { useState } from "react";

import { ErrorMessage } from "@/components/error-message";
import { FormField } from "@/components/form-field";
import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";
import { isIsoDate } from "@/lib/dates";
import { errorMessage } from "@/lib/errors";
import { centsToAmountInput, parseAmountToCents } from "@/lib/money";

/** What the form hands over. A target of 0 means "no goal", and then the date is empty. */
export interface PotFormValues {
  name: string;
  target_cents: number;
  target_date: string;
}

/**
 * The form for a pot, used both to create one and to edit one. The default
 * pot is monthly and has no savings goal, so its goal fields are hidden.
 * onSubmit saves the values and throws on failure; the form shows the error.
 */
export function PotForm({
  initial,
  hideGoal,
  onSubmit,
}: {
  initial?: PotFormValues;
  hideGoal?: boolean;
  onSubmit: (values: PotFormValues) => Promise<void>;
}) {
  const [name, setName] = useState(initial?.name ?? "");
  const [target, setTarget] = useState(initial?.target_cents ? centsToAmountInput(initial.target_cents) : "");
  const [targetDate, setTargetDate] = useState(initial?.target_date ?? "");
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  async function submit() {
    const problem = validate(name, target, targetDate);
    if (problem) {
      setError(problem);
      return;
    }
    const cents = target.trim() ? parseAmountToCents(target)! : 0;
    setBusy(true);
    setError(undefined);
    try {
      await onSubmit({ name: name.trim(), target_cents: cents, target_date: cents > 0 ? targetDate.trim() : "" });
    } catch (e) {
      setError(errorMessage(e));
      setBusy(false);
    }
  }

  return (
    <>
      <FormField label="Name" value={name} onChangeText={setName} />
      {!hideGoal && (
        <>
          <FormField
            label="Savings goal (€, optional)"
            value={target}
            onChangeText={setTarget}
            keyboardType="decimal-pad"
            placeholder="0.00"
          />
          <FormField
            label="Goal date (YYYY-MM-DD, optional)"
            value={targetDate}
            onChangeText={setTargetDate}
            autoCapitalize="none"
          />
        </>
      )}
      <ErrorMessage message={error} />
      <Button onPress={submit} disabled={busy}>
        <Text>{busy ? "Saving…" : "Save"}</Text>
      </Button>
    </>
  );
}

/** Returns a message for the first invalid input, or undefined if all are fine (rules T1, T3, T4). */
function validate(name: string, target: string, targetDate: string): string | undefined {
  if (!name.trim()) {
    return "Enter a name.";
  }
  if (target.trim() && parseAmountToCents(target) === null) {
    return "Enter the goal as an amount greater than zero, with at most two decimal places, or leave it empty.";
  }
  if (targetDate.trim() && !target.trim()) {
    return "A goal date needs a savings goal.";
  }
  if (targetDate.trim() && !isIsoDate(targetDate.trim())) {
    return "Enter the goal date as YYYY-MM-DD.";
  }
  return undefined;
}

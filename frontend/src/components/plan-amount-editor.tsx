import { useState } from "react";
import { View } from "react-native";

import type { PlanScope } from "@/api/types";
import { ErrorMessage } from "@/components/error-message";
import { FormField } from "@/components/form-field";
import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";
import { errorMessage } from "@/lib/errors";
import { centsToAmountInput, parseAmountToCents } from "@/lib/money";

/**
 * Changes the budgeted amount of one category. Every change asks which months
 * it affects (rule P5), so there is one button per scope instead of a
 * preselected choice that is easy to overlook.
 */
export function PlanAmountEditor({
  categoryName,
  plannedCents,
  onSave,
  onRemove,
  onCancel,
}: {
  categoryName: string;
  /** 0 when the category is not planned yet; then there is nothing to remove. */
  plannedCents: number;
  onSave: (cents: number, scope: PlanScope) => Promise<void>;
  onRemove: (scope: PlanScope) => Promise<void>;
  onCancel: () => void;
}) {
  const [amount, setAmount] = useState(plannedCents > 0 ? centsToAmountInput(plannedCents) : "");
  const [removing, setRemoving] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  async function run(action: () => Promise<void>) {
    setBusy(true);
    setError(undefined);
    try {
      await action();
    } catch (e) {
      setError(errorMessage(e));
      setBusy(false);
    }
  }

  function save(scope: PlanScope) {
    const cents = parseAmountToCents(amount);
    if (cents === null) {
      setError("Enter an amount greater than zero, with at most two decimal places.");
      return;
    }
    void run(() => onSave(cents, scope));
  }

  if (removing) {
    return (
      <View className="gap-2">
        <Text>{`Remove “${categoryName}” from the plan?`}</Text>
        <ScopeButtons busy={busy} destructive onPress={(scope) => void run(() => onRemove(scope))} />
        <Button variant="ghost" onPress={() => setRemoving(false)} disabled={busy}>
          <Text>Cancel</Text>
        </Button>
        <ErrorMessage message={error} />
      </View>
    );
  }

  return (
    <View className="gap-2">
      <FormField
        label={`Planned for ${categoryName} (€)`}
        value={amount}
        onChangeText={setAmount}
        keyboardType="decimal-pad"
        placeholder="0.00"
        autoFocus
      />
      <ScopeButtons busy={busy} onPress={save} />
      <View className="flex-row gap-2">
        {plannedCents > 0 && (
          <Button className="flex-1" variant="outline" onPress={() => setRemoving(true)} disabled={busy}>
            <Text className="text-destructive">Remove from plan</Text>
          </Button>
        )}
        <Button className="flex-1" variant="ghost" onPress={onCancel} disabled={busy}>
          <Text>Cancel</Text>
        </Button>
      </View>
      <ErrorMessage message={error} />
    </View>
  );
}

function ScopeButtons({
  busy,
  destructive,
  onPress,
}: {
  busy: boolean;
  destructive?: boolean;
  onPress: (scope: PlanScope) => void;
}) {
  const variant = destructive ? "destructive" : "default";
  return (
    <View className="flex-row gap-2">
      <Button className="flex-1" variant={variant} onPress={() => onPress("this_month")} disabled={busy}>
        <Text>Only this month</Text>
      </Button>
      <Button className="flex-1" variant={variant} onPress={() => onPress("from_now_on")} disabled={busy}>
        <Text>This and coming months</Text>
      </Button>
    </View>
  );
}

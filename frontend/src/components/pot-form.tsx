import { useState } from "react";
import { View } from "react-native";

import { ErrorMessage } from "@/components/error-message";
import { FormField } from "@/components/form-field";
import { KindButton, Segmented } from "@/components/kind-button";
import { SwitchRow } from "@/components/switch-row";
import { RoundUpChoice } from "@/components/round-up-choice";
import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";
import { isIsoDate } from "@/lib/dates";
import { errorMessage } from "@/lib/errors";
import { centsToAmountInput, parseAmountToCents } from "@/lib/money";

/**
 * What the form hands over. 0 and "" mean "not set": no target, no date, no
 * fixed rate. A fixed rate and a date never come together (rule T6).
 */
export interface PotFormValues {
  name: string;
  target_cents: number;
  target_date: string;
  monthly_rate_cents: number;
  is_separate_account: boolean;
  /** Only set when the form offers round-ups (rule T10). */
  round_up_multiplier?: number;
}

/**
 * The goal of a pot in one of three kinds:
 * - none: the pot only holds money;
 * - target: an amount to reach, optionally by a date (then the app works out
 *   how much to save per month; without a date the pot only shows progress);
 * - rate: a fixed amount per month, optionally up to a target.
 */
type GoalKind = "none" | "target" | "rate";

const goalHints: Record<GoalKind, string> = {
  none: "The pot only holds money.",
  target: "With a date, the app works out how much to save each month. Without one, the pot only shows its progress.",
  rate: "This amount is saved every payday period, until the target is reached if there is one.",
};

/**
 * The form for a pot, used both to create one and to edit one. The default
 * pot is monthly and has no savings goal, so its goal fields are hidden. The
 * default pot and reserves can never be a separate account at the bank, so
 * `hideAccount` hides that switch. Round-ups are only offered when `roundUp`
 * is given, since only some pots of personal ledgers may collect them. onSubmit saves the values and throws on
 * failure; the form shows the error.
 */
export function PotForm({
  initial,
  hideGoal,
  hideAccount,
  roundUp,
  onSubmit,
}: {
  initial?: PotFormValues;
  hideGoal?: boolean;
  hideAccount?: boolean;
  /** Offers round-ups; `replaces` names the pot that collects now, if another one does. */
  roundUp?: { replaces?: string };
  onSubmit: (values: PotFormValues) => Promise<void>;
}) {
  const [name, setName] = useState(initial?.name ?? "");
  const [kind, setKind] = useState<GoalKind>(initialKind(initial));
  const [target, setTarget] = useState(initial?.target_cents ? centsToAmountInput(initial.target_cents) : "");
  const [targetDate, setTargetDate] = useState(initial?.target_date ?? "");
  const [rate, setRate] = useState(initial?.monthly_rate_cents ? centsToAmountInput(initial.monthly_rate_cents) : "");
  const [separate, setSeparate] = useState(initial?.is_separate_account ?? false);
  const [multiplier, setMultiplier] = useState(initial?.round_up_multiplier ?? 0);
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  async function submit() {
    const result = collect(name, hideGoal ? "none" : kind, { target, targetDate, rate });
    if (typeof result === "string") {
      setError(result);
      return;
    }
    result.is_separate_account = !hideAccount && separate;
    if (roundUp) {
      result.round_up_multiplier = multiplier;
    }
    setBusy(true);
    setError(undefined);
    try {
      await onSubmit(result);
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
          <View className="gap-1.5">
            <Text className="text-sm font-medium">Goal</Text>
            <Segmented>
              <KindButton label="No goal" active={kind === "none"} onPress={() => setKind("none")} />
              <KindButton label="Target" active={kind === "target"} onPress={() => setKind("target")} />
              <KindButton label="Monthly amount" active={kind === "rate"} onPress={() => setKind("rate")} />
            </Segmented>
            <Text variant="muted" className="text-sm">
              {goalHints[kind]}
            </Text>
          </View>
          {kind === "rate" && (
            <FormField
              label="Save per month (€)"
              value={rate}
              onChangeText={setRate}
              keyboardType="decimal-pad"
              placeholder="0.00"
            />
          )}
          {kind !== "none" && (
            <FormField
              label={kind === "rate" ? "Target amount (€, optional)" : "Target amount (€)"}
              value={target}
              onChangeText={setTarget}
              keyboardType="decimal-pad"
              placeholder="0.00"
            />
          )}
          {kind === "target" && (
            <FormField
              label="Goal date (YYYY-MM-DD, optional)"
              value={targetDate}
              onChangeText={setTargetDate}
              autoCapitalize="none"
            />
          )}
        </>
      )}
      {!hideAccount && (
        <SwitchRow
          label="Separate account at my bank"
          description="Switch this on if the pot is a real sub-account at your bank. Its money is then not part of your estimated account balance, which shows what your main account holds."
          checked={separate}
          onCheckedChange={setSeparate}
        />
      )}
      {roundUp && <RoundUpChoice value={multiplier} onChange={setMultiplier} replaces={roundUp.replaces} />}
      <ErrorMessage message={error} />
      <Button onPress={submit} disabled={busy}>
        <Text>{busy ? "Saving…" : "Save"}</Text>
      </Button>
    </>
  );
}

function initialKind(initial?: PotFormValues): GoalKind {
  if (initial?.monthly_rate_cents) {
    return "rate";
  }
  return initial?.target_cents ? "target" : "none";
}

/**
 * Checks the inputs and turns them into the values to save: only the fields
 * of the chosen kind count, the others become "not set". Returns a message
 * for the first invalid input instead (rules T1, T3, T4, T6).
 */
function collect(
  name: string,
  kind: GoalKind,
  input: { target: string; targetDate: string; rate: string },
): PotFormValues | string {
  if (!name.trim()) {
    return "Enter a name.";
  }
  const values: PotFormValues = {
    name: name.trim(),
    target_cents: 0,
    target_date: "",
    monthly_rate_cents: 0,
    is_separate_account: false,
  };
  if (kind === "none") {
    return values;
  }

  if (kind === "rate") {
    const rate = parseAmountToCents(input.rate);
    if (rate === null) {
      return "Enter the monthly amount greater than zero, with at most two decimal places.";
    }
    values.monthly_rate_cents = rate;
  }
  const hasTarget = input.target.trim() !== "";
  if (kind === "target" || hasTarget) {
    const target = parseAmountToCents(input.target);
    if (target === null) {
      return kind === "target"
        ? "Enter the target as an amount greater than zero, with at most two decimal places."
        : "Enter the target as an amount greater than zero, with at most two decimal places, or leave it empty.";
    }
    values.target_cents = target;
  }
  if (kind === "target" && input.targetDate.trim()) {
    if (!isIsoDate(input.targetDate.trim())) {
      return "Enter the goal date as YYYY-MM-DD.";
    }
    values.target_date = input.targetDate.trim();
  }
  return values;
}

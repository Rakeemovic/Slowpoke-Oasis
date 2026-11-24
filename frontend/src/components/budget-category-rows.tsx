import { Pressable, View } from "react-native";

import type { BudgetSavingsLine, PlanLine } from "@/api/types";
import { ProgressRow } from "@/components/progress-row";
import { Text } from "@/components/ui/text";
import { type BudgetGroup, recurringText, reserveDueText, reserveText, type SubLineText } from "@/lib/budget";
import { savingsLine } from "@/lib/goals";
import { formatCents } from "@/lib/money";
import { reachedProgress, spentProgress } from "@/lib/progress";
import { cn } from "@/lib/utils";

/**
 * A category of the budget: the freely planned part on top, and indented
 * below it the monthly payments, the reserve rates and the payments due
 * from a reserve. Only the top row can be pressed, to change the plan.
 */
export function BudgetCategoryRows({
  group,
  editable,
  onPress,
}: {
  group: BudgetGroup;
  editable: boolean;
  onPress: () => void;
}) {
  const { line, dues } = group;
  return (
    <View>
      <Pressable
        className="active:bg-accent web:hover:bg-accent/50"
        disabled={!editable}
        onPress={onPress}
        role={editable ? "button" : undefined}
      >
        <PlanRow line={line} />
      </Pressable>
      {line.recurring.map((r) => (
        <SubRow key={`${r.payment_id}-${r.due_on}`} icon="↻" name={r.name} text={recurringText(r)} />
      ))}
      {line.reserves.map((r) => (
        <SubRow key={r.payment_id} icon="→" name={`Save for ${r.name}`} text={reserveText(r)} />
      ))}
      {dues.map((d) => (
        <Text key={`${d.payment_id}-${d.due_on}`} variant="muted" className="px-4 pb-3 pl-10 text-sm">
          {reserveDueText(d)}
        </Text>
      ))}
    </View>
  );
}

/** The category's row is its own progress bar: spent (or received) against the plan. */
function PlanRow({ line }: { line: PlanLine }) {
  const isPlanned = line.planned_cents > 0;
  const bar = line.is_income ? reachedProgress(line.actual_cents, line.planned_cents) : spentProgress(line.actual_cents, line.planned_cents);
  let progress: string;
  if (isPlanned) {
    progress = `${formatCents(line.actual_cents)} of ${formatCents(line.planned_cents)}`;
  } else if (line.actual_cents > 0) {
    progress = `${formatCents(line.actual_cents)}, not planned`;
  } else {
    // The category is only here for its payments or reserves.
    progress = "Nothing else planned";
  }
  return (
    <ProgressRow progress={bar} label={line.name} className="flex-row items-center gap-3 px-4 py-3">
      <View className="flex-1 gap-0.5">
        <Text className="font-medium">{line.name}</Text>
        <Text variant="muted">{progress}</Text>
      </View>
      {isPlanned && <Difference line={line} />}
    </ProgressRow>
  );
}

/** A line below a category, indented, with a progress bar of its own. */
function SubRow({ icon, name, text }: { icon: string; name: string; text: SubLineText }) {
  return (
    <ProgressRow progress={text.bar} label={name} className="flex-row items-center gap-3 py-2 pl-10 pr-4">
      <View className="flex-1 gap-0.5">
        <Text className="text-sm">{`${icon} ${name}`}</Text>
        <Text variant="muted" className="text-sm">
          {text.progress}
        </Text>
      </View>
      {text.status && <Text className={cn("text-sm font-semibold", text.warn && "text-destructive")}>{text.status}</Text>}
    </ProgressRow>
  );
}

/**
 * What is left of the plan. Too much spent or too little received is shown
 * in the warning colour; the server already gives the sign for both kinds.
 */
function Difference({ line }: { line: PlanLine }) {
  const short = line.difference_cents < 0;
  const amount = formatCents(Math.abs(line.difference_cents));
  let label: string;
  if (line.is_income) {
    label = short ? `${amount} to come` : `${amount} more`;
  } else {
    label = short ? `${amount} over` : `${amount} left`;
  }
  return <Text className={cn("font-semibold", short && "text-destructive")}>{label}</Text>;
}

/** A savings goal of the budget: its rate this month, as a progress bar of what was saved. */
export function SavingsRow({ line }: { line: BudgetSavingsLine }) {
  const text = savingsLine(line);
  return (
    <ProgressRow progress={text.bar} label={line.name} className="flex-row items-center gap-3 px-4 py-3">
      <View className="flex-1 gap-0.5">
        <Text className="font-medium">{line.name}</Text>
        <Text variant="muted">{text.progress}</Text>
      </View>
      {text.status && <Text className="font-semibold">{text.status}</Text>}
    </ProgressRow>
  );
}

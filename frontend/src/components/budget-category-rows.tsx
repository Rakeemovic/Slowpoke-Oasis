import { Pressable, View } from "react-native";

import type { PlanLine } from "@/api/types";
import { Text } from "@/components/ui/text";
import { type BudgetGroup, recurringText, reserveDueText, reserveText, type SubLineText } from "@/lib/budget";
import { formatCents } from "@/lib/money";
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

function PlanRow({ line }: { line: PlanLine }) {
  const isPlanned = line.planned_cents > 0;
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
    <View className="flex-row items-center gap-3 px-4 py-3">
      <View className="flex-1 gap-0.5">
        <Text className="font-medium">{line.name}</Text>
        <Text variant="muted">{progress}</Text>
      </View>
      {isPlanned && <Difference line={line} />}
    </View>
  );
}

function SubRow({ icon, name, text }: { icon: string; name: string; text: SubLineText }) {
  return (
    <View className="flex-row items-center gap-3 px-4 pb-3 pl-10">
      <View className="flex-1 gap-0.5">
        <Text className="text-sm">{`${icon} ${name}`}</Text>
        <Text variant="muted" className="text-sm">
          {text.progress}
        </Text>
      </View>
      {text.status && <Text className={cn("text-sm font-semibold", text.warn && "text-destructive")}>{text.status}</Text>}
    </View>
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

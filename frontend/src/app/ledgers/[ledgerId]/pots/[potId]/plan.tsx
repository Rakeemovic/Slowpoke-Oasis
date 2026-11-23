import { Stack, useLocalSearchParams } from "expo-router";
import { useCallback, useState } from "react";
import { ActivityIndicator, Pressable, View } from "react-native";

import { ledgers, pots } from "@/api/endpoints";
import type { Category, PlanLine, PlanScope, Pot, PotSummary } from "@/api/types";
import { ChoicePicker } from "@/components/choice-picker";
import { ErrorMessage } from "@/components/error-message";
import { HeaderActions } from "@/components/header-actions";
import { PlanAmountEditor } from "@/components/plan-amount-editor";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { formatDate, formatMonth, shiftMonth, today } from "@/lib/dates";
import { formatCents } from "@/lib/money";
import { cn } from "@/lib/utils";

interface PlanData {
  pot?: Pot;
  summary: PotSummary;
  categories: Category[];
}

export default function PlanScreen() {
  const { ledgerId, potId } = useLocalSearchParams<{ ledgerId: string; potId: string }>();
  // undefined means "the current month", which the server works out from the
  // app's date and the payday (rule P4). Its answer tells us which month that is.
  const [month, setMonth] = useState<string>();
  const [currentMonth, setCurrentMonth] = useState<string>();

  const load = useCallback(async (): Promise<PlanData> => {
    const [potList, summary, categories] = await Promise.all([
      pots.list(ledgerId),
      pots.summary(ledgerId, potId, today(), month),
      ledgers.categories(ledgerId),
    ]);
    if (month === undefined) {
      setCurrentMonth(summary.month);
    }
    return { pot: potList.find((p) => p.id === potId), summary, categories };
  }, [ledgerId, potId, month]);
  const { data, error, loading, reload } = useLoader(load);

  const monthly = data?.pot?.kind === "monthly";
  const summary = data?.summary;
  // The loader keeps the previous answer while loading, so nothing is
  // editable until the answer for the chosen month has arrived.
  const shownMonth = summary?.month;
  const upToDate = !loading && summary !== undefined && (month === undefined || shownMonth === month);
  // Rule P5: only the plan of the current month can be changed.
  const editable = upToDate && (monthly ? summary.plan_source === "month" && shownMonth === currentMonth : true);

  return (
    <Screen>
      <Stack.Screen options={{ title: data?.pot ? `Plan · ${data.pot.name}` : "Plan", headerRight: () => <HeaderActions /> }} />
      {monthly && shownMonth && (
        <MonthSwitcher
          month={shownMonth}
          isCurrent={shownMonth === currentMonth}
          disabled={loading}
          onShift={(delta) => setMonth(shiftMonth(shownMonth, delta))}
          onCurrent={() => setMonth(undefined)}
        />
      )}
      {loading && !data && <ActivityIndicator />}
      <ErrorMessage message={error} onRetry={reload} />
      {data && summary && (
        <PlanContent
          ledgerId={ledgerId}
          potId={potId}
          monthly={monthly}
          summary={summary}
          categories={data.categories}
          editable={editable}
          onChanged={reload}
        />
      )}
    </Screen>
  );
}

function MonthSwitcher({
  month,
  isCurrent,
  disabled,
  onShift,
  onCurrent,
}: {
  month: string;
  isCurrent: boolean;
  disabled: boolean;
  onShift: (delta: number) => void;
  onCurrent: () => void;
}) {
  return (
    <View className="gap-2">
      <View className="flex-row items-center gap-2">
        <Button variant="outline" size="sm" onPress={() => onShift(-1)} disabled={disabled} aria-label="Previous month">
          <Text>‹</Text>
        </Button>
        <Text className="flex-1 text-center text-lg font-semibold">{formatMonth(month)}</Text>
        <Button variant="outline" size="sm" onPress={() => onShift(1)} disabled={disabled} aria-label="Next month">
          <Text>›</Text>
        </Button>
      </View>
      {!isCurrent && (
        <Button variant="ghost" size="sm" onPress={onCurrent} disabled={disabled}>
          <Text>Back to this month</Text>
        </Button>
      )}
    </View>
  );
}

/** Says why a month cannot be changed, or nothing when it can. */
function sourceNote(summary: PotSummary, editable: boolean): string | undefined {
  switch (summary.plan_source) {
    case "preview":
      return "Preview from the plan for the coming months. It becomes this month's plan when the month begins.";
    case "none":
      return "No plan was made for this month.";
    case "month":
      return editable ? undefined : "Past months cannot be changed.";
    default:
      return undefined;
  }
}

function PlanContent({
  ledgerId,
  potId,
  monthly,
  summary,
  categories,
  editable,
  onChanged,
}: {
  ledgerId: string;
  potId: string;
  monthly: boolean;
  summary: PotSummary;
  categories: Category[];
  editable: boolean;
  onChanged: () => Promise<void>;
}) {
  // The category whose amount is being edited, at most one at a time.
  const [editing, setEditing] = useState<string>();
  const [adding, setAdding] = useState(false);

  const expenses = summary.lines.filter((l) => !l.is_income);
  const income = summary.lines.filter((l) => l.is_income);
  const planned = new Set(summary.lines.filter((l) => l.planned_cents > 0).map((l) => l.category_id));
  // Archived categories accept no bookings (rule K8), so planning them makes no sense.
  const unplanned = categories.filter((c) => !c.archived && !planned.has(c.id));
  const editingNew = editing !== undefined && !summary.lines.some((l) => l.category_id === editing);
  const note = sourceNote(summary, editable);

  async function save(categoryId: string, cents: number, scope?: PlanScope) {
    await pots.setPlanned(ledgerId, potId, categoryId, cents, today(), scope);
    setEditing(undefined);
    await onChanged();
  }

  async function remove(categoryId: string, scope?: PlanScope) {
    await pots.removePlanned(ledgerId, potId, categoryId, today(), scope);
    setEditing(undefined);
    await onChanged();
  }

  function editor(categoryId: string, name: string, plannedCents: number) {
    return (
      <PlanAmountEditor
        key={categoryId}
        categoryName={name}
        plannedCents={plannedCents}
        monthly={monthly}
        onSave={(cents, scope) => save(categoryId, cents, scope)}
        onRemove={(scope) => remove(categoryId, scope)}
        onCancel={() => setEditing(undefined)}
      />
    );
  }

  return (
    <>
      {summary.from && summary.to && (
        <Text variant="muted" className="text-center">
          {formatDate(summary.from)} – {formatDate(summary.to)}
        </Text>
      )}
      {note && <Text variant="muted">{note}</Text>}
      <Totals summary={summary} />
      {summary.lines.length === 0 && <Text variant="muted">Nothing planned or booked yet.</Text>}
      {[
        { title: "Expenses", lines: expenses },
        { title: "Income", lines: income },
      ].map(
        (group) =>
          group.lines.length > 0 && (
            <View key={group.title} className="gap-2">
              <Text variant="muted">{group.title}</Text>
              <Card className="py-2">
                <CardContent className="gap-0 px-0">
                  {group.lines.map((line, index) => (
                    <View key={line.category_id} className={cn(index > 0 && "border-t border-border")}>
                      <Pressable
                        className="active:bg-accent web:hover:bg-accent/50"
                        disabled={!editable}
                        onPress={() => setEditing(editing === line.category_id ? undefined : line.category_id)}
                        role={editable ? "button" : undefined}
                      >
                        <PlanRow line={line} />
                      </Pressable>
                      {editing === line.category_id && (
                        <View className="px-4 pb-3">{editor(line.category_id, line.name, line.planned_cents)}</View>
                      )}
                    </View>
                  ))}
                </CardContent>
              </Card>
            </View>
          ),
      )}
      {editable && (
        <View className="gap-2">
          {editingNew ? (
            editor(editing, categories.find((c) => c.id === editing)?.name ?? "", 0)
          ) : adding ? (
            <>
              <Text>Which category do you want to plan?</Text>
              <ChoicePicker
                choices={unplanned.map((c) => ({ id: c.id, label: c.name }))}
                onSelect={(id) => {
                  setAdding(false);
                  setEditing(id);
                }}
                emptyText="Every category is planned already."
              />
              <Button variant="ghost" onPress={() => setAdding(false)}>
                <Text>Cancel</Text>
              </Button>
            </>
          ) : (
            <Button variant="outline" onPress={() => setAdding(true)}>
              <Text>Plan a category</Text>
            </Button>
          )}
        </View>
      )}
    </>
  );
}

/** Planned against actual for all expenses and all income, plus transfers if any. */
function Totals({ summary }: { summary: PotSummary }) {
  const sum = (lines: PlanLine[], field: "planned_cents" | "actual_cents") => lines.reduce((total, l) => total + l[field], 0);
  const expenses = summary.lines.filter((l) => !l.is_income);
  const income = summary.lines.filter((l) => l.is_income);
  return (
    <Card>
      <CardContent className="gap-1">
        <Text>{`Spent ${formatCents(sum(expenses, "actual_cents"))} of ${formatCents(sum(expenses, "planned_cents"))} planned`}</Text>
        {income.length > 0 && (
          <Text>{`Received ${formatCents(sum(income, "actual_cents"))} of ${formatCents(sum(income, "planned_cents"))} planned`}</Text>
        )}
        {(summary.transfers_in_cents > 0 || summary.transfers_out_cents > 0) && (
          <Text variant="muted">
            {`Moved in ${formatCents(summary.transfers_in_cents)}, moved out ${formatCents(summary.transfers_out_cents)}`}
          </Text>
        )}
      </CardContent>
    </Card>
  );
}

function PlanRow({ line }: { line: PlanLine }) {
  const isPlanned = line.planned_cents > 0;
  const progress = isPlanned
    ? `${formatCents(line.actual_cents)} of ${formatCents(line.planned_cents)}`
    : `${formatCents(line.actual_cents)}, not planned`;
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

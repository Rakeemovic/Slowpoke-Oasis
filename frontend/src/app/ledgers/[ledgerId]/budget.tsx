import { Stack, useLocalSearchParams } from "expo-router";
import { useCallback, useState } from "react";
import { ActivityIndicator, View } from "react-native";

import { ledgers, pots } from "@/api/endpoints";
import type { Budget, Category, PlanScope } from "@/api/types";
import { BudgetCategoryRows } from "@/components/budget-category-rows";
import { ChoicePicker } from "@/components/choice-picker";
import { ErrorMessage } from "@/components/error-message";
import { HeaderActions } from "@/components/header-actions";
import { PlanAmountEditor } from "@/components/plan-amount-editor";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { type BudgetGroup, budgetGroups, budgetTotals } from "@/lib/budget";
import { formatDate, formatMonth, shiftMonth, today } from "@/lib/dates";
import { formatCents } from "@/lib/money";
import { cn } from "@/lib/utils";

interface BudgetData {
  budget: Budget;
  categories: Category[];
}

/** The budget of the ledger's main pot, one month at a time. */
export default function BudgetScreen() {
  const { ledgerId } = useLocalSearchParams<{ ledgerId: string }>();
  // undefined means "the current month", which the server works out from the
  // app's date and the payday (rule P4).
  const [month, setMonth] = useState<string>();

  const load = useCallback(async (): Promise<BudgetData> => {
    const [budget, categories] = await Promise.all([
      ledgers.budget(ledgerId, today(), month),
      ledgers.categories(ledgerId),
    ]);
    return { budget, categories };
  }, [ledgerId, month]);
  const { data, error, loading, reload } = useLoader(load);

  const budget = data?.budget;
  // The loader keeps the previous answer while loading, so nothing is
  // editable until the answer for the chosen month has arrived.
  const upToDate = !loading && budget !== undefined && (month === undefined || budget.month === month);
  // Rule P5: only the budget of the current month can be changed.
  const editable = upToDate && budget.plan_source === "month" && budget.month === budget.current_month;

  return (
    <Screen>
      <Stack.Screen options={{ title: "Budget", headerRight: () => <HeaderActions /> }} />
      {budget && (
        <MonthSwitcher
          month={budget.month}
          isCurrent={budget.month === budget.current_month}
          disabled={loading}
          onShift={(delta) => setMonth(shiftMonth(budget.month, delta))}
          onCurrent={() => setMonth(undefined)}
        />
      )}
      {loading && !data && <ActivityIndicator />}
      <ErrorMessage message={error} onRetry={reload} />
      {data && (
        <BudgetContent
          ledgerId={ledgerId}
          budget={data.budget}
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
function sourceNote(budget: Budget, editable: boolean): string | undefined {
  switch (budget.plan_source) {
    case "preview":
      return "Preview from the budget for the coming months. It becomes this month's budget when the month begins.";
    case "none":
      return "No budget was made for this month.";
    case "month":
      return editable ? undefined : "Past months cannot be changed.";
    default:
      return undefined;
  }
}

function BudgetContent({
  ledgerId,
  budget,
  categories,
  editable,
  onChanged,
}: {
  ledgerId: string;
  budget: Budget;
  categories: Category[];
  editable: boolean;
  onChanged: () => Promise<void>;
}) {
  // The category whose amount is being edited, at most one at a time.
  const [editing, setEditing] = useState<string>();
  const [adding, setAdding] = useState(false);

  const groups = budgetGroups(budget, categories);
  const expenses = groups.filter((g) => !g.line.is_income);
  const income = groups.filter((g) => g.line.is_income);
  const planned = new Set(budget.categories.filter((c) => c.planned_cents > 0).map((c) => c.category_id));
  // Archived categories accept no bookings (rule K8), so planning them makes no sense.
  const unplanned = categories.filter((c) => !c.archived && !planned.has(c.id));
  const editingNew = editing !== undefined && !groups.some((g) => g.line.category_id === editing);
  const note = sourceNote(budget, editable);

  async function save(categoryId: string, cents: number, scope: PlanScope) {
    await pots.setPlanned(ledgerId, budget.pot_id, categoryId, cents, today(), scope);
    setEditing(undefined);
    await onChanged();
  }

  async function remove(categoryId: string, scope: PlanScope) {
    await pots.removePlanned(ledgerId, budget.pot_id, categoryId, today(), scope);
    setEditing(undefined);
    await onChanged();
  }

  function editor(categoryId: string, name: string, plannedCents: number) {
    return (
      <PlanAmountEditor
        key={categoryId}
        categoryName={name}
        plannedCents={plannedCents}
        onSave={(cents, scope) => save(categoryId, cents, scope)}
        onRemove={(scope) => remove(categoryId, scope)}
        onCancel={() => setEditing(undefined)}
      />
    );
  }

  return (
    <>
      {budget.from && budget.to && (
        <Text variant="muted" className="text-center">
          {formatDate(budget.from)} – {formatDate(budget.to)}
        </Text>
      )}
      {note && <Text variant="muted">{note}</Text>}
      <Totals budget={budget} />
      {groups.length === 0 && <Text variant="muted">Nothing planned or booked yet.</Text>}
      {[
        { title: "Expenses", groups: expenses },
        { title: "Income", groups: income },
      ].map(
        (section) =>
          section.groups.length > 0 && (
            <View key={section.title} className="gap-2">
              <Text variant="muted">{section.title}</Text>
              <Card className="py-2">
                <CardContent className="gap-0 px-0">
                  {section.groups.map((group: BudgetGroup, index) => (
                    <View key={group.line.category_id} className={cn(index > 0 && "border-t border-border")}>
                      <BudgetCategoryRows
                        group={group}
                        editable={editable}
                        onPress={() =>
                          setEditing(editing === group.line.category_id ? undefined : group.line.category_id)
                        }
                      />
                      {editing === group.line.category_id && (
                        <View className="px-4 pb-3">
                          {editor(group.line.category_id, group.line.name, group.line.planned_cents)}
                        </View>
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
              <Text>Which category do you want to budget?</Text>
              <ChoicePicker
                choices={unplanned.map((c) => ({ id: c.id, label: c.name }))}
                onSelect={(id) => {
                  setAdding(false);
                  setEditing(id);
                }}
                emptyText="Every category has a budget already."
              />
              <Button variant="ghost" onPress={() => setAdding(false)}>
                <Text>Cancel</Text>
              </Button>
            </>
          ) : (
            <Button variant="outline" onPress={() => setAdding(true)}>
              <Text>Budget a category</Text>
            </Button>
          )}
        </View>
      )}
    </>
  );
}

/** The whole month: free plan, monthly payments and reserve rates together. */
function Totals({ budget }: { budget: Budget }) {
  const t = budgetTotals(budget);
  return (
    <Card>
      <CardContent className="gap-1">
        <Text>{`Spent ${formatCents(t.spent)} of ${formatCents(t.plannedExpenses)} planned`}</Text>
        {t.hasIncome && <Text>{`Received ${formatCents(t.received)} of ${formatCents(t.plannedIncome)} planned`}</Text>}
      </CardContent>
    </Card>
  );
}

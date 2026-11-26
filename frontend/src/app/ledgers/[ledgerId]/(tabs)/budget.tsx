import { ChartColumn, ChartPie } from "lucide-react-native";
import { useColorScheme } from "nativewind";
import { useCallback, useState } from "react";
import { View } from "react-native";

import { ledgers, pots } from "@/api/endpoints";
import type { Budget, Category, PlanScope } from "@/api/types";
import { BudgetCategoryRows, SavingsRow } from "@/components/budget-category-rows";
import { ChoicePicker } from "@/components/choice-picker";
import { ErrorMessage } from "@/components/error-message";
import { EmptyState } from "@/components/empty-state";
import { ListRow, ListSection } from "@/components/list";
import { MonthSwitcher } from "@/components/month-switcher";
import { CircularProgress } from "@/components/magic/circular-progress";
import { PlanAmountEditor } from "@/components/plan-amount-editor";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { type BudgetGroup, budgetGroups, budgetTotals } from "@/lib/budget";
import { formatDate, monthName, shiftMonth, today } from "@/lib/dates";
import { hueFor } from "@/lib/hues";
import { useLedger } from "@/lib/ledger-context";
import { formatCents } from "@/lib/money";
import { THEME } from "@/lib/theme";

interface BudgetData {
  budget: Budget;
  categories: Category[];
}

/** The budget of the ledger's main pot, one month at a time. */
export default function BudgetScreen() {
  const { ledgerId } = useLedger();
  // undefined means "the current month", which the server works out from the
  // app's date and the payday (rule P4).
  const [month, setMonth] = useState<string>();

  const load = useCallback(async (): Promise<BudgetData> => {
    const [budget, categories] = await Promise.all([ledgers.budget(ledgerId, today(), month), ledgers.categories(ledgerId)]);
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
      {budget && (
        <MonthSwitcher
          month={budget.month}
          isCurrent={budget.month === budget.current_month}
          disabled={loading}
          onShift={(delta) => setMonth(shiftMonth(budget.month, delta))}
          onCurrent={() => setMonth(undefined)}
        />
      )}
      {loading && !data && <Skeleton className="h-64 w-full rounded-2xl" />}
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
      {/* A month that has not begun has nothing to look back on. */}
      {budget.month <= budget.current_month && <InsightsLink ledgerId={ledgerId} month={budget.month} />}
      {groups.length === 0 && budget.savings.length === 0 && (
        <EmptyState
          icon={ChartPie}
          title="Nothing planned or booked yet"
          text={editable ? "Give a category a monthly budget to see how much is left." : undefined}
        />
      )}
      {[
        { title: "Expenses", groups: expenses },
        { title: "Income", groups: income },
      ].map(
        (section) =>
          section.groups.length > 0 && (
            <ListSection key={section.title} title={section.title}>
              {section.groups.map((group: BudgetGroup) => (
                <View key={group.line.category_id}>
                  <BudgetCategoryRows
                    group={group}
                    editable={editable}
                    onPress={() => setEditing(editing === group.line.category_id ? undefined : group.line.category_id)}
                  />
                  {editing === group.line.category_id && (
                    <View className="px-4 pb-3">{editor(group.line.category_id, group.line.name, group.line.planned_cents)}</View>
                  )}
                </View>
              ))}
            </ListSection>
          ),
      )}
      {budget.savings.length > 0 && (
        <ListSection title="Savings">
          {budget.savings.map((line) => (
            <SavingsRow key={line.pot_id} line={line} />
          ))}
        </ListSection>
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

/** Leads to the purposes of the month shown here, so it need not be picked again. */
function InsightsLink({ ledgerId, month }: { ledgerId: string; month: string }) {
  return (
    <ListSection>
      <ListRow
        title="Insights"
        subtitle={`Needs, wants and savings in ${monthName(month)}`}
        icon={ChartColumn}
        hue={hueFor("insights")}
        href={{ pathname: "/ledgers/[ledgerId]/insights", params: { ledgerId, month } }}
        chevron
      />
    </ListSection>
  );
}

/** The whole month: free plan, monthly payments and reserve rates together. */
function Totals({ budget }: { budget: Budget }) {
  const t = budgetTotals(budget);
  const { colorScheme } = useColorScheme();
  const theme = THEME[colorScheme === "light" ? "light" : "dark"];
  const fraction = t.plannedExpenses > 0 ? t.spent / t.plannedExpenses : 0;
  const over = t.spent > t.plannedExpenses && t.plannedExpenses > 0;
  return (
    <Card>
      <CardContent className="flex-row items-center gap-4">
        <CircularProgress fraction={fraction} color={over ? theme.destructive : theme.primary} trackColor={theme.muted} size={72}>
          <Text className="text-xs font-semibold">{t.plannedExpenses > 0 ? `${Math.round(fraction * 100)}%` : "–"}</Text>
        </CircularProgress>
        <View className="flex-1 gap-1">
          <Text className="font-medium">
            {t.plannedExpenses > 0
              ? `Spent ${formatCents(t.spent)} of ${formatCents(t.plannedExpenses)} planned`
              : `Spent ${formatCents(t.spent)}, nothing planned`}
          </Text>
          {t.hasIncome && (
            <Text variant="muted">
              {t.plannedIncome > 0
                ? `Received ${formatCents(t.received)} of ${formatCents(t.plannedIncome)} planned`
                : `Received ${formatCents(t.received)}`}
            </Text>
          )}
        </View>
      </CardContent>
    </Card>
  );
}

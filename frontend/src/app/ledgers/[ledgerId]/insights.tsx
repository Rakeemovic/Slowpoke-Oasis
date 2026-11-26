import { useLocalSearchParams } from "expo-router";
import { ChartPie } from "lucide-react-native";
import { useCallback, useState } from "react";
import { View } from "react-native";

import { ledgers } from "@/api/endpoints";
import type { PurposeReport } from "@/api/types";
import { Amount } from "@/components/amount";
import { EmptyState } from "@/components/empty-state";
import { ErrorMessage } from "@/components/error-message";
import { MonthSwitcher } from "@/components/month-switcher";
import { PurposeRow } from "@/components/purpose-row";
import { Screen } from "@/components/screen";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { formatDate, monthName, shiftMonth, today } from "@/lib/dates";
import { isEmptyReport } from "@/lib/purposes";

/**
 * Where the money of one payday period went, by purpose: needs, wants,
 * savings and what has no purpose (rule L10). Each row is its own bar, filled
 * by its share of the income, and says how it moved against the period before.
 */
export default function InsightsScreen() {
  const { ledgerId } = useLocalSearchParams<{ ledgerId: string }>();
  // undefined means "the current period", which the server works out from the
  // app's date and the payday.
  const [month, setMonth] = useState<string>();
  const load = useCallback(() => ledgers.purposeReport(ledgerId, today(), month), [ledgerId, month]);
  const { data: report, error, loading, reload } = useLoader(load);

  return (
    <Screen>
      {report && (
        <MonthSwitcher
          month={report.month}
          isCurrent={report.month === report.current_month}
          disabled={loading}
          onShift={(delta) => setMonth(shiftMonth(report.month, delta))}
          onCurrent={() => setMonth(undefined)}
        />
      )}
      {loading && !report && <Skeleton className="h-64 w-full rounded-2xl" />}
      <ErrorMessage message={error} onRetry={reload} />
      {report && <ReportContent report={report} />}
    </Screen>
  );
}

function ReportContent({ report }: { report: PurposeReport }) {
  const period = `${formatDate(report.from)} – ${formatDate(report.to)}`;
  if (isEmptyReport(report)) {
    return (
      <>
        <Text variant="muted" className="text-center text-sm">
          {period}
        </Text>
        <EmptyState
          icon={ChartPie}
          title="Nothing to show for this period"
          text="Book expenses and income, and they show up here, sorted by what they are for."
        />
      </>
    );
  }
  return (
    <>
      <Text variant="muted" className="-mt-4 text-center text-sm">
        {period}
      </Text>
      <IncomeCard report={report} />
      <View className="gap-3">
        {report.lines.map((line) => (
          <PurposeRow key={line.purpose} line={line} comparedWith={monthName(report.previous_month)} />
        ))}
      </View>
      <Text variant="muted" className="text-sm">
        Savings count what you moved into your pots, minus what you took back, and expenses in categories marked as
        savings. Money for reserves does not count.
      </Text>
    </>
  );
}

function IncomeCard({ report }: { report: PurposeReport }) {
  return (
    <Card>
      <CardContent className="flex-row items-center justify-between gap-3">
        <View className="gap-0.5">
          <Text variant="muted" className="text-sm">
            Income this period
          </Text>
          <Amount cents={report.income_cents} size="lg" />
        </View>
        <Text variant="muted" className="max-w-32 text-right text-xs">
          {report.income_cents > 0 ? "Shares below are of this income" : "No income booked, so there are no shares"}
        </Text>
      </CardContent>
    </Card>
  );
}

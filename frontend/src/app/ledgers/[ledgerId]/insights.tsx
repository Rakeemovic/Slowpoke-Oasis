import { useLocalSearchParams } from "expo-router";
import { ArrowDown, ArrowUp, ChartPie, Equal, Zap, type LucideIcon } from "lucide-react-native";
import { useCallback, useState } from "react";
import { View } from "react-native";

import { ledgers } from "@/api/endpoints";
import type { PurposeLine, PurposeReport } from "@/api/types";
import { Amount } from "@/components/amount";
import { EmptyState } from "@/components/empty-state";
import { ErrorMessage } from "@/components/error-message";
import { IconTile } from "@/components/icon-tile";
import { MonthSwitcher } from "@/components/month-switcher";
import { ProgressRow } from "@/components/progress-row";
import { Screen } from "@/components/screen";
import { Card, CardContent } from "@/components/ui/card";
import { Icon } from "@/components/ui/icon";
import { Skeleton } from "@/components/ui/skeleton";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { formatDate, shiftMonth, today } from "@/lib/dates";
import {
  isEmptyReport,
  purposeLook,
  shareProgress,
  shareText,
  spontaneousText,
  transferText,
  trendOf,
  type Trend,
} from "@/lib/purposes";

const trendIcon: Record<Trend["direction"], LucideIcon> = { up: ArrowUp, down: ArrowDown, same: Equal };

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
          <PurposeRow key={line.purpose} line={line} previousMonth={report.previous_month} />
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

/** One purpose: its amount, its share as the row's own bar, and what changed. */
function PurposeRow({ line, previousMonth }: { line: PurposeLine; previousMonth: string }) {
  const look = purposeLook[line.purpose];
  const share = shareText(line);
  const trend = trendOf(line, previousMonth);
  const spontaneous = spontaneousText(line);
  const transfer = transferText(line);
  return (
    <Card className="overflow-hidden py-0">
      <ProgressRow progress={shareProgress(line)} label={look.name} fillClassName={look.fill} className="gap-2 p-4">
        <View className="flex-row items-center gap-3">
          <IconTile icon={look.icon} hue={look.hue} />
          <View className="flex-1 gap-0.5">
            <Text className="font-medium">{look.name}</Text>
            {share && <Text variant="muted">{share}</Text>}
          </View>
          <Amount cents={line.cents} size="lg" />
        </View>
        {trend && <DetailLine icon={trendIcon[trend.direction]} text={trend.text} />}
        {spontaneous && <DetailLine icon={Zap} text={spontaneous} />}
        {transfer && <Text variant="muted" className="text-sm">{transfer}</Text>}
      </ProgressRow>
    </Card>
  );
}

function DetailLine({ icon, text }: { icon: LucideIcon; text: string }) {
  return (
    <View className="flex-row items-center gap-1.5">
      <Icon as={icon} size={14} className="text-muted-foreground" />
      <Text variant="muted" className="text-sm">
        {text}
      </Text>
    </View>
  );
}

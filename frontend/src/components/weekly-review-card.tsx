import { NotebookPen } from "lucide-react-native";
import { useCallback } from "react";

import { ledgers } from "@/api/endpoints";
import { ErrorMessage } from "@/components/error-message";
import { ListRow, ListSection } from "@/components/list";
import { useLoader } from "@/hooks/use-loader";
import { formatDate, today } from "@/lib/dates";
import { MAIN_HUE } from "@/lib/hues";

/** Points to the weekly review while one is open (rule W1); otherwise it shows nothing. */
export function WeeklyReviewCard({ ledgerId }: { ledgerId: string }) {
  const { data, error, reload } = useLoader(useCallback(() => ledgers.weeklyReview(ledgerId, today()), [ledgerId]));

  if (!data?.due) {
    return <ErrorMessage message={error} onRetry={reload} />;
  }
  return (
    <ListSection>
      <ListRow
        icon={NotebookPen}
        hue={MAIN_HUE}
        title="Weekly review"
        subtitle={`Look back at ${formatDate(data.covered_from)} – ${formatDate(data.covered_until)}`}
        href={{ pathname: "/ledgers/[ledgerId]/review", params: { ledgerId } }}
      />
    </ListSection>
  );
}

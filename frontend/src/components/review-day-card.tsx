import { useCallback, useState } from "react";
import { toast } from "sonner-native";

import { auth } from "@/api/endpoints";
import { ChoicePicker } from "@/components/choice-picker";
import { ErrorMessage } from "@/components/error-message";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { useLoader } from "@/hooks/use-loader";
import { errorMessage } from "@/lib/errors";

/** In the order of the API's weekday numbers, which start with 1 = Monday. */
const WEEKDAYS = ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"];

/** Chooses the day on which the weekly review of every ledger opens (rule W1). Saved right away. */
export function ReviewDayCard() {
  const { data: me, error: loadError, reload } = useLoader(useCallback(() => auth.me(), []));
  const [error, setError] = useState<string>();

  async function choose(weekday: number) {
    setError(undefined);
    try {
      await auth.setReviewWeekday(weekday);
      await reload();
      toast.success("Review day saved");
    } catch (e) {
      setError(errorMessage(e));
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Weekly review</CardTitle>
        <CardDescription>The day your review opens, in all your ledgers.</CardDescription>
      </CardHeader>
      <CardContent className="gap-4">
        <ChoicePicker
          choices={WEEKDAYS.map((label, index) => ({ id: String(index + 1), label }))}
          selected={me && String(me.review_weekday)}
          onSelect={(id) => choose(Number(id))}
          emptyText=""
        />
        <ErrorMessage message={error ?? loadError} />
      </CardContent>
    </Card>
  );
}

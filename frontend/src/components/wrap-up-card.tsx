import { Link } from "expo-router";
import { useCallback, useState } from "react";
import { View } from "react-native";

import { ErrorMessage } from "@/components/error-message";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { monthName, today } from "@/lib/dates";
import { finishedMonth } from "@/lib/wrap-up";
import { readDismissedMonth, storeDismissedMonth } from "@/lib/wrap-up-storage";

/**
 * Points out the period that has just ended during the first days of the new
 * one: look back at it, then adjust the new budget. Otherwise it shows nothing.
 */
export function WrapUpCard({ ledgerId }: { ledgerId: string }) {
  const { data: month, error, reload } = useLoader(useCallback(() => finishedMonth(ledgerId, today()), [ledgerId]));
  const [dismissed, setDismissed] = useState(() => readDismissedMonth(ledgerId));

  if (!month || month === dismissed) {
    return <ErrorMessage message={error} onRetry={reload} />;
  }

  function dismiss(finished: string) {
    storeDismissedMonth(ledgerId, finished);
    setDismissed(finished);
  }

  return (
    <Card>
      <CardContent className="gap-3">
        <View className="gap-0.5">
          <Text className="font-medium">{monthName(month)} is over</Text>
          <Text variant="muted" className="text-sm">
            See where the money went, then check the budget for the new period.
          </Text>
        </View>
        <View className="flex-row flex-wrap gap-2">
          <Link href={{ pathname: "/ledgers/[ledgerId]/insights", params: { ledgerId, month } }} asChild>
            <Button size="sm">
              <Text>View insights</Text>
            </Button>
          </Link>
          <Link href={{ pathname: "/ledgers/[ledgerId]/budget", params: { ledgerId } }} asChild>
            <Button size="sm" variant="outline">
              <Text>Open budget</Text>
            </Button>
          </Link>
          <Button size="sm" variant="ghost" onPress={() => dismiss(month)}>
            <Text>Dismiss</Text>
          </Button>
        </View>
      </CardContent>
    </Card>
  );
}

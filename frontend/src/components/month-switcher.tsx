import { ChevronLeft, ChevronRight } from "lucide-react-native";
import { View } from "react-native";

import { Button } from "@/components/ui/button";
import { Icon } from "@/components/ui/icon";
import { Text } from "@/components/ui/text";
import { formatMonth } from "@/lib/dates";

/**
 * Steps through the payday periods of a ledger, named by their month, and
 * offers the way back once another period than the current one is shown.
 * Budget and insights share it.
 */
export function MonthSwitcher({
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
    <View className="gap-1">
      <View className="flex-row items-center gap-2">
        <Button variant="ghost" size="icon" onPress={() => onShift(-1)} disabled={disabled} aria-label="Previous month">
          <Icon as={ChevronLeft} size={22} />
        </Button>
        <Text variant="h3" className="flex-1 text-center">
          {formatMonth(month)}
        </Text>
        <Button variant="ghost" size="icon" onPress={() => onShift(1)} disabled={disabled} aria-label="Next month">
          <Icon as={ChevronRight} size={22} />
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

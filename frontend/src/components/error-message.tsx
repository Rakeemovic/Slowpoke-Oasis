import { CircleAlert } from "lucide-react-native";
import { View } from "react-native";

import { Button } from "@/components/ui/button";
import { Icon } from "@/components/ui/icon";
import { Text } from "@/components/ui/text";

/** Shows an error, optionally with a button to try again. Renders nothing without a message. */
export function ErrorMessage({ message, onRetry }: { message?: string; onRetry?: () => void }) {
  if (!message) {
    return null;
  }
  return (
    <View className="gap-3 rounded-xl bg-destructive/10 p-3" role="alert">
      <View className="flex-row gap-2">
        <Icon as={CircleAlert} size={18} className="mt-0.5 text-destructive" />
        <Text className="flex-1 text-destructive">{message}</Text>
      </View>
      {onRetry && (
        <Button variant="outline" size="sm" className="self-start" onPress={onRetry}>
          <Text>Try again</Text>
        </Button>
      )}
    </View>
  );
}

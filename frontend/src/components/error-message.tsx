import { View } from "react-native";

import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";

/** Shows an error, optionally with a button to try again. Renders nothing without a message. */
export function ErrorMessage({ message, onRetry }: { message?: string; onRetry?: () => void }) {
  if (!message) {
    return null;
  }
  return (
    <View className="gap-2" role="alert">
      <Text className="text-destructive">{message}</Text>
      {onRetry && (
        <Button variant="outline" size="sm" className="self-start" onPress={onRetry}>
          <Text>Try again</Text>
        </Button>
      )}
    </View>
  );
}

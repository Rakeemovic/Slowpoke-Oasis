import { View } from "react-native";

import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";

/**
 * Asks "are you sure?" inline before an action that cannot be undone.
 * Inline instead of Alert.alert, because Alert shows no buttons on the web.
 */
export function ConfirmPanel({
  message,
  confirmLabel,
  busy,
  onConfirm,
  onCancel,
}: {
  message: string;
  confirmLabel: string;
  busy: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  return (
    <View className="gap-3 rounded-md border border-destructive/50 p-3" role="alertdialog">
      <Text>{message}</Text>
      <View className="flex-row gap-2">
        <Button variant="destructive" size="sm" onPress={onConfirm} disabled={busy}>
          <Text>{confirmLabel}</Text>
        </Button>
        <Button variant="outline" size="sm" onPress={onCancel} disabled={busy}>
          <Text>Cancel</Text>
        </Button>
      </View>
    </View>
  );
}

import { View } from "react-native";

import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";
import { cn } from "@/lib/utils";

/**
 * Asks "are you sure?" inline before an action that cannot be undone, or
 * (tone "default") offers a follow-up step that is not dangerous.
 * Inline instead of Alert.alert, because Alert shows no buttons on the web.
 */
export function ConfirmPanel({
  message,
  confirmLabel,
  busy,
  onConfirm,
  onCancel,
  tone = "destructive",
}: {
  message: string;
  confirmLabel: string;
  busy: boolean;
  onConfirm: () => void;
  onCancel: () => void;
  tone?: "destructive" | "default";
}) {
  return (
    <View
      className={cn(
        "gap-3 rounded-2xl border p-4",
        tone === "destructive" ? "border-destructive/40 bg-destructive/5" : "border-border bg-card",
      )}
      role="alertdialog">
      <Text>{message}</Text>
      <View className="flex-row gap-2">
        <Button variant={tone} size="sm" onPress={onConfirm} disabled={busy}>
          <Text>{confirmLabel}</Text>
        </Button>
        <Button variant="ghost" size="sm" onPress={onCancel} disabled={busy}>
          <Text>Cancel</Text>
        </Button>
      </View>
    </View>
  );
}

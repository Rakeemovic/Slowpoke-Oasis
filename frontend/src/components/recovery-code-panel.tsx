import { View } from "react-native";

import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";

/**
 * Shows a recovery code once and asks the user to keep it. The server stores
 * only a hash, so this is the only moment the code can be seen. It is the
 * only way back into the account if the password is forgotten.
 */
export function RecoveryCodePanel({
  code,
  continueLabel,
  busy,
  onContinue,
}: {
  code: string;
  continueLabel: string;
  busy: boolean;
  onContinue: () => void;
}) {
  return (
    <View className="gap-4">
      <Text variant="h3">Your recovery code</Text>
      <Text>
        If you forget your password, this code is the only way back into your account. Write it down or keep it in a
        password manager. It is shown only now and cannot be looked up later.
      </Text>
      <View className="rounded-md border border-border bg-muted p-4" aria-label="Recovery code">
        <Text selectable className="text-center font-mono text-lg">
          {code}
        </Text>
      </View>
      <Text variant="muted">
        The code works once. After you use it to reset your password, you get a new one.
      </Text>
      <Button onPress={onContinue} disabled={busy}>
        <Text>{continueLabel}</Text>
      </Button>
    </View>
  );
}

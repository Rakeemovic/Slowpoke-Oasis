import * as Clipboard from "expo-clipboard";
import { useState } from "react";
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
  const [copyState, setCopyState] = useState<"idle" | "copied" | "failed">("idle");

  async function copyCode() {
    // isSensitive keeps the code out of Android's clipboard preview.
    // On web the browser may refuse; then the user can still select the text.
    const saved = await Clipboard.setStringAsync(code, { android: { isSensitive: true } }).catch(() => false);
    setCopyState(saved ? "copied" : "failed");
  }

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
      <Button variant="outline" onPress={copyCode}>
        <Text>{copyState === "copied" ? "Copied" : "Copy code"}</Text>
      </Button>
      {copyState === "failed" ? (
        <Text variant="muted">Copying did not work here. Please select the code and copy it by hand.</Text>
      ) : null}
      <Text variant="muted">
        The code works once. After you use it to reset your password, you get a new one.
      </Text>
      <Button onPress={onContinue} disabled={busy}>
        <Text>{continueLabel}</Text>
      </Button>
    </View>
  );
}

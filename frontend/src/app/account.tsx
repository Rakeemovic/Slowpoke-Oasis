import { useState } from "react";

import { auth } from "@/api/endpoints";
import { ErrorMessage } from "@/components/error-message";
import { FormField } from "@/components/form-field";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";
import { errorMessage } from "@/lib/errors";

/** Account settings. For now only the password can be changed here. */
export default function AccountScreen() {
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [error, setError] = useState<string>();
  const [done, setDone] = useState(false);
  const [busy, setBusy] = useState(false);

  async function submit() {
    setBusy(true);
    setError(undefined);
    setDone(false);
    try {
      await auth.changePassword(currentPassword, newPassword);
      setCurrentPassword("");
      setNewPassword("");
      setDone(true);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Screen>
      <Text variant="h4">Change password</Text>
      <FormField
        label="Current password"
        value={currentPassword}
        onChangeText={setCurrentPassword}
        secureTextEntry
        autoComplete="current-password"
      />
      <FormField
        label="New password"
        value={newPassword}
        onChangeText={setNewPassword}
        secureTextEntry
        autoComplete="new-password"
        onSubmitEditing={submit}
      />
      <ErrorMessage message={error} />
      {done && <Text role="status">Password changed. Your other devices have been signed out.</Text>}
      <Button onPress={submit} disabled={busy || !currentPassword || !newPassword}>
        <Text>{busy ? "Saving…" : "Change password"}</Text>
      </Button>
      <Text variant="muted">Your recovery code stays the same.</Text>
    </Screen>
  );
}

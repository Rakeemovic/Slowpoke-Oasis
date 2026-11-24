import { useState } from "react";

import { auth } from "@/api/endpoints";
import { useAuth } from "@/auth/auth-context";
import { AuthHeader } from "@/components/auth-header";
import { ErrorMessage } from "@/components/error-message";
import { FormField } from "@/components/form-field";
import { RecoveryCodePanel } from "@/components/recovery-code-panel";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";
import { errorMessage } from "@/lib/errors";

/** Sets a new password with the recovery code from registration. */
export default function RecoverScreen() {
  const { signIn } = useAuth();
  const [username, setUsername] = useState("");
  const [code, setCode] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [newCode, setNewCode] = useState<string>();
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  const complete = username.trim() && code.trim() && newPassword;

  async function submit() {
    setBusy(true);
    setError(undefined);
    try {
      const result = await auth.recover({
        username: username.trim(),
        recovery_code: code,
        new_password: newPassword,
      });
      // The old code is used up. The new one must be shown before signing in,
      // because signing in leaves this screen.
      setNewCode(result.recovery_code);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }

  async function continueToApp() {
    setBusy(true);
    setError(undefined);
    try {
      await signIn(username.trim(), newPassword);
    } catch (e) {
      setError(errorMessage(e));
      setBusy(false);
    }
  }

  if (newCode) {
    return (
      <Screen>
        <Text>Your password has been changed and all devices have been signed out.</Text>
        <RecoveryCodePanel
          code={newCode}
          continueLabel={busy ? "Signing in…" : "I have saved it, sign in"}
          busy={busy}
          onContinue={continueToApp}
        />
        <ErrorMessage message={error} />
      </Screen>
    );
  }

  return (
    <Screen>
      <AuthHeader
        title="Reset your password"
        text="Enter the six-word recovery code you received when you created your account."
      />
      <FormField label="Username" value={username} onChangeText={setUsername} autoCapitalize="none" autoComplete="username" />
      <FormField
        label="Recovery code"
        value={code}
        onChangeText={setCode}
        autoCapitalize="none"
        autoCorrect={false}
        autoComplete="off"
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
      <Button onPress={submit} disabled={busy || !complete}>
        <Text>{busy ? "Resetting…" : "Reset password"}</Text>
      </Button>
      <Text variant="muted">Lost the code as well? The person who runs this server can reset your password for you.</Text>
    </Screen>
  );
}

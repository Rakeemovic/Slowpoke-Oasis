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

export default function RegisterScreen() {
  const { signIn } = useAuth();
  const [username, setUsername] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [recoveryCode, setRecoveryCode] = useState<string>();
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  const complete = username.trim() && displayName.trim() && password;

  async function submit() {
    setBusy(true);
    setError(undefined);
    try {
      const created = await auth.register({
        username: username.trim(),
        display_name: displayName.trim(),
        password,
      });
      // Signing in would leave this screen at once, so the code is shown first.
      setRecoveryCode(created.recovery_code);
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
      // Log in right away, so nobody has to type the same data twice.
      await signIn(username.trim(), password);
    } catch (e) {
      setError(errorMessage(e));
      setBusy(false);
    }
  }

  if (recoveryCode) {
    return (
      <Screen>
        <RecoveryCodePanel
          code={recoveryCode}
          continueLabel={busy ? "Signing in…" : "I have saved it, continue"}
          busy={busy}
          onContinue={continueToApp}
        />
        <ErrorMessage message={error} />
      </Screen>
    );
  }

  return (
    <Screen>
      <AuthHeader title="Create your account" text="No email needed: you get a recovery code instead." />
      <FormField label="Username" value={username} onChangeText={setUsername} autoCapitalize="none" autoComplete="username-new" />
      <FormField label="Display name" value={displayName} onChangeText={setDisplayName} autoComplete="name" />
      <FormField
        label="Password"
        value={password}
        onChangeText={setPassword}
        secureTextEntry
        autoComplete="new-password"
        onSubmitEditing={submit}
      />
      <ErrorMessage message={error} />
      <Button onPress={submit} disabled={busy || !complete}>
        <Text>{busy ? "Creating account…" : "Create account"}</Text>
      </Button>
    </Screen>
  );
}

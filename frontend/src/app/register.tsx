import { useState } from "react";

import { auth } from "@/api/endpoints";
import { useAuth } from "@/auth/auth-context";
import { ErrorMessage } from "@/components/error-message";
import { FormField } from "@/components/form-field";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";
import { errorMessage } from "@/lib/errors";

export default function RegisterScreen() {
  const { signIn } = useAuth();
  const [username, setUsername] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  const complete = username.trim() && displayName.trim() && email.trim() && password;

  async function submit() {
    setBusy(true);
    setError(undefined);
    try {
      await auth.register({
        username: username.trim(),
        display_name: displayName.trim(),
        email: email.trim(),
        password,
      });
      // Log in right away, so nobody has to type the same data twice.
      await signIn(username.trim(), password);
    } catch (e) {
      setError(errorMessage(e));
      setBusy(false);
    }
  }

  return (
    <Screen>
      <FormField label="Username" value={username} onChangeText={setUsername} autoCapitalize="none" autoComplete="username-new" />
      <FormField label="Display name" value={displayName} onChangeText={setDisplayName} autoComplete="name" />
      <FormField
        label="Email"
        value={email}
        onChangeText={setEmail}
        autoCapitalize="none"
        keyboardType="email-address"
        autoComplete="email"
      />
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

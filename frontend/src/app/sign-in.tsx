import { Link } from "expo-router";
import { useState } from "react";

import { useAuth } from "@/auth/auth-context";
import { AuthHeader } from "@/components/auth-header";
import { ErrorMessage } from "@/components/error-message";
import { FormField } from "@/components/form-field";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";
import { errorMessage } from "@/lib/errors";

export default function SignInScreen() {
  const { signIn } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  async function submit() {
    setBusy(true);
    setError(undefined);
    try {
      // On success the navigator switches to the households screen by itself.
      await signIn(username.trim(), password);
    } catch (e) {
      setError(errorMessage(e));
      setBusy(false);
    }
  }

  return (
    <Screen>
      <AuthHeader title="Welcome back" text="Your money, your household, at a calm pace." />
      <FormField label="Username" value={username} onChangeText={setUsername} autoCapitalize="none" autoComplete="username" />
      <FormField
        label="Password"
        value={password}
        onChangeText={setPassword}
        secureTextEntry
        autoComplete="current-password"
        onSubmitEditing={submit}
      />
      <ErrorMessage message={error} />
      <Button onPress={submit} disabled={busy || !username.trim() || !password}>
        <Text>{busy ? "Signing in…" : "Sign in"}</Text>
      </Button>
      <Link href="/recover" asChild>
        <Button variant="link">
          <Text>Forgot your password?</Text>
        </Button>
      </Link>
      <Link href="/register" asChild>
        <Button variant="link">
          <Text>No account yet? Create one</Text>
        </Button>
      </Link>
    </Screen>
  );
}

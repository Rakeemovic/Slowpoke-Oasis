import { Link, Stack } from "expo-router";
import { useState } from "react";
import { ActivityIndicator, Pressable, View } from "react-native";

import { households } from "@/api/endpoints";
import { useAuth } from "@/auth/auth-context";
import { ErrorMessage } from "@/components/error-message";
import { FormField } from "@/components/form-field";
import { LedgerBalance } from "@/components/ledger-balance";
import { Screen } from "@/components/screen";
import { ThemeToggle } from "@/components/theme-toggle";
import { Button } from "@/components/ui/button";
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { errorMessage } from "@/lib/errors";
import { loadMyLedgers, type LedgerSummary } from "@/lib/ledgers";

export default function OverviewScreen() {
  const { signOut } = useAuth();
  // loadMyLedgers is defined outside the component, so it is stable without useCallback.
  const { data, error, loading, reload } = useLoader(loadMyLedgers);
  const personal = data?.filter((l) => l.kind === "personal") ?? [];
  const shared = data?.filter((l) => l.kind === "household") ?? [];

  return (
    <Screen>
      <Stack.Screen
        options={{
          headerRight: () => (
            <View className="flex-row">
              <ThemeToggle />
              <Button variant="ghost" size="sm" onPress={signOut}>
                <Text>Sign out</Text>
              </Button>
            </View>
          ),
        }}
      />
      {loading && !data && <ActivityIndicator />}
      <ErrorMessage message={error} onRetry={reload} />
      {personal.map((l) => (
        <LedgerCard key={l.id} ledger={l} />
      ))}
      {data && (
        <Text variant="h4" className="mt-4">
          Households
        </Text>
      )}
      {data && shared.length === 0 && <Text variant="muted">You are not in a household yet. Create one below.</Text>}
      {shared.map((l) => (
        <LedgerCard key={l.id} ledger={l} />
      ))}
      <CreateHousehold onCreated={reload} />
    </Screen>
  );
}

/** A card that opens the transactions of a ledger and shows its balance. */
function LedgerCard({ ledger }: { ledger: LedgerSummary }) {
  const params = {
    ledgerId: ledger.id,
    name: ledger.name,
    kind: ledger.kind,
    // Route params are strings, so a missing household ID is left out entirely.
    ...(ledger.householdId && { householdId: ledger.householdId }),
  };
  return (
    <Link href={{ pathname: "/ledgers/[ledgerId]", params }} asChild>
      <Pressable accessibilityRole="link">
        <Card>
          <CardHeader className="flex-row items-center justify-between gap-4">
            <View className="flex-1 gap-1.5">
              <CardTitle>{ledger.name}</CardTitle>
              <CardDescription>{ledger.description}</CardDescription>
            </View>
            <LedgerBalance ledgerId={ledger.id} />
          </CardHeader>
        </Card>
      </Pressable>
    </Link>
  );
}

function CreateHousehold({ onCreated }: { onCreated: () => void }) {
  const [name, setName] = useState("");
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  async function submit() {
    setBusy(true);
    setError(undefined);
    try {
      await households.create(name.trim());
      setName("");
      onCreated();
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <View className="mt-4 gap-3">
      <FormField label="New household" value={name} onChangeText={setName} placeholder="e.g. Shared flat" onSubmitEditing={submit} />
      <ErrorMessage message={error} />
      <Button variant="secondary" onPress={submit} disabled={busy || !name.trim()}>
        <Text>Create household</Text>
      </Button>
    </View>
  );
}

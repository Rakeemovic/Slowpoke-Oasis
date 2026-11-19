import { Link, Stack } from "expo-router";
import { useState, type ReactNode } from "react";
import { ActivityIndicator, Pressable, View } from "react-native";

import { auth, households } from "@/api/endpoints";
import type { Household, Me } from "@/api/types";
import { useAuth } from "@/auth/auth-context";
import { ErrorMessage } from "@/components/error-message";
import { FormField } from "@/components/form-field";
import { Screen } from "@/components/screen";
import { ThemeToggle } from "@/components/theme-toggle";
import { Button } from "@/components/ui/button";
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { errorMessage } from "@/lib/errors";

/** Loads the user and the households together. Defined outside the component, so it is stable without useCallback. */
async function loadOverview() {
  const [me, list] = await Promise.all([auth.me(), households.list()]);
  return { me, households: list };
}

export default function OverviewScreen() {
  const { signOut } = useAuth();
  const { data, error, loading, reload } = useLoader(loadOverview);

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
      {data && <PersonalLedgerCard me={data.me} />}
      {data && (
        <Text variant="h4" className="mt-4">
          Households
        </Text>
      )}
      {data?.households.length === 0 && <Text variant="muted">You are not in a household yet. Create one below.</Text>}
      {data?.households.map((household) => <HouseholdCard key={household.id} household={household} />)}
      <CreateHousehold onCreated={reload} />
    </Screen>
  );
}

function PersonalLedgerCard({ me }: { me: Me }) {
  if (!me.personal_ledger_id) {
    // Only accounts created before every account got a personal ledger.
    return <Text variant="muted">Your account has no personal ledger.</Text>;
  }
  return (
    <LedgerLink ledgerId={me.personal_ledger_id} name="Personal">
      <CardTitle>Personal</CardTitle>
      <CardDescription>Only you can see this ledger</CardDescription>
    </LedgerLink>
  );
}

function HouseholdCard({ household }: { household: Household }) {
  return (
    <LedgerLink ledgerId={household.ledger_id} name={household.name}>
      <CardTitle>{household.name}</CardTitle>
      <CardDescription>{household.role === "owner" ? "Owner" : "Member"}</CardDescription>
    </LedgerLink>
  );
}

/** A card that opens the transactions of a ledger. */
function LedgerLink({ ledgerId, name, children }: { ledgerId: string; name: string; children: ReactNode }) {
  return (
    <Link href={{ pathname: "/ledgers/[ledgerId]", params: { ledgerId, name } }} asChild>
      <Pressable accessibilityRole="link">
        <Card>
          <CardHeader>{children}</CardHeader>
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

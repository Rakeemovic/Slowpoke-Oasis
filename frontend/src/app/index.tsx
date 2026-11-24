import { Link, Stack } from "expo-router";
import { CircleUser, LogOut, Users, Wallet } from "lucide-react-native";
import { useState } from "react";
import { toast } from "sonner-native";

import { households } from "@/api/endpoints";
import { useAuth } from "@/auth/auth-context";
import { EmptyState } from "@/components/empty-state";
import { ErrorMessage } from "@/components/error-message";
import { FormField } from "@/components/form-field";
import { HeaderActions, HeaderIconButton } from "@/components/header-actions";
import { LedgerBalance } from "@/components/ledger-balance";
import { ListRow, ListSection } from "@/components/list";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Icon } from "@/components/ui/icon";
import { Skeleton } from "@/components/ui/skeleton";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { errorMessage } from "@/lib/errors";
import { hueFor, MAIN_HUE } from "@/lib/hues";
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
            <HeaderActions>
              <Link href="/account" asChild>
                <Button variant="ghost" size="icon" aria-label="Account">
                  <Icon as={CircleUser} size={20} />
                </Button>
              </Link>
              <HeaderIconButton icon={LogOut} label="Sign out" onPress={signOut} />
            </HeaderActions>
          ),
        }}
      />
      {loading && !data && <Skeleton className="h-24 w-full rounded-2xl" />}
      <ErrorMessage message={error} onRetry={reload} />
      {personal.length > 0 && (
        <ListSection title="Your money">
          {personal.map((l) => (
            <LedgerRow key={l.id} ledger={l} />
          ))}
        </ListSection>
      )}
      {data && (
        <ListSection title="Households">
          {shared.map((l) => (
            <LedgerRow key={l.id} ledger={l} />
          ))}
        </ListSection>
      )}
      {data && shared.length === 0 && (
        <EmptyState
          icon={Users}
          title="No household yet"
          text="Share a ledger with the people you live with. Create one below."
        />
      )}
      <CreateHousehold onCreated={reload} />
    </Screen>
  );
}

/** A row that opens the tabs of a ledger and shows its balance. */
function LedgerRow({ ledger }: { ledger: LedgerSummary }) {
  const params = {
    ledgerId: ledger.id,
    name: ledger.name,
    kind: ledger.kind,
    // Route params are strings, so a missing household ID is left out entirely.
    ...(ledger.householdId && { householdId: ledger.householdId }),
  };
  return (
    <ListRow
      href={{ pathname: "/ledgers/[ledgerId]", params }}
      icon={ledger.kind === "personal" ? Wallet : Users}
      hue={ledger.kind === "personal" ? MAIN_HUE : hueFor(ledger.id)}
      title={<Text className="font-heading text-base">{ledger.name}</Text>}
      subtitle={ledger.description}
      trailing={<LedgerBalance ledgerId={ledger.id} />}
      className="min-h-20"
    />
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
      toast.success(`Household “${name.trim()}” created`);
      setName("");
      onCreated();
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card>
      <CardContent className="gap-3">
        <FormField
          label="New household"
          value={name}
          onChangeText={setName}
          placeholder="e.g. Shared flat"
          onSubmitEditing={submit}
        />
        <ErrorMessage message={error} />
        <Button variant="secondary" onPress={submit} disabled={busy || !name.trim()}>
          <Text>Create household</Text>
        </Button>
      </CardContent>
    </Card>
  );
}

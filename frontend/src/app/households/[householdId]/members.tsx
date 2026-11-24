import { router, Stack, useLocalSearchParams } from "expo-router";
import { useCallback, useState } from "react";
import { View } from "react-native";

import { auth, households } from "@/api/endpoints";
import type { Member } from "@/api/types";
import { ConfirmPanel } from "@/components/confirm-panel";
import { ErrorMessage } from "@/components/error-message";
import { ListSection } from "@/components/list";
import { FormField } from "@/components/form-field";
import { HeaderActions } from "@/components/header-actions";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { errorMessage } from "@/lib/errors";
import { hueFor } from "@/lib/hues";
import { cn } from "@/lib/utils";

/** An action that cannot be undone, waiting for the user to confirm it. */
type PendingAction = { kind: "remove"; member: Member } | { kind: "owner"; member: Member } | { kind: "leave" };

export default function MembersScreen() {
  const { householdId } = useLocalSearchParams<{ householdId: string }>();
  // The user's own ID tells which row is "you" and whether they are the owner.
  const load = useCallback(() => Promise.all([households.members(householdId), auth.me()]), [householdId]);
  const { data, error, loading, reload } = useLoader(load);
  const [members, me] = data ?? [];
  const isOwner = members?.some((m) => m.user_id === me?.user_id && m.role === "owner") ?? false;

  const [pending, setPending] = useState<PendingAction>();
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState<string>();

  function ask(action?: PendingAction) {
    setPending(action);
    setActionError(undefined);
  }

  async function confirm() {
    if (!pending || !me) {
      return;
    }
    setBusy(true);
    setActionError(undefined);
    try {
      await runAction(householdId, pending, me.user_id);
      if (pending.kind === "leave") {
        // The household is gone for this user, so its screens are too.
        router.dismissTo("/");
        return;
      }
      setPending(undefined);
      await reload();
    } catch (e) {
      setActionError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }

  const confirmPanel = (action: PendingAction) => (
    <ConfirmPanel
      {...describeAction(action, members?.length ?? 0, isOwner)}
      busy={busy}
      onConfirm={confirm}
      onCancel={() => ask(undefined)}
    />
  );

  return (
    <Screen>
      <Stack.Screen options={{ headerRight: () => <HeaderActions /> }} />
      {loading && !data && <Skeleton className="h-32 w-full rounded-2xl" />}
      <ErrorMessage message={error} onRetry={reload} />
      {members && me && (
        <ListSection title={`${members.length} ${members.length === 1 ? "member" : "members"}`}>
          {members.map((member) => {
            const isMe = member.user_id === me.user_id;
            const target = pending && pending.kind !== "leave" && pending.member.user_id === member.user_id;
            return (
              <View key={member.user_id} className="gap-3 px-4 py-3">
                <View className="flex-row items-center gap-3">
                  <Avatar name={member.username} id={member.user_id} />
                  <View className="flex-1 gap-0.5">
                    <Text className="font-medium">
                      {member.username}
                      {isMe && " (you)"}
                    </Text>
                    <Text variant="muted">{member.role === "owner" ? "Owner" : "Member"}</Text>
                  </View>
                  {/* Only the owner manages others; everyone else just sees the list. */}
                  {isOwner && !isMe && (
                    <View className="flex-row">
                      <Button variant="ghost" size="sm" disabled={busy} onPress={() => ask({ kind: "owner", member })}>
                        <Text>Make owner</Text>
                      </Button>
                      <Button variant="ghost" size="sm" disabled={busy} onPress={() => ask({ kind: "remove", member })}>
                        <Text className="text-destructive">Remove</Text>
                      </Button>
                    </View>
                  )}
                </View>
                {target && confirmPanel(pending)}
              </View>
            );
          })}
        </ListSection>
      )}
      <ErrorMessage message={actionError} />
      {isOwner && <AddMember householdId={householdId} onAdded={reload} />}
      {members && me && (
        <View className="gap-3">
          <Button variant="outline" disabled={busy} onPress={() => ask({ kind: "leave" })}>
            <Text className="text-destructive">Leave household</Text>
          </Button>
          {pending?.kind === "leave" && confirmPanel(pending)}
        </View>
      )}
    </Screen>
  );
}

/** A round tile with the member's initial, coloured per member, so people are easy to tell apart. */
function Avatar({ name, id }: { name: string; id: string }) {
  const hue = hueFor(id);
  return (
    <View aria-hidden className={cn("h-10 w-10 items-center justify-center rounded-full", hue.tile)}>
      <Text className={cn("font-heading text-base", hue.icon)}>{name.slice(0, 1).toUpperCase()}</Text>
    </View>
  );
}

function runAction(householdId: string, action: PendingAction, myUserId: string): Promise<void> {
  switch (action.kind) {
    case "remove":
      return households.removeMember(householdId, action.member.user_id);
    case "owner":
      return households.transferOwnership(householdId, action.member.user_id);
    case "leave":
      // Leaving is removing oneself; the API allows that for every member.
      return households.removeMember(householdId, myUserId);
  }
}

/** What happens after confirming, so nobody is surprised by the consequences. */
function describeAction(action: PendingAction, memberCount: number, isOwner: boolean): { message: string; confirmLabel: string } {
  switch (action.kind) {
    case "remove":
      return {
        message: `${action.member.username} will lose access to this household. Their transactions stay, shown as “Former member”.`,
        confirmLabel: "Remove",
      };
    case "owner":
      return {
        message: `${action.member.username} becomes the owner. You stay a member, but can no longer add or remove members.`,
        confirmLabel: "Make owner",
      };
    case "leave":
      // The backend deletes a household when its last member leaves.
      if (memberCount <= 1) {
        return {
          message: "You are the last member. Leaving deletes this household with all its transactions.",
          confirmLabel: "Leave and delete",
        };
      }
      return {
        message:
          "Your transactions stay, shown as “Former member”." +
          (isOwner ? " The member who joined first becomes the owner." : "") +
          " Only the owner can add you again.",
        confirmLabel: "Leave",
      };
  }
}

function AddMember({ householdId, onAdded }: { householdId: string; onAdded: () => void }) {
  const [username, setUsername] = useState("");
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  async function submit() {
    setBusy(true);
    setError(undefined);
    try {
      await households.addMember(householdId, username.trim());
      setUsername("");
      onAdded();
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <View className="gap-3">
      <FormField
        label="Add member"
        value={username}
        onChangeText={setUsername}
        placeholder="Username"
        autoCapitalize="none"
        autoCorrect={false}
        onSubmitEditing={submit}
      />
      <ErrorMessage message={error} />
      <Button variant="secondary" onPress={submit} disabled={busy || !username.trim()}>
        <Text>Add member</Text>
      </Button>
    </View>
  );
}

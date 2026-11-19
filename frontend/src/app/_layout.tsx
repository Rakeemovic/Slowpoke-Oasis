import "@/global.css";

import { PortalHost } from "@rn-primitives/portal";
import { Stack, ThemeProvider } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { useColorScheme } from "nativewind";
import { useEffect } from "react";
import { ActivityIndicator, View } from "react-native";

import { AuthProvider, useAuth } from "@/auth/auth-context";
import { ThemeToggle } from "@/components/theme-toggle";
import { applySavedScheme } from "@/lib/color-scheme";
import { NAV_THEME } from "@/lib/theme";

// Applied before the first render, so Android starts in the right scheme.
applySavedScheme();

export default function RootLayout() {
  // Applied again after mounting: on the web, NativeWind can only switch once
  // the stylesheet is loaded, which in development happens after this module.
  useEffect(applySavedScheme, []);

  return (
    <AuthProvider>
      <RootNavigator />
    </AuthProvider>
  );
}

function RootNavigator() {
  const { status } = useAuth();
  const { colorScheme } = useColorScheme();
  const scheme = colorScheme === "dark" ? "dark" : "light";

  if (status === "loading") {
    return (
      <View className="flex-1 items-center justify-center bg-background">
        <ActivityIndicator />
      </View>
    );
  }

  // Protected groups hide screens that do not fit the login state. When the
  // state changes, Expo Router moves the user to an allowed screen by itself.
  const signedIn = status === "signedIn";
  return (
    <ThemeProvider value={NAV_THEME[scheme]}>
      <StatusBar style={scheme === "dark" ? "light" : "dark"} />
      <Stack screenOptions={{ headerRight: () => <ThemeToggle /> }}>
        <Stack.Protected guard={!signedIn} redirectTo="/">
          <Stack.Screen name="sign-in" options={{ title: "Sign in" }} />
          <Stack.Screen name="register" options={{ title: "Create account" }} />
        </Stack.Protected>
        <Stack.Protected guard={signedIn} redirectTo="/sign-in">
          <Stack.Screen name="index" options={{ title: "Overview" }} />
          <Stack.Screen name="ledgers/[ledgerId]/index" options={{ title: "Transactions" }} />
          <Stack.Screen name="ledgers/[ledgerId]/new" options={{ title: "New transaction" }} />
          <Stack.Screen name="ledgers/[ledgerId]/transfer" options={{ title: "Transfer" }} />
          <Stack.Screen name="households/[householdId]/members" options={{ title: "Members" }} />
        </Stack.Protected>
      </Stack>
      {/* Overlays such as dialogs and dropdowns render here, above all screens. */}
      <PortalHost />
    </ThemeProvider>
  );
}

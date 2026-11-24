import "@/global.css";

import { Nunito_700Bold, Nunito_800ExtraBold, useFonts } from "@expo-google-fonts/nunito";
import { PortalHost } from "@rn-primitives/portal";
import { Stack, ThemeProvider } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { useColorScheme } from "nativewind";
import { useEffect } from "react";
import { ActivityIndicator, View } from "react-native";
import { GestureHandlerRootView } from "react-native-gesture-handler";
import { Toaster } from "sonner-native";

import { AuthProvider, useAuth } from "@/auth/auth-context";
import { ThemeToggle } from "@/components/theme-toggle";
import { useRefreshOnReturn } from "@/hooks/use-refresh-on-return";
import { applySavedScheme } from "@/lib/color-scheme";
import { NAV_THEME, THEME } from "@/lib/theme";

// Applied before the first render, so Android starts in the right scheme.
applySavedScheme();

export default function RootLayout() {
  // Applied again after mounting: on the web, NativeWind can only switch once
  // the stylesheet is loaded, which in development happens after this module.
  useEffect(applySavedScheme, []);

  // Without the fonts, headings fall back to the system font for a moment;
  // that is better than a blank screen, so the app does not wait for them.
  useFonts({ Nunito_700Bold, Nunito_800ExtraBold });

  return (
    <GestureHandlerRootView style={{ flex: 1 }}>
      <AuthProvider>
        <RootNavigator />
      </AuthProvider>
    </GestureHandlerRootView>
  );
}

function RootNavigator() {
  const { status } = useAuth();
  useRefreshOnReturn();
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
      <Stack
        screenOptions={{
          headerRight: () => <ThemeToggle />,
          // A flat header in the page colour: the content, not the chrome, stands out.
          headerShadowVisible: false,
          headerTitleStyle: { fontFamily: "Nunito_800ExtraBold" },
          headerBackButtonDisplayMode: "minimal",
        }}>
        <Stack.Protected guard={!signedIn} redirectTo="/">
          <Stack.Screen name="sign-in" options={{ title: "Sign in" }} />
          <Stack.Screen name="register" options={{ title: "Create account" }} />
          <Stack.Screen name="recover" options={{ title: "Reset password" }} />
        </Stack.Protected>
        <Stack.Protected guard={signedIn} redirectTo="/sign-in">
          <Stack.Screen name="index" options={{ title: "Overview" }} />
          <Stack.Screen name="account" options={{ title: "Account" }} />
          <Stack.Screen name="ledgers/[ledgerId]/(tabs)" options={{ title: "Ledger" }} />
          <Stack.Screen name="ledgers/[ledgerId]/new" options={{ title: "New transaction" }} />
          <Stack.Screen name="ledgers/[ledgerId]/transfer" options={{ title: "Transfer" }} />
          <Stack.Screen name="ledgers/[ledgerId]/move" options={{ title: "Move between pots" }} />
          <Stack.Screen name="ledgers/[ledgerId]/pots/new" options={{ title: "New pot" }} />
          <Stack.Screen name="ledgers/[ledgerId]/pots/[potId]" options={{ title: "Pot" }} />
          <Stack.Screen name="ledgers/[ledgerId]/categories/index" options={{ title: "Categories" }} />
          <Stack.Screen name="ledgers/[ledgerId]/categories/new" options={{ title: "New category" }} />
          <Stack.Screen name="ledgers/[ledgerId]/categories/[categoryId]" options={{ title: "Category" }} />
          <Stack.Screen name="ledgers/[ledgerId]/recurring/index" options={{ title: "Recurring payments" }} />
          <Stack.Screen name="ledgers/[ledgerId]/recurring/new" options={{ title: "New recurring payment" }} />
          <Stack.Screen name="ledgers/[ledgerId]/recurring/[paymentId]" options={{ title: "Recurring payment" }} />
          <Stack.Screen name="ledgers/[ledgerId]/transactions/[transactionId]" options={{ title: "Transaction" }} />
          <Stack.Screen name="ledgers/[ledgerId]/reconcile" options={{ title: "Reconcile" }} />
          <Stack.Screen name="households/[householdId]/members" options={{ title: "Members" }} />
        </Stack.Protected>
      </Stack>
      {/* Overlays such as dialogs and dropdowns render here, above all screens. */}
      <PortalHost />
      {/* Short confirmations after an action, e.g. "Transaction saved". */}
      <Toaster
        theme={scheme}
        // At the top, so it never covers the floating buttons or the tab bar.
        position="top-center"
        duration={2500}
        toastOptions={{ style: { backgroundColor: THEME[scheme].card, borderColor: THEME[scheme].border, borderWidth: 1 } }}
      />
    </ThemeProvider>
  );
}

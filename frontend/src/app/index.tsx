import { StyleSheet, Text, View } from "react-native";

// Placeholder start screen until login and the household overview exist.
export default function Index() {
  return (
    <View style={styles.container}>
      <Text style={styles.title}>Slowpoke Oasis</Text>
      <Text>Household book, tasks and shopping list.</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    gap: 8,
  },
  title: {
    fontSize: 24,
    fontWeight: "bold",
  },
});

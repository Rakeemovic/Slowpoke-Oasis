const { getDefaultConfig } = require("expo/metro-config");
const { withNativeWind } = require("nativewind/metro");

const config = getDefaultConfig(__dirname);

// inlineRem: 16 makes "rem" match the web default, as React Native Reusables expects.
module.exports = withNativeWind(config, { input: "./src/global.css", inlineRem: 16 });

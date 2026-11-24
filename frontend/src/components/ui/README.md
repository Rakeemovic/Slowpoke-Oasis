# UI components

These components come from [React Native Reusables](https://github.com/founded-labs/react-native-reusables),
the React Native port of shadcn/ui (NativeWind variant). Like with shadcn, the code is copied into the
project and may be changed here. Only the import paths were adjusted to `@/`.

They are licensed under the MIT license in [LICENSE](LICENSE); the rest of the app is AGPL-3.0-or-later.

To add a component, copy it from `packages/registry/src/nativewind/components/ui/` in that repository,
replace `@/registry/nativewind/` with `@/`, and install its `@rn-primitives/*` packages with
`npx expo install`.

Copied so far: button, card, icon, input, label, skeleton, switch, text. Changed against the originals: larger
touch targets (buttons, inputs and the switch reach 44–48 px on phones, after Apple's and Material's guidelines),
rounder corners, and headings in the app's display font (Nunito, see `tailwind.config.js`).

# Motion components

Ports of a few components from [Magic UI](https://magicui.design) (MIT license,
copyright Dillion Verma). Magic UI is built for the web with Framer Motion and
the DOM, which React Native cannot use, so these are rewritten with
React Native Reanimated and react-native-svg. They keep the idea and the
behaviour of the originals, not their code:

| Component | Magic UI original |
| --- | --- |
| `NumberTicker` | Number Ticker |
| `BlurFade` | Blur Fade (without the blur, which Android cannot do cheaply) |
| `CircularProgress` | Animated Circular Progress Bar |

All of them respect the system setting "reduce motion".

Use them sparingly: motion should explain a change (a balance moving, a list
appearing), never decorate.

import type { ReactNode } from "react";
import Animated, { FadeInDown } from "react-native-reanimated";

/**
 * Port of Magic UI's "Blur Fade": content fades in and slides up a little
 * when it appears. Lists pass their index, so rows appear one after another.
 * Android cannot blur views cheaply, so this keeps the fade and the slide.
 * Reanimated skips the animation when the system asks for reduced motion.
 */
export function BlurFade({ index = 0, className, children }: { index?: number; className?: string; children: ReactNode }) {
  // Long lists should not keep the user waiting: the delay stops growing after a few rows.
  const delay = Math.min(index, 8) * 40;
  return (
    <Animated.View entering={FadeInDown.duration(320).delay(delay)} className={className}>
      {children}
    </Animated.View>
  );
}

import type { ReactNode } from "react";
import { useEffect } from "react";
import { View } from "react-native";
import Animated, { useAnimatedProps, useReducedMotion, useSharedValue, withTiming } from "react-native-reanimated";
import Svg, { Circle } from "react-native-svg";

import { cn } from "@/lib/utils";

const AnimatedCircle = Animated.createAnimatedComponent(Circle);

/**
 * Port of Magic UI's "Animated Circular Progress Bar": a ring that fills up
 * to `fraction` (0…1) and animates when the value changes. The ring is
 * decoration; the numbers belong as text next to or inside it (`children`).
 * Colours come as CSS colour strings, because SVG cannot read Tailwind classes.
 */
export function CircularProgress({
  fraction,
  color,
  trackColor,
  size = 64,
  strokeWidth = 7,
  className,
  children,
}: {
  fraction: number;
  color: string;
  trackColor: string;
  size?: number;
  strokeWidth?: number;
  className?: string;
  children?: ReactNode;
}) {
  const reduceMotion = useReducedMotion();
  const radius = (size - strokeWidth) / 2;
  const circumference = 2 * Math.PI * radius;
  const target = Math.max(0, Math.min(1, fraction));
  const progress = useSharedValue(reduceMotion ? target : 0);

  useEffect(() => {
    progress.value = reduceMotion ? target : withTiming(target, { duration: 800 });
  }, [target, reduceMotion, progress]);

  const animatedProps = useAnimatedProps(() => ({
    strokeDashoffset: circumference * (1 - progress.value),
  }));

  return (
    <View aria-hidden className={cn("items-center justify-center", className)} style={{ width: size, height: size }}>
      <Svg width={size} height={size} style={{ position: "absolute", transform: [{ rotate: "-90deg" }] }}>
        <Circle cx={size / 2} cy={size / 2} r={radius} stroke={trackColor} strokeWidth={strokeWidth} fill="none" />
        <AnimatedCircle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          stroke={color}
          strokeWidth={strokeWidth}
          strokeLinecap="round"
          fill="none"
          strokeDasharray={`${circumference} ${circumference}`}
          animatedProps={animatedProps}
        />
      </Svg>
      {children}
    </View>
  );
}

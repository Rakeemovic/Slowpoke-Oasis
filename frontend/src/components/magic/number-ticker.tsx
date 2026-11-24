import { useEffect, useRef, useState, type ComponentProps } from "react";
import { useReducedMotion } from "react-native-reanimated";

import { Text } from "@/components/ui/text";

const DURATION_MS = 700;

/**
 * Port of Magic UI's "Number Ticker": the number counts towards its new value
 * instead of jumping. It counts from 0 on the first render and from the old
 * value on every change. With "reduce motion" switched on in the system, the
 * final value shows at once.
 *
 * Screen readers and the end-to-end tests get the final value through
 * aria-label, so they never see an intermediate number.
 */
export function NumberTicker({
  value,
  format,
  ...textProps
}: { value: number; format: (value: number) => string } & Omit<ComponentProps<typeof Text>, "children">) {
  const reduceMotion = useReducedMotion();
  const [shown, setShown] = useState(0);
  const shownRef = useRef(shown);

  useEffect(() => {
    if (reduceMotion) {
      // Rendered straight from `value` below; the next animation starts from here.
      shownRef.current = value;
      return;
    }
    const from = shownRef.current;
    const start = Date.now();
    let frame = 0;
    const step = () => {
      const t = Math.min(1, (Date.now() - start) / DURATION_MS);
      // Ease out: fast at first, then settling on the value like a spring.
      const eased = 1 - Math.pow(1 - t, 3);
      const next = t === 1 ? value : Math.round(from + (value - from) * eased);
      shownRef.current = next;
      setShown(next);
      if (t < 1) {
        frame = requestAnimationFrame(step);
      }
    };
    frame = requestAnimationFrame(step);
    return () => cancelAnimationFrame(frame);
  }, [value, reduceMotion]);

  return (
    <Text aria-label={format(value)} {...textProps}>
      {format(reduceMotion ? value : shown)}
    </Text>
  );
}

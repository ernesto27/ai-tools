import { useCallback, useEffect, useRef, useState } from "react";

const MAX_OUTPUT_LENGTH = 200_000;

export function useRunDebug() {
  const [output, setOutput] = useState<string | null>(null);
  const buffer = useRef("");
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const mounted = useRef(true);

  const flush = useCallback(() => {
    if (timer.current !== null) clearTimeout(timer.current);
    timer.current = null;
    if (mounted.current) setOutput(buffer.current);
  }, []);

  const reset = useCallback((initial: string | null = null) => {
    if (timer.current !== null) clearTimeout(timer.current);
    timer.current = null;
    buffer.current = initial ?? "";
    if (mounted.current) setOutput(initial);
  }, []);

  const append = useCallback((text: string) => {
    if (!mounted.current || !text) return;
    buffer.current = (buffer.current + text).slice(-MAX_OUTPUT_LENGTH);
    if (timer.current === null) timer.current = setTimeout(flush, 50);
  }, [flush]);

  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      if (timer.current !== null) clearTimeout(timer.current);
    };
  }, []);

  return { output, reset, append, flush };
}

import { useCallback, useEffect, useRef, useState } from "react";

export function useCopyFeedback(value: string) {
  const [copied, setCopied] = useState(false);
  const timeoutRef = useRef<number>(undefined);

  useEffect(() => () => window.clearTimeout(timeoutRef.current), []);

  const copy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.clearTimeout(timeoutRef.current);
      timeoutRef.current = window.setTimeout(() => setCopied(false), 1_200);
    } catch {
      setCopied(false);
    }
  }, [value]);

  return { copied, copy };
}

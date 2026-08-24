"use client";

import { FastAverageColor } from "fast-average-color";
import { type CSSProperties, type SyntheticEvent, useCallback, useState } from "react";

const averageColor = new FastAverageColor();
const BACKGROUND_CACHE_LIMIT = 100;
type IdentityBackground = CSSProperties & Record<string, string>;
const backgroundCache = new Map<string, IdentityBackground>();
const colorSamplingHosts = new Set([
  "cdn.jsdmirror.com",
  "cdn.jsdelivr.net",
  "fastly.jsdelivr.net",
  "gcore.jsdelivr.net",
  "raw.githubusercontent.com",
]);

export function useImageBackground(src?: string) {
  const canSample = supportsColorSampling(src);
  const [loadedSrc, setLoadedSrc] = useState<string>();
  const [sampled, setSampled] = useState<{
    src: string;
    style: IdentityBackground;
  }>();
  const background =
    sampled && sampled.src === src ? sampled.style : src ? backgroundCache.get(src) : undefined;

  const handleLoad = useCallback(
    (event: SyntheticEvent<HTMLImageElement>) => {
      if (!src) {
        return;
      }
      setLoadedSrc(src);
      if (!canSample) {
        return;
      }
      const result = averageColor.getColor(event.currentTarget, {
        algorithm: "sqrt",
        ignoredColor: [0, 0, 0, 0, 32],
        mode: "speed",
        silent: true,
      });
      if (!result.error) {
        const style = createBackgroundStyle(result.hex, result.value);
        cacheBackground(src, style);
        setSampled({ src, style });
      }
    },
    [canSample, src],
  );

  return {
    crossOrigin: canSample ? ("anonymous" as const) : undefined,
    handleLoad,
    isLoaded: Boolean(src) && loadedSrc === src,
    style: background,
  };
}

function createBackgroundStyle(
  hex: string,
  [red, green, blue]: [number, number, number, number],
): IdentityBackground {
  const chroma = Math.max(red, green, blue) - Math.min(red, green, blue);
  const luminance = red * 0.2126 + green * 0.7152 + blue * 0.0722;
  const style: IdentityBackground = { "--crypto-identity-color": hex };

  if (chroma <= 28 && luminance <= 72) {
    style["--crypto-identity-light-surface"] = "#d4d4d8";
    style["--crypto-identity-dark-filter"] = "invert(1)";
  } else if (chroma <= 28 && luminance >= 188) {
    style["--crypto-identity-light-surface"] = "#52525b";
    style["--crypto-identity-dark-surface"] = "#27272a";
  }

  return style;
}

function cacheBackground(src: string, style: IdentityBackground): void {
  if (!backgroundCache.has(src) && backgroundCache.size >= BACKGROUND_CACHE_LIMIT) {
    backgroundCache.clear();
  }
  backgroundCache.set(src, style);
}

function supportsColorSampling(src?: string): boolean {
  if (!src) {
    return false;
  }
  try {
    const url = new URL(src, window.location.href);
    return url.origin === window.location.origin || colorSamplingHosts.has(url.hostname);
  } catch {
    return false;
  }
}

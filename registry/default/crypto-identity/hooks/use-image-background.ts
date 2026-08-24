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
      if (!canSample || backgroundCache.has(src)) {
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
  const color: Rgb = [red, green, blue];
  const style: IdentityBackground = {
    "--crypto-identity-color": hex,
    "--crypto-identity-light-surface": lightSurface(color, chroma, luminance),
    "--crypto-identity-dark-surface": darkSurface(color, luminance),
  };

  if (chroma <= 28 && luminance <= 72) {
    style["--crypto-identity-dark-filter"] = "invert(1)";
  }

  return style;
}

type Rgb = [number, number, number];

function lightSurface(color: Rgb, chroma: number, luminance: number): string {
  if (chroma <= 28 && luminance <= 72) {
    return "rgb(241 245 249)";
  }
  if (luminance >= 188) {
    return rgb(scaleToLuminance(color, 104, luminance));
  }
  return rgb(mix(color, [255, 255, 255], 0.82));
}

function darkSurface(color: Rgb, luminance: number): string {
  if (luminance <= 72) {
    return "rgb(24 24 27)";
  }
  return rgb(mix(color, [9, 11, 17], luminance >= 188 ? 0.78 : 0.72));
}

function scaleToLuminance(color: Rgb, target: number, luminance: number): Rgb {
  const factor = target / luminance;
  return color.map((channel) => channel * factor) as Rgb;
}

function mix(color: Rgb, surface: Rgb, surfaceRatio: number): Rgb {
  return color.map(
    (channel, index) => channel * (1 - surfaceRatio) + (surface[index] ?? 0) * surfaceRatio,
  ) as Rgb;
}

function rgb(color: Rgb): string {
  return `rgb(${color.map((channel) => Math.round(Math.max(0, Math.min(255, channel)))).join(" ")})`;
}

function cacheBackground(src: string, style: IdentityBackground): void {
  if (!backgroundCache.has(src) && backgroundCache.size >= BACKGROUND_CACHE_LIMIT) {
    const oldest = backgroundCache.keys().next().value;
    if (oldest) {
      backgroundCache.delete(oldest);
    }
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

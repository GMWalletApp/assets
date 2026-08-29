"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { resolveIconUrls } from "../lib/resolve-icon-urls";
import type { AssetQuery, CryptoIdentityIcon } from "../lib/types";

export function useIconSource(icon?: CryptoIdentityIcon, preferredBaseUrl?: string) {
  const key = `${stableIconKey(icon)}:${preferredBaseUrl ?? ""}`;
  const activeKeyRef = useRef(key);
  const iconRef = useRef(icon);
  iconRef.current = icon;
  const [sources, setSources] = useState<string[]>([]);
  const [index, setIndex] = useState(0);
  const [isResolving, setIsResolving] = useState(Boolean(icon));

  useEffect(() => {
    const currentIcon = iconRef.current;
    activeKeyRef.current = key;
    setSources([]);
    setIndex(0);
    setIsResolving(Boolean(currentIcon));
    let cancelled = false;

    if (!currentIcon) {
      return;
    }

    resolveSources(currentIcon, preferredBaseUrl).then((urls) => {
      if (cancelled || activeKeyRef.current !== key) {
        return;
      }
      setSources(urls);
      setIsResolving(false);
    });

    return () => {
      cancelled = true;
    };
  }, [key, preferredBaseUrl]);

  const handleError = useCallback(() => {
    setIndex((value) => Math.min(value + 1, sources.length));
  }, [sources.length]);

  return { src: sources[index], isResolving, handleError };
}

function resolveSources(icon: AssetQuery, preferredBaseUrl?: string) {
  return preferredBaseUrl ? resolveIconUrls(icon, preferredBaseUrl) : resolveIconUrls(icon);
}

function stableIconKey(icon?: CryptoIdentityIcon): string {
  if (!icon) {
    return "";
  }
  return [
    icon.type,
    icon.name,
    icon.type === "token" ? icon.network : "",
    icon.type === "token" ? (icon.contractAddress ?? "") : "",
  ].join(":");
}

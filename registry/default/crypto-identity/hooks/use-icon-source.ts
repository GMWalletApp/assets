"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { resolveIconUrls } from "../lib/resolve-icon-urls";
import type { CryptoIdentityIcon } from "../lib/types";

export function useIconSource(icon?: CryptoIdentityIcon) {
  const key = stableIconKey(icon);
  const activeKeyRef = useRef(key);
  const iconRef = useRef(icon);
  iconRef.current = icon;
  const catalogRequestedRef = useRef(false);
  const [sources, setSources] = useState<string[]>([]);
  const [index, setIndex] = useState(0);
  const [isResolving, setIsResolving] = useState(Boolean(icon));

  useEffect(() => {
    const currentIcon = iconRef.current;
    activeKeyRef.current = key;
    setSources([]);
    setIndex(0);
    setIsResolving(Boolean(currentIcon));
    catalogRequestedRef.current = false;
    let cancelled = false;

    if (!currentIcon) {
      return;
    }

    resolveIconUrls(currentIcon).then((urls) => {
      if (cancelled) {
        return;
      }
      setSources(urls);
      if (urls.length === 0 && currentIcon.type === "token") {
        catalogRequestedRef.current = true;
        resolveIconUrls({ ...currentIcon, includeCatalog: true }).then((catalogUrls) => {
          if (!cancelled) {
            setSources(catalogUrls);
            setIsResolving(false);
          }
        });
        return;
      }
      setIsResolving(false);
    });

    return () => {
      cancelled = true;
    };
  }, [key]);

  const handleError = useCallback(() => {
    if (index + 1 < sources.length) {
      setIndex((value) => value + 1);
      return;
    }

    const currentIcon = iconRef.current;
    if (!currentIcon) {
      return;
    }
    if (catalogRequestedRef.current) {
      setIndex(sources.length);
      return;
    }

    if (currentIcon.type === "network" || currentIcon.type === "dapp") {
      setIndex(sources.length);
      return;
    }

    catalogRequestedRef.current = true;
    setIsResolving(true);
    const requestedKey = key;
    resolveIconUrls({ ...currentIcon, includeCatalog: true }).then((urls) => {
      if (activeKeyRef.current !== requestedKey) {
        return;
      }
      const fallbackUrls = urls.filter((url) => !sources.includes(url));
      setSources(fallbackUrls);
      setIndex(0);
      setIsResolving(false);
    });
  }, [index, key, sources]);

  return { src: sources[index], isResolving, handleError };
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

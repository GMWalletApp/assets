export interface TokenAssetQuery {
  type: "token";
  /** Token symbol or canonical asset name. */
  name: string;
  /** Canonical repository network key, such as `ethereum`, `smartchain`, or `tron`. */
  network: string;
  /** Chain-specific token contract address. */
  contractAddress?: string | null;
  /** Include matches from the complete compressed token catalog. */
  includeCatalog?: boolean;
}

interface SupportAssetQuery<T extends "exchange" | "wallet"> {
  type: T;
  /** Repository asset key or display name. */
  name: string;
  /** Include display-name matches from the compressed support catalog. */
  includeCatalog?: boolean;
}

interface DirectAssetQuery<T extends "network" | "dapp"> {
  type: T;
  /** Repository asset key without a file extension. */
  name: string;
}

export type AssetQuery =
  | TokenAssetQuery
  | SupportAssetQuery<"exchange">
  | SupportAssetQuery<"wallet">
  | DirectAssetQuery<"network">
  | DirectAssetQuery<"dapp">;

export type AssetType = AssetQuery["type"];
type WithoutCatalog<T> = T extends unknown ? Omit<T, "includeCatalog"> : never;

export type CryptoIdentityIcon = WithoutCatalog<AssetQuery>;

import { execFile } from "node:child_process";
import { writeFile } from "node:fs/promises";
import { promisify } from "node:util";

const run = promisify(execFile);
const { stdout } = await run("git", ["ls-tree", "-r", "--name-only", "origin/main", "dapps"]);
const names = stdout
  .trim()
  .split("\n")
  .filter(Boolean)
  .map((path) => path.replace(/^dapps\//, ""));
const source = [
  "export const DAPP_FILES = [",
  ...names.map((name) => `  ${JSON.stringify(name)},`),
  "] as const;",
  "",
].join("\n");

await writeFile(new URL("../src/data/dapps.ts", import.meta.url), source);

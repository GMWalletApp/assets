import { readdir, readFile, stat, writeFile } from "node:fs/promises";
import { dirname, extname, relative, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { parse } from "@babel/parser";
import * as t from "@babel/types";
import { __unstable__loadDesignSystem } from "@tailwindcss/node";

const DEFAULT_CSS_PATH = "src/styles.css";
const DEFAULT_SOURCE_PATHS = ["src", "registry"];
const SOURCE_EXTENSIONS = new Set([".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs"]);
const IGNORED_DIRECTORIES = new Set([".git", ".output", "coverage", "dist", "node_modules"]);
const CLASS_FUNCTIONS = new Set([
  "cc",
  "classNames",
  "clsx",
  "cn",
  "cnb",
  "ctl",
  "cva",
  "tv",
  "twJoin",
  "twMerge",
]);

type CanonicalizeCandidates = (
  candidates: string[],
  options?: { collapse?: boolean; logicalToPhysical?: boolean; rem?: number },
) => string[];

export interface CanonicalEdit {
  after: string;
  before: string;
  end: number;
  start: number;
}

interface CliOptions {
  cssPath: string;
  paths: string[];
  write: boolean;
}

export function canonicalizeSource(
  source: string,
  fileName: string,
  canonicalizeCandidates: CanonicalizeCandidates,
): { edits: CanonicalEdit[]; output: string } {
  const sourceFile = parse(source, {
    errorRecovery: true,
    plugins: ["typescript", "jsx"],
    sourceFilename: fileName,
    sourceType: "unambiguous",
  });
  const ranges = new Map<string, { end: number; start: number }>();

  function addString(node: t.StringLiteral | t.TemplateLiteral) {
    if (node.start == null || node.end == null) return;
    const start = node.start + 1;
    const end = node.end - 1;
    ranges.set(`${start}:${end}`, { end, start });
  }

  function collectClassExpression(node: t.Node): void {
    if (t.isStringLiteral(node) || (t.isTemplateLiteral(node) && node.expressions.length === 0)) {
      addString(node);
      return;
    }
    if (
      t.isParenthesizedExpression(node) ||
      t.isTSAsExpression(node) ||
      t.isTSTypeAssertion(node) ||
      t.isTSNonNullExpression(node) ||
      t.isTSSatisfiesExpression(node)
    ) {
      collectClassExpression(node.expression);
      return;
    }
    if (t.isArrayExpression(node)) {
      for (const element of node.elements) {
        if (element) collectClassExpression(element);
      }
      return;
    }
    if (t.isObjectExpression(node)) {
      for (const property of node.properties) {
        if (t.isObjectProperty(property)) collectClassExpression(property.value);
        if (t.isSpreadElement(property)) collectClassExpression(property.argument);
      }
      return;
    }
    if (t.isConditionalExpression(node)) {
      collectClassExpression(node.consequent);
      collectClassExpression(node.alternate);
      return;
    }
    if (t.isLogicalExpression(node)) {
      collectClassExpression(node.right);
      if (t.isStringLiteral(node.left)) collectClassExpression(node.left);
    }
  }

  function visit(node: t.Node): void {
    if (
      t.isJSXAttribute(node) &&
      t.isJSXIdentifier(node.name) &&
      (node.name.name === "className" || node.name.name === "class")
    ) {
      if (t.isStringLiteral(node.value)) addString(node.value);
      if (t.isJSXExpressionContainer(node.value) && t.isExpression(node.value.expression)) {
        collectClassExpression(node.value.expression);
      }
    }

    if (t.isCallExpression(node) && CLASS_FUNCTIONS.has(callName(node.callee))) {
      for (const argument of node.arguments) {
        if (!t.isArgumentPlaceholder(argument)) collectClassExpression(argument);
      }
    }

    for (const key of t.VISITOR_KEYS[node.type] ?? []) {
      const child = (node as unknown as Record<string, unknown>)[key];
      if (Array.isArray(child)) {
        for (const item of child) if (t.isNode(item)) visit(item);
      } else if (t.isNode(child)) {
        visit(child);
      }
    }
  }

  visit(sourceFile);

  const edits = Array.from(ranges.values())
    .map(({ end, start }): CanonicalEdit | null => {
      const before = source.slice(start, end);
      const tokens = splitClassList(before);
      if (tokens.length === 0) return null;

      const canonical = canonicalizeUntilStable(tokens, canonicalizeCandidates);
      if (sameTokens(tokens, canonical)) return null;

      const leading = before.match(/^\s*/)?.[0] ?? "";
      const trailing = before.match(/\s*$/)?.[0] ?? "";
      return { after: `${leading}${canonical.join(" ")}${trailing}`, before, end, start };
    })
    .filter((edit): edit is CanonicalEdit => edit !== null)
    .sort((a, b) => b.start - a.start);

  let output = source;
  for (const edit of edits)
    output = `${output.slice(0, edit.start)}${edit.after}${output.slice(edit.end)}`;

  return { edits: edits.slice().reverse(), output };
}

export function splitClassList(value: string): string[] {
  const tokens: string[] = [];
  let current = "";
  let depth = 0;
  let quote = "";
  let escaped = false;

  for (const character of value.trim()) {
    if (escaped) {
      current += character;
      escaped = false;
      continue;
    }
    if (character === "\\") {
      current += character;
      escaped = true;
      continue;
    }
    if (quote) {
      current += character;
      if (character === quote) quote = "";
      continue;
    }
    if (character === '"' || character === "'") {
      current += character;
      quote = character;
      continue;
    }
    if (character === "[" || character === "(" || character === "{") depth += 1;
    if (character === "]" || character === ")" || character === "}") depth -= 1;
    if (/\s/.test(character) && depth === 0) {
      if (current) tokens.push(current);
      current = "";
      continue;
    }
    current += character;
  }

  if (current) tokens.push(current);
  return tokens;
}

async function main() {
  const options = parseArgs(process.argv.slice(2));
  const root = process.cwd();
  const cssFile = resolve(root, options.cssPath);
  const css = await readFile(cssFile, "utf8");
  const designSystem = await __unstable__loadDesignSystem(css, { base: dirname(cssFile) });
  const files = await collectSourceFiles(options.paths.map((path) => resolve(root, path)));
  let changedFiles = 0;
  let editCount = 0;

  for (const file of files) {
    const source = await readFile(file, "utf8");
    const result = canonicalizeSource(source, file, (candidates, canonicalOptions) =>
      designSystem.canonicalizeCandidates(candidates, canonicalOptions),
    );
    if (result.edits.length === 0) continue;

    changedFiles += 1;
    editCount += result.edits.length;
    for (const edit of result.edits) {
      const position = lineAndColumn(source, edit.start);
      console.log(
        `${relative(root, file)}:${position.line}:${position.column} ${JSON.stringify(edit.before)} -> ${JSON.stringify(edit.after)}`,
      );
    }
    if (options.write) await writeFile(file, result.output, "utf8");
  }

  if (editCount === 0) {
    console.log(`Tailwind classes are canonical (${files.length} files checked).`);
    return;
  }

  const summary = `${editCount} replacement${editCount === 1 ? "" : "s"} in ${changedFiles} file${changedFiles === 1 ? "" : "s"}`;
  if (options.write) {
    console.log(`Canonicalized ${summary}.`);
    return;
  }

  console.error(`Found ${summary}. Run with --write to apply.`);
  process.exitCode = 1;
}

function parseArgs(args: string[]): CliOptions {
  const paths: string[] = [];
  let cssPath = DEFAULT_CSS_PATH;
  let write = false;

  for (let index = 0; index < args.length; index += 1) {
    const argument = args[index];
    if (argument === "--write") {
      write = true;
      continue;
    }
    if (argument === "--css") {
      const next = args[index + 1];
      if (!next) throw new Error("--css requires a path");
      cssPath = next;
      index += 1;
      continue;
    }
    if (argument === "--help" || argument === "-h") {
      console.log(
        "Usage: bun scripts/tailwind-canonicalize.ts [--write] [--css <path>] [files or directories]",
      );
      process.exit(0);
    }
    if (argument?.startsWith("-")) throw new Error(`Unknown option: ${argument}`);
    if (argument) paths.push(argument);
  }

  return { cssPath, paths: paths.length > 0 ? paths : DEFAULT_SOURCE_PATHS, write };
}

async function collectSourceFiles(paths: string[]): Promise<string[]> {
  const files: string[] = [];

  async function collect(path: string): Promise<void> {
    const details = await stat(path);
    if (details.isFile()) {
      if (isSourceFile(path)) files.push(path);
      return;
    }
    if (!details.isDirectory()) return;

    for (const entry of await readdir(path, { withFileTypes: true })) {
      if (entry.isDirectory() && IGNORED_DIRECTORIES.has(entry.name)) continue;
      await collect(resolve(path, entry.name));
    }
  }

  for (const path of paths) await collect(path);
  return files.sort();
}

function isSourceFile(path: string): boolean {
  return (
    SOURCE_EXTENSIONS.has(extname(path)) && !path.endsWith(".d.ts") && !path.endsWith(".gen.ts")
  );
}

function callName(expression: t.Expression | t.V8IntrinsicIdentifier): string {
  if (t.isIdentifier(expression)) return expression.name;
  if (t.isMemberExpression(expression) && t.isIdentifier(expression.property)) {
    return expression.property.name;
  }
  return "";
}

function sameTokens(left: string[], right: string[]): boolean {
  return left.length === right.length && left.every((token, index) => token === right[index]);
}

function canonicalizeUntilStable(
  candidates: string[],
  canonicalizeCandidates: CanonicalizeCandidates,
): string[] {
  let current = candidates;
  for (let attempt = 0; attempt < 8; attempt += 1) {
    const next = canonicalizeCandidates(current, {
      collapse: true,
      logicalToPhysical: false,
      rem: 16,
    });
    if (sameTokens(current, next)) return next;
    current = next;
  }
  throw new Error(`Tailwind canonicalization did not stabilize: ${candidates.join(" ")}`);
}

function lineAndColumn(source: string, offset: number): { column: number; line: number } {
  const preceding = source.slice(0, offset);
  const lines = preceding.split("\n");
  return { column: (lines.at(-1)?.length ?? 0) + 1, line: lines.length };
}

const entryPath = process.argv[1] ? pathToFileURL(resolve(process.argv[1])).href : "";
if (entryPath === import.meta.url) {
  main().catch((error: unknown) => {
    console.error(error instanceof Error ? error.message : error);
    process.exitCode = 2;
  });
}

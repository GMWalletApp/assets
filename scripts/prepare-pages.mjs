import { copyFile, writeFile } from "node:fs/promises";

const outputDirectory = new URL("../dist/client/", import.meta.url);

await Promise.all([
  copyFile(new URL("index.html", outputDirectory), new URL("404.html", outputDirectory)),
  writeFile(new URL(".nojekyll", outputDirectory), ""),
]);

// Writes THIRD_PARTY_NOTICES.md: the licenses of what ships in the app.
// Run with `bun run notices` after the dependencies change; the file is committed
// because the About screen embeds it.
import { execFileSync } from "node:child_process";
import { existsSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const root = new URL("..", import.meta.url).pathname;
const read = (p) => readFileSync(p, "utf8").trim();
const licenseFile = (dir) => {
  const f = readdirSync(dir).find((n) => /^(licen[sc]e|copying|unlicense)(\.|$)/i.test(n));
  return f ? read(join(dir, f)) : "";
};
const section = (name, meta, text) => `### ${name}\n\n${meta}\n\n\`\`\`text\n${text || "(no license file in the package)"}\n\`\`\`\n`;

// Native code and models that ship with the app or are downloaded by it.
const native = [
  ["whisper.cpp and ggml (the bundled whisper-server)", "MIT, https://github.com/ggml-org/whisper.cpp", "licenses/whisper.cpp.txt"],
  ["ONNX Runtime (bundled with sherpa-onnx)", "MIT, https://github.com/microsoft/onnxruntime", "licenses/onnxruntime.txt"],
  ["sherpa-onnx (bundled)", "Apache-2.0, https://github.com/k2-fsa/sherpa-onnx", "licenses/sherpa-onnx.txt"],
].map(([n, m, f]) => section(n, m, read(join(root, f))));

const models = `The models are not part of the app. It downloads them when you ask, from the places below.

- Whisper large-v3-turbo (ggml, q5_0): MIT. OpenAI Whisper, converted by the whisper.cpp project. https://huggingface.co/ggerganov/whisper.cpp
- pyannote segmentation 3.0 (ONNX): MIT. Copyright (c) 2022 CNRS, converted by sherpa-onnx. https://huggingface.co/pyannote/segmentation-3.0
- WeSpeaker ResNet34-LM, VoxCeleb (ONNX): Creative Commons Attribution 4.0 International (the license of the VoxCeleb data it was trained on). WeSpeaker, https://github.com/wenet-e2e/wespeaker; VoxCeleb, https://mm.kaist.ac.kr/datasets/voxceleb/
`;

// Go modules linked into the binary.
const go = execFileSync("go", ["list", "-deps", "-f", "{{if .Module}}{{.Module.Path}} {{.Module.Version}} {{.Module.Dir}} {{.Module.Main}}{{end}}", "."], { cwd: root, encoding: "utf8" })
  .split("\n").filter(Boolean).map((l) => l.split(" "))
  .filter(([, , , main]) => main !== "true");
const goMods = [...new Map(go.map(([p, v, d]) => [p, [p, v, d]])).values()].sort((a, b) => a[0].localeCompare(b[0]))
  .map(([p, v, d]) => section(`${p} ${v}`, "Go module", licenseFile(d)));

// npm packages the page bundles: the production dependencies and theirs.
// shadcn is the component CLI; only its CSS reaches the page, not its own dependencies.
const pkg = JSON.parse(read(join(root, "package.json")));
const seen = new Map();
const visit = (name, from) => {
  if (seen.has(name)) return;
  let dir = from;
  while (!existsSync(join(dir, "node_modules", name, "package.json"))) {
    const up = join(dir, "..");
    if (up === dir) return;
    dir = up;
  }
  const d = join(dir, "node_modules", name);
  const p = JSON.parse(read(join(d, "package.json")));
  seen.set(name, [name, p.version, p.license ?? "", licenseFile(d)]);
  if (name !== "shadcn") for (const dep of Object.keys(p.dependencies ?? {})) visit(dep, d);
};
for (const name of Object.keys(pkg.dependencies)) visit(name, root);
const npm = [...seen.values()].sort((a, b) => a[0].localeCompare(b[0]))
  .map(([n, v, l, t]) => section(`${n} ${v}`, `npm package, ${l || "license: see text"}`, t || `The package declares "${l}" in its package.json and ships no license text.`));

writeFileSync(join(root, "THIRD_PARTY_NOTICES.md"), [
  "# Third-party notices\n",
  "transflowy is MIT licensed (see LICENSE). It includes and uses the software below.\n",
  "## Bundled native code\n", ...native,
  "## Models\n", models,
  `## Go modules (${goMods.length})\n`, ...goMods,
  `## npm packages (${npm.length})\n`, ...npm,
].join("\n"));
console.log(`THIRD_PARTY_NOTICES.md: ${goMods.length} Go modules, ${npm.length} npm packages`);

"use strict";

const queryInput = document.querySelector("#query-input");
const parseButton = document.querySelector("#parse-button");
const normalizeButton = document.querySelector("#normalize-button");
const copyButton = document.querySelector("#copy-button");
const output = document.querySelector("#output");
const outputCode = output.querySelector("code");
const runtimeStatus = document.querySelector("#runtime-status");
const processMeta = document.querySelector("#process-meta");
const commandMeta = document.querySelector("#command-meta");
const exampleButtons = document.querySelectorAll(".example");

let wasmModulePromise;
let lastOutput = "";
let running = false;

function setStatus(kind, message) {
  runtimeStatus.className = `status ${kind}`;
  runtimeStatus.lastElementChild.textContent = message;
}

function setOutput(text, isError = false) {
  lastOutput = text;
  outputCode.textContent = text;
  output.classList.toggle("error", isError);
  copyButton.disabled = text.length === 0;
}

async function loadModule() {
  const response = await fetch("./qsgo.wasm", { cache: "no-cache" });
  if (!response.ok) {
    throw new Error(`WebAssembly download failed (HTTP ${response.status})`);
  }
  const bytes = await response.arrayBuffer();
  return WebAssembly.compile(bytes);
}

async function prepareRuntime() {
  if (typeof Go !== "function") {
    throw new Error("Go browser runtime did not load");
  }
  wasmModulePromise = loadModule();
  await wasmModulePromise;
  parseButton.disabled = false;
  normalizeButton.disabled = false;
  setStatus("ready", "Go/WASM ready");
  setOutput("Ready. Choose Parse or Normalize to execute the actual qs-go CLI.");
}

async function execute(command) {
  if (running) {
    return;
  }

  const query = queryInput.value;
  if (query.length > 4096) {
    setOutput("Demo input exceeds the 4 KiB presentation limit.", true);
    return;
  }

  running = true;
  parseButton.disabled = true;
  normalizeButton.disabled = true;
  copyButton.disabled = true;
  setStatus("", "Executing Go…");
  setOutput("Starting a fresh Go/WASM instance…");

  const streams = { 1: "", 2: "" };
  const decoders = { 1: new TextDecoder(), 2: new TextDecoder() };
  const originalWriteSync = globalThis.fs.writeSync;
  let exitCode = 0;
  const started = performance.now();

  globalThis.fs.writeSync = (fd, bytes) => {
    const stream = fd === 2 ? 2 : 1;
    streams[stream] += decoders[stream].decode(bytes, { stream: true });
    return bytes.length;
  };

  try {
    const go = new Go();
    go.argv = ["qsgo", command, query];
    go.exit = (code) => {
      exitCode = code;
    };

    const module = await wasmModulePromise;
    const instance = await WebAssembly.instantiate(module, go.importObject);
    await go.run(instance);

    streams[1] += decoders[1].decode();
    streams[2] += decoders[2].decode();
    const elapsed = performance.now() - started;
    const stdout = streams[1].trimEnd();
    const stderr = streams[2].trimEnd();

    if (exitCode !== 0 || stderr) {
      setOutput(stderr || `qs-go exited with status ${exitCode}`, true);
      setStatus("error", `Go exited ${exitCode}`);
    } else {
      setOutput(stdout || "(no output)");
      setStatus("ready", "Go/WASM ready");
    }
    processMeta.textContent = `Fresh Go/WASM instance · ${elapsed.toFixed(1)} ms in this browser`;
    commandMeta.textContent = `Command: qsgo ${command}`;
  } catch (error) {
    setOutput(error instanceof Error ? error.message : String(error), true);
    setStatus("error", "Runtime error");
  } finally {
    globalThis.fs.writeSync = originalWriteSync;
    running = false;
    parseButton.disabled = false;
    normalizeButton.disabled = false;
    copyButton.disabled = lastOutput.length === 0;
  }
}

parseButton.addEventListener("click", () => execute("parse"));
normalizeButton.addEventListener("click", () => execute("normalize"));

queryInput.addEventListener("keydown", (event) => {
  if (event.ctrlKey && event.key === "Enter") {
    event.preventDefault();
    execute("parse");
  }
});

for (const button of exampleButtons) {
  button.addEventListener("click", () => {
    queryInput.value = button.dataset.query || "";
    queryInput.focus();
  });
}

copyButton.addEventListener("click", async () => {
  try {
    await navigator.clipboard.writeText(lastOutput);
    const previous = copyButton.textContent;
    copyButton.textContent = "Copied";
    window.setTimeout(() => {
      copyButton.textContent = previous;
    }, 1200);
  } catch {
    copyButton.textContent = "Select output";
    output.focus();
  }
});

prepareRuntime().catch((error) => {
  setStatus("error", "Runtime unavailable");
  setOutput(error instanceof Error ? error.message : String(error), true);
});

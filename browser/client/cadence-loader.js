(() => {
  const script = document.currentScript;
  const wasm = script.dataset.wasm;
  const manifests = [...document.querySelectorAll("script[data-cadence-manifest]")]
    .map(node => JSON.parse(node.textContent));
  globalThis.__cadenceManifests = manifests;
  const go = new Go();
  const load = WebAssembly.instantiateStreaming(fetch(wasm), go.importObject)
    .catch(async () => WebAssembly.instantiate(await (await fetch(wasm)).arrayBuffer(), go.importObject));
  load.then(result => go.run(result.instance));
})();

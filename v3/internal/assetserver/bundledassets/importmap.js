
/**
 * importmapPolyfill loads es-module-shims from the bundled assets if necessary
 * 
 * Credit: https://github.com/guybedford/es-module-shims/issues/371#issuecomment-1800861026
 * es-module-shims: https://www.npmjs.com/package/es-module-shims
 */
function importmapPolyfill() {
  if (!(HTMLScriptElement.supports && HTMLScriptElement.supports('importmap'))) {
    const polyfill = document.createElement('script');
    polyfill.async = true;
    polyfill.src = "/wails/es-module-shims.js";

    document.head.appendChild(polyfill);
    console.debug("es-module-shims loaded to pollyfill import maps")
  }
}
importmapPolyfill();

/**
 * injectImportmap dynamically injects an import map
 * 
 * Credit: https://www.npmjs.com/package/es-module-shims#dynamic-import-maps
 */
function injectImportmap() {
  document.head.prepend(Object.assign(document.createElement('script'), {
    type: 'importmap',
    innerHTML: JSON.stringify({ imports: { "@wailsio/runtime": "/wails/runtime.js" } }),
  }));
}
injectImportmap();
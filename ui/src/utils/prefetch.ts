/**
 * Prefetch utilities — trigger lazy chunk downloads on hover so they are
 * already in the browser cache when the user clicks.
 */

let monacoInitialized = false;

/**
 * Starts downloading Monaco Editor assets.
 * Safe to call multiple times; subsequent calls are no-ops.
 * Attach to `onMouseEnter` of any button/tab that opens a YAML editor.
 */
export function prefetchMonaco(): void {
  if (monacoInitialized) return;
  monacoInitialized = true;
  import('@monaco-editor/react')
    .then(({ loader }) => { loader.init().catch(() => { /* noop */ }); })
    .catch(() => { /* noop */ });
}

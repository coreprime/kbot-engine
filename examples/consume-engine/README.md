# consume-engine

A standalone consumer proof for the published `@coreprime/kbot-engine`
package: it installs the artifact exactly as a downstream project would
and drives it headless in Node, asserting the engine loads and steps.

```sh
npm install   # resolves @coreprime/kbot-engine from the registry (@coreprime scope auth required)
npm test      # runs test.mjs
```

This example is not part of the package's gated CI; it exists to verify,
by hand or in a downstream job, that the published tarball is consumable.

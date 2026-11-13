# Notice

The web kit embeds the third-party files below. Each keeps its own license.

## Scripts

| File | Version | License | sha256 |
|---|---|---|---|
| `third_party/mermaid.min.js` | mermaid 11.17.2 | MIT | 581ed7d74bd9048d0e3a91363927d72ef22942d7722546b27f7cc29e35390eb8 |
| `third_party/htmx.min.js` | htmx 2.0.4 | 0BSD | e209dda5c8235479f3166defc7750e1dbcd5a5c1808b7792fc2e6733768fb447 |
| `third_party/idiomorph-ext.min.js` | idiomorph 0.7.3 | 0BSD | 1589f425608653a841fa06ef2bb103a59e19f1e6d27064cff894d365efd6d948 |
| `third_party/htmx-ext-sse.min.js` | htmx-ext-sse 2.2.4 | 0BSD | 98a46496de0c3605fbffdce9167ba427bdd9553184f83f149c261891a92c0136 |
| rough.js (used by the `excalidraw` package, not by the web kit) | 4.6.6 | MIT | not vendored here |

The mermaid bundle also includes these libraries. The source column says where the license was read.

| Library | License | Source |
|---|---|---|
| DOMPurify 3.4.12 | Apache-2.0 or MPL-2.0 | license comment in the bundle |
| lodash | MIT | license comment in the bundle |
| underscore | MIT | license comment in the bundle |
| bezier-easing | MIT | license comment in the bundle |
| code by Ralf S. Engelschall | MIT | license comment in the bundle |
| d3 7.9.2 | ISC | `license` field in its `package.json` |
| dagre-d3-es 7.0.14 | MIT | `license` field in its `package.json` |
| cytoscape 3.34.4 | MIT | `license` field in its `package.json` |
| katex 0.16.47 | MIT | `license` field in its `package.json` |
| khroma 2.1.0 | MIT | `license` file in its package |

## Fonts

| File | Font | License | Source |
|---|---|---|---|
| `assets/InstrumentSerif.woff2` | Instrument Serif | SIL OFL 1.1 | fontsource `package.json` |
| `assets/SourceSans3.woff2` | Source Sans 3 | SIL OFL 1.1 | fontsource `package.json` |
| `assets/IBMPlexMono.woff2` | IBM Plex Mono | SIL OFL 1.1 | fontsource `package.json` |
| `assets/Excalifont.woff2` | Excalifont | SIL OFL 1.1 | not verified in a file |

The web kit embeds each font as a base64 data URL. It loads nothing from the network.

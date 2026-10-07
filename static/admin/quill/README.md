# Quill (vendored)

The admin rich-text editor. Served from this directory (`/static/admin/quill/`)
because the admin Content-Security-Policy (`internal/middleware/security.go`)
only allows scripts and stylesheets from `'self'`; a CDN copy is blocked.

| | |
|---|---|
| Package | `quill` |
| Version | 2.0.3 |
| Licence | BSD-3-Clause (see `LICENSE`, `quill.js.LICENSE.txt`) |
| Upstream | https://github.com/slab/quill |
| Tarball | https://registry.npmjs.org/quill/-/quill-2.0.3.tgz |
| Tarball integrity | `sha512-xEYQBqfYx/sfb33VJiKnSJp8ehloavImQ2A6564GAbqG55PGw1dAWUn1MUbQB62t0azawUS2CZZhWCjO8gRvTw==` |
| Fetched | 2026-10-07 |

Files are byte-for-byte copies from the npm tarball (`package/dist/`,
`package/LICENSE`), identical to what the templates loaded from jsDelivr before.
Do not edit them.

| File | Source URL | SHA-256 |
|---|---|---|
| `quill.js` | https://cdn.jsdelivr.net/npm/quill@2.0.3/dist/quill.js | `f6157c72ac9b3f51cdead426335688a027b12405d9d6a4daadd38a676b2d7ff2` |
| `quill.snow.css` | https://cdn.jsdelivr.net/npm/quill@2.0.3/dist/quill.snow.css | `1c7948cd13aa92fac6390319bc1e5e461823da171519d3a768db56164f871636` |
| `quill.js.LICENSE.txt` | https://cdn.jsdelivr.net/npm/quill@2.0.3/dist/quill.js.LICENSE.txt | `7b1938804d68d96764233d0a148a7501f39e547e6ae2dead4b16836e9b8d123c` |
| `LICENSE` | https://cdn.jsdelivr.net/npm/quill@2.0.3/LICENSE | `395c12b616d6f58238b4be39284d4d9221b58dd6e1f1e34d9ab537e34abbb022` |

`TestVendoredQuill_MatchesNotice` (internal/handlers) checks these hashes.

## Updating

1. Download the new tarball from the npm registry and verify its integrity.
2. Replace the four files, update the version, URLs and hashes above.
3. Update the `?v=` query on the two tags in `internal/handlers/admin_templates.go`.

## Native dialogs

`quill.js` contains two `prompt()` fallbacks in its toolbar module (link, and
embeds with no handler). Neither is reachable here: the `snow` theme supplies
its own image/video/formula handlers and the admin templates register a styled
link handler. Any new toolbar button for an embed format needs a handler, or
Quill falls back to the native prompt.

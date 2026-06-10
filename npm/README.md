# overplane

Overplane CLI — evolve verified software.

This package installs the native `overplane` binary (Go) for your platform
from the project's [GitHub releases](https://github.com/overplane/overplane/releases),
verifying it against the release's sha256 `checksums.txt` (which is itself
signed with cosign keyless; see the release notes for verification steps).

```sh
npm install -g overplane
overplane version
```

Supported platforms: Linux, macOS, and Windows on x64 and arm64.

If you install with `--ignore-scripts`, the binary is not downloaded; run
`npm rebuild overplane` afterwards.

- Website: <https://www.overplane.dev/>
- Source: <https://github.com/overplane/overplane>
- License: Apache-2.0

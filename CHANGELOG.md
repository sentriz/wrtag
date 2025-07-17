# Changelog

## [0.15.0](https://www.github.com/sentriz/wrtag/compare/v0.14.0...v0.15.0) (2025-07-17)


### Features

* **metadata:** print keys in order they came ([7abac5d](https://www.github.com/sentriz/wrtag/commit/7abac5d507a135ef1e69fbc92e22ffebaeef68a7))
* **musicdesc:** new addon for key and bpm detection ([53b79bd](https://www.github.com/sentriz/wrtag/commit/53b79bdb08dd4addb09aad740bfebe925cca16f2)), closes [#110](https://www.github.com/sentriz/wrtag/issues/110) [#109](https://www.github.com/sentriz/wrtag/issues/109)
* **wrtagweb:** add pprof handlers ([82f87b3](https://www.github.com/sentriz/wrtag/commit/82f87b3836fdd2e0d4da418d366e21ac08aa13c9))
* **wrtag:** write `COMPOSER` tag ([1dad865](https://www.github.com/sentriz/wrtag/commit/1dad865699a635c402c39e21c8aa9d865ae25627))
* **wrtag:** write `ISRC` tag ([5004569](https://www.github.com/sentriz/wrtag/commit/5004569b191e965b19c1abf31ddcf103b2614e37))


### Bug Fixes

* **wrtag:** handle case where src dirs have string prefix of path-format root ([2162391](https://www.github.com/sentriz/wrtag/commit/2162391cec28e43a5089dc86d5b4695e3e5d0ae3)), closes [#135](https://www.github.com/sentriz/wrtag/issues/135)

## [0.14.0](https://www.github.com/sentriz/wrtag/compare/v0.13.0...v0.14.0) (2025-06-28)


### ⚠ BREAKING CHANGES

* **wrtag:** rename `tag-weight` option to `diff-weight`
* **wrtag:** tag mainly as `BARCODE` instead of `UPC`, use `Barcode` in research-links
* **wrtag:** clear unknown tags by default

### Features

* **config:** add discogs research-link example ([f3ddfef](https://www.github.com/sentriz/wrtag/commit/f3ddfefd3478155499da2b2394905361f2a02503))
* **pathformat:** add `artistsSort` and `artistsSortString` helpers ([6f63702](https://www.github.com/sentriz/wrtag/commit/6f63702fefe02b79e017b9eaadc8053d985b744a))
* **pathformat:** add `the` helper ([b53bccc](https://www.github.com/sentriz/wrtag/commit/b53bccc10027bfb9f54282540e40bc1f3eef76c3)), closes [#123](https://www.github.com/sentriz/wrtag/issues/123)
* **pathformat:** set index to 0 for pregap tracks ([4f9c2c2](https://www.github.com/sentriz/wrtag/commit/4f9c2c2369ea450cf50819ac87cf6c1e5c130cfa)), closes [#124](https://www.github.com/sentriz/wrtag/issues/124)
* **tagmap:** also keep ReplayGain range settings, and INITIALKEY ([7afda34](https://www.github.com/sentriz/wrtag/commit/7afda3446e4527539e834918a86c37a3338280a6))
* **tagmap:** write "MUSICBRAINZ_RELEASETRACKID" tag ([4f13881](https://www.github.com/sentriz/wrtag/commit/4f13881adb7779eb08a5778f13f5cb6c69ade7b5))
* **tagmap:** write "RELEASETYPE" tag ([c19bd5d](https://www.github.com/sentriz/wrtag/commit/c19bd5d270d7ef24dd067af2a2b60dd01d93f0d4))
* **tags:** add more known tags and variants ([9fb9f2a](https://www.github.com/sentriz/wrtag/commit/9fb9f2ae2795791dc6fbfe91cdd41f6dcdeb5299))
* **wrtag:** add `tag-config` option ([97acec5](https://www.github.com/sentriz/wrtag/commit/97acec5819607cb19b19f168ebadcb636f2abdff)), closes [#120](https://www.github.com/sentriz/wrtag/issues/120) [#107](https://www.github.com/sentriz/wrtag/issues/107)
* **wrtag:** add some more tests ([b96b53f](https://www.github.com/sentriz/wrtag/commit/b96b53fe021695158e462ad4ac6a0234532d877a))
* **wrtag:** clear unknown tags by default ([0c94f10](https://www.github.com/sentriz/wrtag/commit/0c94f100bf75e9e22a4a5219198af5482c67c2df)), closes [#115](https://www.github.com/sentriz/wrtag/issues/115)
* **wrtag:** rename `tag-weight` option to `diff-weight` ([a25e256](https://www.github.com/sentriz/wrtag/commit/a25e256a061fe3bb2bb864cef59e89fd6a44ef3f))
* **wrtag:** tag mainly as `BARCODE` instead of `UPC`, use `Barcode` in research-links ([e263dd6](https://www.github.com/sentriz/wrtag/commit/e263dd633970a6eebb31a23cc88de8bfea179aad)), closes [#121](https://www.github.com/sentriz/wrtag/issues/121)
* **wrtag:** write REMIXER/REMIXERS tags ([238ef45](https://www.github.com/sentriz/wrtag/commit/238ef451e9a5ad6a5d3f2a68b2ed820def55b592)), closes [#77](https://www.github.com/sentriz/wrtag/issues/77)


### Bug Fixes

* **wrtag:** remove `TRACKC` tag alternative ([d1d41c5](https://www.github.com/sentriz/wrtag/commit/d1d41c5e9b9f8bb28811fdbcf43034b1b99c9491))

## [0.13.0](https://www.github.com/sentriz/wrtag/compare/v0.12.0...v0.13.0) (2025-05-30)


### Features

* **ci:** use native gha cache ([9795428](https://www.github.com/sentriz/wrtag/commit/97954283ba4e4bd195e3364e972cf44e98520c87))
* **lyrics:** esc `&` in for Genius ([2bd0b3e](https://www.github.com/sentriz/wrtag/commit/2bd0b3e698bbf0f19c607a5d804ad15679260036))

## [0.12.0](https://www.github.com/sentriz/wrtag/compare/v0.11.0...v0.12.0) (2025-05-29)


### Features

* **ci:** add errcheck ([6ff44e1](https://www.github.com/sentriz/wrtag/commit/6ff44e1d17c119152b2b4730aeb3d46e7905cc12))
* **ci:** add more linters ([69f2c82](https://www.github.com/sentriz/wrtag/commit/69f2c8200217c88e98a27f414b720abbf325fb88))
* **ci:** cache docker builds ([8f2a5c4](https://www.github.com/sentriz/wrtag/commit/8f2a5c4c1b326207aeb283402b138beaa658340a))
* **metadata:** buffer stdout for `read` ([8911983](https://www.github.com/sentriz/wrtag/commit/89119832a6a9d21137f7ea666b471295566433ee))
* **wrtag:** adjust tag match to account for larger left side ([0de5233](https://www.github.com/sentriz/wrtag/commit/0de52339085dd012551be8e9a3676e4542a089f9))
* **wrtagweb:** restart old in-progress jobs on startup ([173fad9](https://www.github.com/sentriz/wrtag/commit/173fad9bebaac432dd9ee588a38ff45004cbe00b))
* **wrtagweb:** set manual import form width to 500px by default ([#102](https://www.github.com/sentriz/wrtag/issues/102)) ([d2ae6a1](https://www.github.com/sentriz/wrtag/commit/d2ae6a1daa9c54c81e9f2a99fec8f02404649480))


### Bug Fixes

* **lyrics:** pass rate limit lyric sources ([a29d477](https://www.github.com/sentriz/wrtag/commit/a29d4776a573ecf31b729a625b5d17cc94a9c6c7))
* **wrtagweb:** horizonal overflow on mobile ([0dc263b](https://www.github.com/sentriz/wrtag/commit/0dc263b959a32c7305201f8dffc9bcf9a1aba6e2)), closes [#106](https://www.github.com/sentriz/wrtag/issues/106)
* **wrtagweb:** return error in template execute ([07d1b31](https://www.github.com/sentriz/wrtag/commit/07d1b31e5bd117e1a50e332e374d47b91c3ef469))

## [0.11.0](https://www.github.com/sentriz/wrtag/compare/v0.9.0...v0.11.0) (2025-04-17)


### ⚠ BREAKING CHANGES

* **ci:** upgrade to go1.24, bump golangci-lint

### Features

* **ci:** upgrade to go1.24, bump golangci-lint ([1c8c8e0](https://www.github.com/sentriz/wrtag/commit/1c8c8e0fa55aed786b23d34dcf1b4f498e0dff95))
* **wrtag:** add a `reflink` operation ([343f601](https://www.github.com/sentriz/wrtag/commit/343f6019f579b47be7575ff18bf120b3a0b1eb04)), closes [#87](https://www.github.com/sentriz/wrtag/issues/87)
* **wrtag:** support atomic copies ([77767dc](https://www.github.com/sentriz/wrtag/commit/77767dc741d79302254af871895ba46b5f1a2d7a))
* **wrtagweb:** process user actions in job loop ([9ec8b67](https://www.github.com/sentriz/wrtag/commit/9ec8b67198af43fd89d015cfd9470b6d80df9501))


### Bug Fixes

* **wrtag:** clamp score to 0-100 ([3f587d4](https://www.github.com/sentriz/wrtag/commit/3f587d44f87f879a4c44e9de58229958078037df))

## [0.9.0](https://www.github.com/sentriz/wrtag/compare/v0.8.1...v0.9.0) (2025-03-23)


### Features

* **wrtag:** support using english locale paths in path-format ([62b776e](https://www.github.com/sentriz/wrtag/commit/62b776e5ff42c5f963fc53ae2a08f6938f024277)), closes [#85](https://www.github.com/sentriz/wrtag/issues/85)

### [0.8.1](https://www.github.com/sentriz/wrtag/compare/v0.8.0...v0.8.1) (2025-03-16)


### Bug Fixes

* **lyrics:** write to file not lyrics string ([47d83a4](https://www.github.com/sentriz/wrtag/commit/47d83a42b7b74d96b9c22b2c078d90766df19957)), closes [#81](https://www.github.com/sentriz/wrtag/issues/81)

## [0.8.0](https://www.github.com/sentriz/wrtag/compare/v0.7.0...v0.8.0) (2025-03-14)


### ⚠ BREAKING CHANGES

* **pathformat:** tidy up keys

### Features

* **musicbrainz:** compilation if VA or compilation secondary types ([c38bc59](https://www.github.com/sentriz/wrtag/commit/c38bc5906b15ed191f99895c2a7d6e403b980e63))


### Code Refactoring

* **pathformat:** tidy up keys ([58ea3a8](https://www.github.com/sentriz/wrtag/commit/58ea3a810fd98856c68d53d591a9970c561c6fbd)), closes [#80](https://www.github.com/sentriz/wrtag/issues/80)

## [0.7.0](https://www.github.com/sentriz/wrtag/compare/v0.6.1...v0.7.0) (2025-01-04)


### Features

* **wrtag:** avoid more IO when no tag changes ([b4da400](https://www.github.com/sentriz/wrtag/commit/b4da4000fa4c597cf4fd1bd5c9771260bcb953ca))


### Bug Fixes

* **tagmap:** normalise empty vs null tags ([676d640](https://www.github.com/sentriz/wrtag/commit/676d6404176cb4708a8c85d9a912005fb18550c1))
* **wrtag:** don't wipe unknown metadata ([1a9f99e](https://www.github.com/sentriz/wrtag/commit/1a9f99ea3a0c267323bcd7ef3b90b9a3d5950779))

### [0.6.1](https://www.github.com/sentriz/wrtag/compare/v0.6.0...v0.6.1) (2024-12-11)


### Bug Fixes

* **wrtag:** fix windows tag read/write ([e5c9013](https://www.github.com/sentriz/wrtag/commit/e5c901365d3c2539b948193868788ea74152c2ae))

## [0.6.0](https://www.github.com/sentriz/wrtag/compare/v0.5.1...v0.6.0) (2024-12-06)


### Features

* rebrand back to wrtag ([2a9c836](https://www.github.com/sentriz/wrtag/commit/2a9c836120a3ef360ec7c7ed2c138d7f5f6f8e8b))

### [0.5.1](https://www.github.com/sentriz/wrtag/compare/v0.5.0...v0.5.1) (2024-12-05)


### Bug Fixes

* **ci:** upload binaries ([9d8d9b3](https://www.github.com/sentriz/wrtag/commit/9d8d9b324d967890f1823463849169ef66fe21c4))

## [0.5.0](https://www.github.com/sentriz/wrtag/compare/v0.4.0...v0.5.0) (2024-12-05)


### Features

* **ci:** use matrix to build binaries ([938ae37](https://www.github.com/sentriz/wrtag/commit/938ae379056646a4f3801405d136b7d8273e34f1))

## [0.4.0](https://www.github.com/sentriz/wrtag/compare/v0.3.0...v0.4.0) (2024-12-05)


### Features

* **ci:** don't use qemu for multi platform builds ([b4b90c0](https://www.github.com/sentriz/wrtag/commit/b4b90c08eeedcd500c7a0961759d4b9798cb1e81))

## [0.3.0](https://www.github.com/sentriz/wrtag/compare/v0.2.2...v0.3.0) (2024-12-01)


### ⚠ BREAKING CHANGES

* rebrand to wrtag
* rename `wrtagsync` -> `wrtag sync`

### Features

* rebrand to wrtag ([a8399af](https://www.github.com/sentriz/wrtag/commit/a8399af5452f037689d1f66ad57907541c1d9a93)), closes [#58](https://www.github.com/sentriz/wrtag/issues/58)
* rename `wrtagsync` -> `wrtag sync` ([a3c097f](https://www.github.com/sentriz/wrtag/commit/a3c097f1197d4e63780c0b66be08a8c3ff7c379c))

### [0.2.2](https://www.github.com/sentriz/wrtag/compare/v0.2.1...v0.2.2) (2024-11-28)


### Bug Fixes

* **ci:** upload binaries to output tag ([c0b5677](https://www.github.com/sentriz/wrtag/commit/c0b5677b9b077cc2c710d5712f2b3531a377bf4f))

### [0.2.1](https://www.github.com/sentriz/wrtag/compare/v0.2.0...v0.2.1) (2024-11-28)


### Bug Fixes

* **ci:** don't use hardcoded binary names ([c9a80b2](https://www.github.com/sentriz/wrtag/commit/c9a80b2be3d4f2ee38e932169ab2701fd6983584))

## [0.2.0](https://www.github.com/sentriz/wrtag/compare/v0.1.0...v0.2.0) (2024-11-28)


### Features

* **ci:** faster binary build ([696eb83](https://www.github.com/sentriz/wrtag/commit/696eb838bdd2a5608359a475faa80f3c28c740e8))

## 0.1.0 (2024-11-28)


### ⚠ BREAKING CHANGES

* **wrtagweb:** replace bolt with sqlite

### Features

* **ci:** add binaries ([dcf0424](https://www.github.com/sentriz/wrtag/commit/dcf042458978ec0743e79b8b43abb0759e61ab49))
* **clientutil:** log with ctx ([814372a](https://www.github.com/sentriz/wrtag/commit/814372ac47c3e8847634d21e3bdaab753499cf96))
* use go.senan.xyz/taglib-wasm ([5318e65](https://www.github.com/sentriz/wrtag/commit/5318e65c4a1ebb386e442c2056eae9304b5ffaab))
* **wrtag:** validate situations where tracks can't be sorted before matching ([20c616a](https://www.github.com/sentriz/wrtag/commit/20c616a13e5f112a88e42c724f545534a2279393)), closes [#52](https://www.github.com/sentriz/wrtag/issues/52)
* **wrtagweb:** enforce db path ([a6bf28f](https://www.github.com/sentriz/wrtag/commit/a6bf28f8ae4a8917abc24ee34d966b519d1a8358))
* **wrtagweb:** replace bolt with sqlite ([26e6889](https://www.github.com/sentriz/wrtag/commit/26e688999e252ca5c15eb4c14433319e4b0ae195))


### Bug Fixes

* **metadata:** adjust help output ([76568c5](https://www.github.com/sentriz/wrtag/commit/76568c5ed8382647a3ede5ce9421c85b8cd4a33c))
* **tag:** bump go-taglib-wasm ([cdfb74c](https://www.github.com/sentriz/wrtag/commit/cdfb74ca3453139ec471c236b244c56c353a57ab))

# Changelog

## [0.10.0](https://www.github.com/sentriz/wrtag/compare/v0.9.0...v0.10.0) (2025-03-29)


### Features

* **wrtag:** add a `reflink` operation ([343f601](https://www.github.com/sentriz/wrtag/commit/343f6019f579b47be7575ff18bf120b3a0b1eb04)), closes [#87](https://www.github.com/sentriz/wrtag/issues/87)
* **wrtag:** support atomic copies ([77767dc](https://www.github.com/sentriz/wrtag/commit/77767dc741d79302254af871895ba46b5f1a2d7a))

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

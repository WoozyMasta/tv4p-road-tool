# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog][],
and this project adheres to [Semantic Versioning][].

<!--
## Unreleased

### Added
### Changed
### Removed
-->

## [0.1.1][] - 2026-02-01

### Added

* Crossroads support `crossroad_types` with `default` selection and validation.
* `--scope=roads|crossroads|all` flag for `extract`, `generate`, and `patch`.
* `extract --portable` flag to export a clean, ID-free config.
* `patch --all-crossroads` flag (default patches only defaults,
  one per road type).

[0.1.1]: https://github.com/WoozyMasta/tv4p-road-tool/compare/v0.1.0...v0.1.1

## [0.1.0][] - 2026-01-30

### Added

* First public release

[0.1.0]: https://github.com/WoozyMasta/tv4p-road-tool/tree/v0.1.0

<!--links-->
[Keep a Changelog]: https://keepachangelog.com/en/1.1.0/
[Semantic Versioning]: https://semver.org/spec/v2.0.0.html

# Changelog

## [Unreleased]

### Features

### Fixes
* fix: getmetrics test due to this is environment-dependent test ([#80](https://github.com/line/wasmvm/pull/80))

### Changes
* chore: remove the copied interface from tm-db ([#82](https://github.com/line/wasmvm/pull/82))
* refactor: Revert using line/tm-db ([#77](https://github.com/line/wasmvm/pull/77))
* ci: add release job ([#71](https://github.com/line/wasmvm/pull/71))
* chore: Revert linux_static ([#70](https://github.com/line/wasmvm/pull/70))

## [v1.0.0-0.10.0] - 2022-06-21

### Features

* merge upstream 1.0.0 ([#64](https://github.com/line/wasmvm/issues/64))

### Fixes

* improve CHANGELOG's template and devtools/update_changlog.sh ([#60](https://github.com/line/wasmvm/pull/60))

## [v0.16.3-0.9.0] - 2022-03-03

### Changes


### Features

* merge upstream 0.16.3 ([#54](https://github.com/line/wasmvm/issues/54))

### Fixes

* fix Cargo.toml path in devtools/set_version.sh (part of [#59](https://github.com/line/wasmvm/issues/59))

## [v0.14.0-0.8.0] - 2021-10-01

### Features

* change tag name for static build ([#49](https://github.com/line/wasmvm/issues/49))


## [v0.14.0-0.7.0] - 2021-09-30

### Features

* add static linking of wasmvm ([#46](https://github.com/line/wasmvm/issues/46))


## [v0.14.0-0.6.1] - 2021-07-15

### Fixes

* rebuild shared libs to resolve compile error ([#44](https://github.com/line/wasmvm/issues/44))


## [v0.14.0-0.6.0] - 2021-07-12

### Changes
* update upstream Cosmwasm/wasmvm version to 0.14.0 (#36)
  - Please refer [CHANGELOG_OF_WASMVM_v0.14.0](https://github.com/CosmWasm/wasmvm/blob/v0.14.0/CHANGELOG.md)
* change the depended CosmWasm/cosmwasm to line/cosmwasm


## [v0.14.0-0.5.0] - 2021-05-12

### Changes

* Change the module uri from github.com/CosmWasm/wasmvm to github.com/link/wasmvm ([#23](https://github.com/line/wasmvm/issues/23))


## [v0.14.0-0.4.0] - 2021-05-03

### Changes

* change cargo use to tag from the version ([#17](https://github.com/line/wasmvm/issues/17))

### Code Refactoring

* add build tag 'mocks' ([#16](https://github.com/line/wasmvm/issues/16))
* define own iterator interface spec ([#15](https://github.com/line/wasmvm/issues/15))

  **BREAKING CHANGE**

  The implementation of KVStore now must return a newly defined iterator rather than the `tm-db` defines.


## [v0.14.0-0.3.0] - 2021-04-08

### Changes
* Update upstream Cosmwasm/wasmvm version to 0.14.0-beta1 (#8)
  - Please refer [CHANGELOG_OF_WASMVM_v0.14.0-beta1](https://github.com/CosmWasm/wasmvm/blob/v0.14.0-beta1/CHANGELOG.md)
* Update the depended line/cosmwasm version to 0.14.0-0.3.0 (#8)
* Adjust semantic PR validation rule (#9)


## [v0.12.0-0.1.0] - 2021-02-15

### Add
* Add semantic.yml for semantic pull request (#6)
* Add CHANGELOG-LINK.md (#3)

### Changes
* Change the depended CosmWasm/cosmwasm to line/cosmwasm (#3)


## [wasmvm v0.12.0] - 2021-02-04
Initial code is based on the wasmvm v0.12.0, cosmwasm v0.12.0

* (wasmvm) [v0.12.0](https://github.com/CosmWasm/wasmvm/releases/tag/v0.12.0).
* (cosmwasm) [v0.12.0](https://github.com/CosmWasm/cosmwasm/releases/tag/v0.12.0).

Please refer [CHANGELOG_OF_WASMVM_v0.12.0](https://github.com/CosmWasm/wasmvm/releases?after=v0.13.0)

[Unreleased]:https://github.com/line/wasmvm/compare/v1.0.0-0.10.0...HEAD
[v1.0.0-0.10.0]:https://github.com/line/wasmvm/compare/v0.16.3-0.9.0...v1.0.0-0.10.0
[v0.16.3-0.9.0]:https://github.com/line/wasmvm/compare/v0.14.0-0.8.0...v0.16.3-0.9.0
[v0.14.0-0.8.0]:https://github.com/line/wasmvm/compare/v0.14.0-0.7.0...v0.14.0-0.8.0
[v0.14.0-0.7.0]:https://github.com/line/wasmvm/compare/v0.14.0-0.6.1...v0.14.0-0.7.0
[v0.14.0-0.6.1]:https://github.com/line/wasmvm/compare/v0.14.0-0.6.0...v0.14.0-0.6.1
[v0.14.0-0.6.0]:https://github.com/line/wasmvm/compare/v0.14.0-0.5.0...v0.14.0-0.6.0
[v0.14.0-0.5.0]:https://github.com/line/wasmvm/compare/v0.14.0-0.4.0...v0.14.0-0.5.0
[v0.14.0-0.4.0]:https://github.com/line/wasmvm/compare/v0.14.0-0.3.0...v0.14.0-0.4.0
[v0.14.0-0.3.0]:https://github.com/line/wasmvm/compare/v0.12.0-0.1.0...v0.14.0-0.3.0
[v0.12.0-0.1.0]:https://github.com/line/wasmvm/compare/v0.12.0...v0.12.0-0.1.0

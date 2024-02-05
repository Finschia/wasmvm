# Changelog

## [Unreleased](https://github.com/Finschia/wasmvm/compare/v1.1.1-0.11.6...HEAD)

### Features
* bump up wasmvm from v1.1.1 to v1.4.1 ([#136](https://github.com/Finschia/wasmvm/pull/136))
* bump up wasmvm from v1.1.1 to v1.5.0 ([#138](https://github.com/Finschia/wasmvm/pull/138))

## [[1.1.1-0.11.6](https://github.com/Finschia/wasmvm/compare/v1.1.1+0.11.5...v1.1.1-0.11.6)] - 2023-10-18
### Changes
* revert: use pre-release versioning and set version to 1.1.1-0.11.6 ([#130](https://github.com/Finschia/wasmvm/pull/130))

## [[1.1.1+0.11.5](https://github.com/Finschia/wasmvm/compare/v1.1.1-0.11.4-rc1...v1.1.1+0.11.5)] - 2023-10-5
### Changes
* set version to 1.1.1+0.11.5 for applying v1.1.9+0.8.1 ([#128](https://github.com/Finschia/wasmvm/pull/128))
* add an automatic build shared library c ([#119](https://github.com/Finschia/wasmvm/pull/119))

## Fixes
* hotfix automatic build shared library ci ([#127](https://github.com/Finschia/wasmvm/pull/127))

## [[1.1.1-0.11.4-rc1](https://github.com/Finschia/wasmvm/compare/v1.1.1-0.11.3-rc1...v1.1.1-0.11.4-rc1)] - 2023-08-25
### Changes
* bumpup version to 1.1.1-0.11.4-rc1 ([#126](https://github.com/Finschia/wasmvm/pull/125))

## [[v1.1.1-0.11.3-rc1](https://github.com/Finschia/wasmvm/compare/v1.1.1-0.11.2...v1.1.1-0.11.3-rc1)] - 2023-08-24

### Changes

* enable ci recognize -rcX version ([#125](https://github.com/Finschia/wasmvm/pull/125))
* version bump to 1.1.1-0.11.3-rc1 ([#122](https://github.com/Finschia/wasmvm/pull/122))
* update golang version to 1.20 ([#118](https://github.com/Finschia/wasmvm/pull/118))
* replace line modules with Finschia's ([#109](https://github.com/Finschia/wasmvm/pull/109))
### Features


* add codeowners file ([#100](https://github.com/Finschia/wasmvm/pull/100))
### Fixes


* fix a test for rustc 1.68 or later ([#108](https://github.com/Finschia/wasmvm/pull/108))
* wrong tag reference (v1.1.1-0.11.2) ([#95](https://github.com/Finschia/wasmvm/pull/95))

## [[v1.1.1-0.11.2](https://github.com/Finschia/wasmvm/compare/v1.1.1-0.11.1...v1.1.1-0.11.2)] - 2023-03-13

The functional changes of this version same with v1.1.1-0.11.1, The only difference is that fix the import problem other service (like wasmd and finshia), because I think it seems to be problem to change the v1.1.1-0.11.1 tag commit.

### Fixes
* fix: wrong tag reference (v1.1.1-0.11.2) ([#95](https://github.com/Finschia/wasmvm/pull/95))

## [[v1.1.1-0.11.1](https://github.com/Finschia/wasmvm/compare/v1.1.1-0.11.0...v1.1.1-0.11.1)] - 2023-01-13

### Fixes
* add .so / .dylib file and modify Makefile ([#85](https://github.com/Finschia/wasmvm/pull/85))

## [[v1.1.1-0.11.0](https://github.com/Finschia/wasmvm/compare/v1.0.0-0.10.0...v1.1.1-0.11.0)] - 2023-01-11

### Features
* merge upstream 1.1.1 ([#84](https://github.com/Finschia/wasmvm/pull/84))

### Fixes
* fix: getmetrics test due to this is environment-dependent test ([#80](https://github.com/Finschia/wasmvm/pull/80))

### Changes
* chore: remove the copied interface from tm-db ([#82](https://github.com/Finschia/wasmvm/pull/82))
* refactor: Revert using line/tm-db ([#77](https://github.com/Finschia/wasmvm/pull/77))
* ci: add release job ([#71](https://github.com/Finschia/wasmvm/pull/71))
* chore: Revert linux_static ([#70](https://github.com/Finschia/wasmvm/pull/70))

## [v1.0.0-0.10.0] - 2022-06-21

### Features

* merge upstream 1.0.0 ([#64](https://github.com/Finschia/wasmvm/issues/64))

### Fixes

* improve CHANGELOG's template and devtools/update_changlog.sh ([#60](https://github.com/Finschia/wasmvm/pull/60))

## [v0.16.3-0.9.0] - 2022-03-03

### Changes


### Features

* merge upstream 0.16.3 ([#54](https://github.com/Finschia/wasmvm/issues/54))

### Fixes

* fix Cargo.toml path in devtools/set_version.sh (part of [#59](https://github.com/Finschia/wasmvm/issues/59))

## [v0.14.0-0.8.0] - 2021-10-01

### Features

* change tag name for static build ([#49](https://github.com/Finschia/wasmvm/issues/49))


## [v0.14.0-0.7.0] - 2021-09-30

### Features

* add static linking of wasmvm ([#46](https://github.com/Finschia/wasmvm/issues/46))


## [v0.14.0-0.6.1] - 2021-07-15

### Fixes

* rebuild shared libs to resolve compile error ([#44](https://github.com/Finschia/wasmvm/issues/44))


## [v0.14.0-0.6.0] - 2021-07-12

### Changes
* update upstream Cosmwasm/wasmvm version to 0.14.0 (#36)
  - Please refer [CHANGELOG_OF_WASMVM_v0.14.0](https://github.com/CosmWasm/wasmvm/blob/v0.14.0/CHANGELOG.md)
* change the depended CosmWasm/cosmwasm to line/cosmwasm


## [v0.14.0-0.5.0] - 2021-05-12

### Changes

* Change the module uri from github.com/CosmWasm/wasmvm to github.com/link/wasmvm ([#23](https://github.com/Finschia/wasmvm/issues/23))


## [v0.14.0-0.4.0] - 2021-05-03

### Changes

* change cargo use to tag from the version ([#17](https://github.com/Finschia/wasmvm/issues/17))

### Code Refactoring

* add build tag 'mocks' ([#16](https://github.com/Finschia/wasmvm/issues/16))
* define own iterator interface spec ([#15](https://github.com/Finschia/wasmvm/issues/15))

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

[Unreleased]:https://github.com/Finschia/wasmvm/compare/v1.0.0-0.10.0...HEAD
[v1.0.0-0.10.0]:https://github.com/Finschia/wasmvm/compare/v0.16.3-0.9.0...v1.0.0-0.10.0
[v0.16.3-0.9.0]:https://github.com/Finschia/wasmvm/compare/v0.14.0-0.8.0...v0.16.3-0.9.0
[v0.14.0-0.8.0]:https://github.com/Finschia/wasmvm/compare/v0.14.0-0.7.0...v0.14.0-0.8.0
[v0.14.0-0.7.0]:https://github.com/Finschia/wasmvm/compare/v0.14.0-0.6.1...v0.14.0-0.7.0
[v0.14.0-0.6.1]:https://github.com/Finschia/wasmvm/compare/v0.14.0-0.6.0...v0.14.0-0.6.1
[v0.14.0-0.6.0]:https://github.com/Finschia/wasmvm/compare/v0.14.0-0.5.0...v0.14.0-0.6.0
[v0.14.0-0.5.0]:https://github.com/Finschia/wasmvm/compare/v0.14.0-0.4.0...v0.14.0-0.5.0
[v0.14.0-0.4.0]:https://github.com/Finschia/wasmvm/compare/v0.14.0-0.3.0...v0.14.0-0.4.0
[v0.14.0-0.3.0]:https://github.com/Finschia/wasmvm/compare/v0.12.0-0.1.0...v0.14.0-0.3.0
[v0.12.0-0.1.0]:https://github.com/Finschia/wasmvm/compare/v0.12.0...v0.12.0-0.1.0

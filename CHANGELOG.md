
# CHANGELOG

All notable changes to this project will be documented in this file. The format
is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## UNRELEASED


### Fixed

- resolved a panic with failed account login attempts

### Changed

- updated indigo SDK dependency
- consistent use of HTTP proxy env vars (via indigo SDK change)

## [0.2.1] - 2025-12-05

### Added

- ability to specify version string with ldflags ("main.Version=...")

### Changed

- update indigo dependency

## [0.2.0] - 2025-11-30

### Added

- more commands for lexicon management (`goat lex ...`): `lint`, `new`, `check-dns`, `pull`, `breaking`, etc. there were merged from the `glot` command

### Changed

- require Go v1.25
- update indigo dependency
- finish migration from `indigo:xrpc` to `indigo:atproto/atclient` for all HTTP API requests

## [0.1.2] - 2025-10-21

### Changed

- updated `indigo` dependency, for permission-set lexicon parsing and package renames
- start using `indigo:atproto/client` package for some API requests
- update `account` subcommands: `status` renamed to `check-auth`, and `lookup` renamed to `status`
- `pds admin` commands

## [0.1.1] - 2025-08-12

### Changed

- updated to `urfav/cli/v3`, which (finally) makes CLI argument ordering more flexible
- tweaked release build process

## [0.1.0] - 2025-08-12

First tagged release of `goat`.

## [init] - 2025-08-12

Forked this repo from [indigo](https://github.com/bluesky-social/indigo).

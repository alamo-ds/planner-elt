# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

- Integrated the project `github.com/alamo-ds/dag`
- `alamo-ds/msgraph` version
- added some unit tests
- added an integration test

## [v0.1.7]

### Additions

- feat: added terraform config
- Task worker initiates with a call to MS Graph's User endpoint
  - In the future, this will become its own job
- Task.AddUsers() method to map MS Graph user details
- SnapshotDateTime field to Task (captures the current timestamp in UTC)
- Added Content-Type header to blob requests

### Changes

- authentication switching from client secret to managed identity
- storage container ref
- when run fails, will exit with code 1
- Updated dependencies


## [v0.1.6]

- moved logging to `log/slog`

## [v0.1.5]

- Initial release

[Unreleased]: https://github.com/alamo-ds/planner-elt/compare/v0.1.7...HEAD
[v0.1.5]: https://github.com/alamo-ds/planner-elt/releases/tag/v0.1.5
[v0.1.6]: https://github.com/alamo-ds/planner-elt/compare/v0.1.5...v0.1.6
[v0.1.7]: https://github.com/alamo-ds/planner-elt/compare/v0.1.6...v0.1.7

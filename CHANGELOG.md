# Changelog

## v1.0.0 - 2026-10-15

### Fixed

- Chrome no longer leaks its profile, disk cache and temp files on every render. Each render runs in its own temp
  directory (`--user-data-dir`, `TMPDIR`, `XDG_CACHE_HOME`, `XDG_CONFIG_HOME`) that is removed afterwards, also on
  timeout. The leak filled the production disk at about 1.5 GB per day.
- On timeout the whole Chrome process group is killed, not only the main process.
- `tini` runs as PID 1 in the Docker image and reaps the zombie Chrome helper processes left after each render.

### Changed

- Require Go 1.24.

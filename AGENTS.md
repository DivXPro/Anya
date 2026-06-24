# Repository Guidelines

## Project Structure & Module Organization

This repository contains Elf planning documentation.

- `docs/superpowers/specs/`: product and architecture specs.
- `docs/superpowers/plans/`: executable implementation plans and phase checklists.
- Planned paths: `desktop/` for Wails and `firmware/` for ESP32/StickC.

Keep design decisions in specs and delivery steps in plans. Mirror the documented module layout when adding code.

## Build, Test, and Development Commands

No root build system exists yet. Use module commands once paths exist:

- `cd desktop && wails build`: build the desktop app.
- `cd desktop && go test ./... -count=1`: run Go tests.
- `cd firmware && pio run`: compile firmware with PlatformIO.
- `sqlite3 ~/.elf/elf.db "SELECT count(*) FROM messages"`: inspect smoke-test data.

If you add a package manager or Makefile, document root commands here.

## Implementation Principles

When writing plans or code, prefer simple, proven solutions. Use standard libraries or mature packages before creating custom frameworks, protocols, schedulers, parsers, or storage layers. Add abstractions only when they remove repeated complexity.

## Coding Style & Naming Conventions

Use concise, conventional names matching the plan:

- Go packages under `desktop/internal/...`; test files end with `_test.go`.
- React TypeScript files use `.tsx` for components and `.ts` for utilities.
- Firmware files use lower_snake_case, for example `wifi_portal.cpp` and `mdns_client.cpp`.
- Markdown docs should use dated filenames: `YYYY-MM-DD-topic.md`.

Run `gofmt` on Go code and the project formatter for frontend code once configured.

## Testing Guidelines

Derive test cases from design scenarios and observable user/system behavior, not internal execution paths. Favor fast, deterministic tests close to the change; use integration tests when behavior crosses modules. Keep hardware checks separate unless they run reliably in CI or documented local setup.

Name tests after expected behavior, for example `TestDeviceReconnectsAfterDesktopIPChanges`. Do not make tests pass by weakening assertions, hard-coding internals, skipping validation, or mocking away the behavior under test. Before marking work complete, run relevant automated tests and document manual checks for UI, audio, WiFi, or hardware flows.

## Commit & Pull Request Guidelines

Git history is not available in this checkout. Use Conventional Commit-style messages, such as `feat: add device authorization flow` or `test: add gateway protocol codec tests`.

Pull requests should include a summary, modules, test results, and screenshots or hardware notes for UI, pairing, WiFi, or firmware changes.

## Security & Configuration Tips

Do not commit WiFi credentials, pairing tokens, desktop IDs, local databases, or build artifacts. Keep runtime state under `~/.elf/`.

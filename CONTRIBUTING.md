# Contributing to SpecForge

Thank you for your interest in contributing to **SpecForge**! We welcome bug reports, documentation improvements, new quality gate adapters, and pull requests.

## How to Contribute

### 1. Reporting Bugs
- Search existing issues to ensure the bug hasn't been reported.
- Open a new issue with detailed reproduction steps, environment details (OS, Go version, Agent), and the command output.

### 2. Suggesting Features
- Open a feature request issue describing the use case and expected behavior.

### 3. Pull Request Process
1. Fork the repository and create your branch from `main`:
   ```bash
   git checkout -b feature/my-new-feature
   ```
2. Ensure Clean Architecture principles are respected:
   - Pure business logic belongs in `internal/domain/`.
   - External tools, CLI adapters, and filesystem belong in `internal/adapters/`.
   - Interfaces belong in `internal/ports/`.
3. Add automated unit tests for your changes.
4. Run the test suite:
   ```bash
   go test ./...
   ```
5. Commit your changes following Conventional Commits (`feat:`, `fix:`, `docs:`, `refactor:`).
6. Push to your fork and submit a Pull Request.

## Contributor Licensing

In accordance with Section 5 of the Apache License, Version 2.0 ("Submission of Contributions"), unless you explicitly state otherwise, any Contribution intentionally submitted for inclusion in this project shall be licensed under the **Apache License, Version 2.0**, without any additional terms or conditions.

Contributors retain copyright ownership of their respective contributions, granting the project and its users a perpetual, worldwide, non-exclusive, no-charge, royalty-free, irrevocable license in accordance with the Apache License 2.0 terms.

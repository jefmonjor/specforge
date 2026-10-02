# 🚀 SpecForge: Resilient Spec-Driven & TDD Assembly Line for AI-Assisted Engineering

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)
[![TDD](https://img.shields.io/badge/TDD-Red--Green--Refactor-brightgreen.svg)]()
[![E2E](https://img.shields.io/badge/E2E-TesterArmy%20chromedp-orange.svg)]()
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20macOS%20(Apple%20Silicon%20%26%20Intel)%20%7C%20Linux-lightgrey.svg)]()
[![Zero Dependencies](https://img.shields.io/badge/Dependencies-Zero%20Runtime%20Deps-success)]()

> **Transform AI coding from hallucinated scripts into a deterministic, resilient software assembly line.**  
> SpecForge is a standalone CLI tool written in pure Go (~8.9 MB) that orchestrates **Spec-Driven Development (SDD)**, **Test-Driven Development (TDD)**, and **Autonomous Visual E2E Testing (TesterArmy Engine)** with automated quality guardrails, fault tolerance, and zero configuration overhead.
>
> 📖 **Comprehensive Documentation:** Read the full [SpecForge User & Architecture Guide](USER_GUIDE.md) for detailed tutorials and troubleshooting.

---

## ⚡ The Philosophy: Why SpecForge?

Direct AI coding assistants often produce fragile, hallucinated code that passes tests through tautologies or introduces duplicate logic, dead code, and security blind spots. 

**SpecForge enforces software craftsmanship by treating the AI as an untrusted junior programmer supervised by automated gates:**

```plaintext
      ┌─────────────────────────────────────────────────────────────┐
      │  1. SPECIFICATION (BDD & Gherkin)                           │
      │  - Socratic dialogue defines domain invariants              │
      │  - Sealed with cryptographic SHA-256 (tamper-evident)       │
      └──────────────────────────────┬──────────────────────────────┘
                                     │
                                     ▼
      ┌─────────────────────────────────────────────────────────────┐
      │  2. RED PHASE: Test Stubs First                             │
      │  - AI generates failing unit tests based on Gherkin         │
      │  - Tests executed: if they pass without code -> ABORT YAGNI │
      └──────────────────────────────┬──────────────────────────────┘
                                     │
                                     ▼
      ┌─────────────────────────────────────────────────────────────┐
      │  3. GREEN PHASE: Minimal Implementation                     │
      │  - AI writes minimal code to make tests turn green (KISS)   │
      │  - Self-healing retries with compiler/test feedback (max 3) │
      └──────────────────────────────┬──────────────────────────────┘
                                     │
                                     ▼
      ┌─────────────────────────────────────────────────────────────┐
      │  4. REFACTOR PHASE: Quality Guardrails Gate                 │
      │  - DRY Check (jscpd: 0% duplicate code tolerance)           │
      │  - Dead Code Detection (Knip: zero orphan dependencies)     │
      │  - Mutation Testing (Stryker: tests must kill mutants)      │
      │  - Strict Linters (ESLint, golangci-lint, Ruff)             │
      │  - Clean Architecture Check (Hexagonal/ArchUnit)            │
      └──────────────────────────────┬──────────────────────────────┘
                                     │
                                     ▼
      ┌─────────────────────────────────────────────────────────────┐
      │  5. SECURITY AUDIT (Cloudflare Adversarial Harness)         │
      │  - Reconnaissance -> Red Team Hunter -> Blue Team Verifier  │
      │  - Incremental git diff scanning (--diff) for PRs           │
      └──────────────────────────────┬──────────────────────────────┘
                                     │
                                     ▼
      ┌─────────────────────────────────────────────────────────────┐
      │  6. AUTONOMOUS VISUAL E2E (TesterArmy Engine with chromedp) │
      │  - Native Chrome/Edge navigation without Node.js            │
      │  - Real-time DOM snapshot & deterministic CSS selectors     │
      │  - AI actions: click, type, wait, assert + failure screens  │
      └─────────────────────────────────────────────────────────────┘
```

---

## ✨ Key Capabilities

### 1. Zero-Dependencies & Zero-Keys Onboarding
* **Pure Static Binary:** Compiles into a single ~8.9 MB executable. You don't need Go, Node.js, Python, Puppeteer, or Playwright installed to run `specforge`.
* **Zero Secrets in Code:** Automatically inherits Google Cloud Application Default Credentials (ADC) from `gcloud auth application-default login` or local environment variables.
* **Automatic PATH Integration:** When executed, `specforge init` automatically registers itself into your Windows User PATH (or macOS/Linux `~/.zshrc`) **without requiring administrator or UAC elevation**.
* **Flexible CLI Invocations:** The binary responds to three command names: `specforge`, `forge`, or `sdd`.

### 2. Autonomous Visual E2E Testing (`specforge e2e` — TesterArmy Engine)
* **Native Go Browser Automation:** Built on `chromedp` (Chrome DevTools Protocol). Communicates directly with local Google Chrome or Microsoft Edge.
* **Simplified UI Tree Extraction:** Executes in-page JavaScript to extract visible interactive elements (buttons, inputs, links, alerts) and assigns unique, deterministic CSS selectors (`data-sdd-id`), ignoring useless layout divs.
* **Typed Action Loop:** AI reasoning produces structured `VisualAction` decisions (`click`, `type`, `assert`, `wait`). If the scenario fails or times out, it automatically captures a full screenshot in `docs/e2e/screenshot.png` and exits with code 1.

### 3. Multi-Format Document Ingestion with Microsoft MarkItDown (`specforge doc`)
* Drop architecture PDFs, Word documents (`.docx`), or Excel sheets (`.xlsx` with complex formulas and tables) into `docs/`.
* `specforge ingest` and `specforge doc` detect them and convert them **automatically to high-fidelity Markdown**, extracting formulas and tabular data so the AI can reason over your business logic.

### 4. Legacy Reverse-Engineering & Migration Ground Truth (`--from-repo`)
* Modernizing a legacy codebase (COBOL, Java 6, PHP, Delphi) with zero documentation?
* Point `specforge interview --from-repo /path/to/legacy-repo` to reverse-engineer the old codebase: the AI extracts pure business rules (the **WHAT**) and discards obsolete technology (the **HOW**), generating sealed BDD Gherkin specs in your new repository.

### 5. Resilient TDD Assembly Line with Checkpoints (`--resume`)
* Atomic state machine persisted in `.sdd-state.json`.
* If your VPN drops, API quota runs out, or laptop shuts down: simply run `specforge loop --resume` to continue at the exact micro-step without burning duplicate tokens.

### 6. Adversarial Security Audit (`specforge audit`)
* Multi-phase adversarial pipeline inspired by Cloudflare's security evaluation harness.
* Simulates attacker reconnaissance, red team vulnerability hunting, and blue team false-positive verification.
* Run incremental scans on your pull request diff: `specforge audit --diff`.

### 7. Greenfield & Brownfield Ready
* **New Projects (Greenfield):** `specforge setup --stack react` (or `java`, `go`, `python`) initializes a ready-to-compile project with testing frameworks, quality configs, and GitOps values.
* **Existing Projects (Brownfield):** `specforge setup` detects your technology and **preserves 100% of your existing source code and build files**, only adding SDD standards and optional quality configs.

---

## 📦 Installation & Download

### Option A: Download Precompiled Binaries
Download the binary for your platform from [Releases](../../releases):
* **Windows x64:** `dist/specforge-windows-amd64.exe` (rename to `specforge.exe` or `sdd.exe`)
* **macOS Apple Silicon (M1/M2/M3/M4):** `dist/specforge-darwin-arm64`
* **macOS Intel:** `dist/specforge-darwin-amd64`
* **Linux x64:** `dist/specforge-linux-amd64`

> **macOS Note (Gatekeeper):** If macOS warns about an unidentified developer, run:
> ```bash
> chmod +x specforge-darwin-arm64
> xattr -d com.apple.quarantine specforge-darwin-arm64
> sudo mv specforge-darwin-arm64 /usr/local/bin/specforge
> ```

### Option B: Build from Source (Requires Go 1.24+)
```bash
git clone https://github.com/jefmonjor/specforge.git
cd specforge

# Build for current machine:
go build -ldflags="-s -w" -trimpath -o specforge.exe .

# Or cross-compile for all platforms (Windows, Mac ARM64/Intel, Linux):
# On Windows:
powershell -ExecutionPolicy Bypass -File .\build-all.ps1
# On Linux / macOS:
chmod +x ./build-all.sh && ./build-all.sh
```

---

## 🚀 Quickstart Guide (5 Minutes)

### Step 1: Workstation Initialization (Run Once)
```bash
specforge init
```
*Detects your credentials, configures `~/.specforge/config.json`, and adds `specforge` to your user PATH.*

### Step 2: Project Setup
Inside your project directory:
```bash
git checkout -b feature/user-authentication

# For an existing project (preserves all code):
specforge setup

# Or for a brand new project archetype:
specforge setup --stack react    # Options: react, java, go, python
```

### Step 3: Ingest Architecture & Documents
```bash
specforge ingest
```
*Scans your project in ~20 ms, automatically converts any PDF/Excel files to Markdown, and generates `docs/_context/context-pack.md`.*

### Step 4: Socratic BDD Interview
```bash
specforge interview --feature "OAuth2 login with biometric fallback"
```
*The AI architect interviews you interactively and seals the resulting Gherkin spec with a cryptographic hash in `specs/0001-oauth2-login.md`.*

### Step 5: Execute the TDD Assembly Line
```bash
specforge loop
```
*Watches the Red -> Green -> Refactor cycle execute. If interrupted, run `specforge loop --resume`.*

### Step 6: Adversarial Security Audit
```bash
specforge audit --diff
```
*Audits only the modified lines in your branch before opening a Pull Request.*

### Step 7: Autonomous Visual E2E Test
```bash
specforge e2e --url http://localhost:3000
```
*Navigates the live web application using `chromedp` and verifies that the BDD scenario visually holds true.*

---

## 📋 Command Reference

| Command | Description | Key Flags |
| :--- | :--- | :--- |
| `specforge init` | Zero-Config initialization and ADC credential detection | `--agent`, `--project`, `--non-interactive`, `--force` |
| `specforge setup` | Deploys baseline, GitOps values, and scaffolding | `--stack [react/java/go/python]`, `--force` |
| `specforge ingest` | Ultra-fast project scanner and auto-document converter | `--output`, `--sync`, `--no-convert`, `--print` |
| `specforge doc` | Converts PDF, Word, and Excel files to Markdown via MarkItDown | `--recursive`, `--output` |
| `specforge interview` | Socratic BDD interview and SHA-256 specification sealing | `--feature`, `--from-repo`, `--output`, `--agent` |
| `specforge loop` | Resilient TDD assembly line (Red -> Green -> Refactor) | `--resume`, `--spec`, `--agent` |
| `specforge e2e` | Autonomous Visual E2E test engine (TesterArmy style) with chromedp | `--url`, `--spec`, `--headless`, `--max-steps`, `--screenshot` |
| `specforge audit` | Adversarial security audit (Cloudflare 6-phase harness) | `--diff`, `--full`, `--fail-on [HIGH/MEDIUM]`, `--agent` |
| `specforge consistency` | Deterministic architectural consistency verification | `--FailOn` |
| `specforge build` | Project compilation validation | `--dry-run` |
| `specforge version` | Displays binary telemetry, Go version, and platform info | — |

---

## 🛡️ Built-in Quality Guardrails

| Guardrail | Tool | Frontend (React/TS) | Java / Spring | Go | Python |
| :--- | :--- | :---: | :---: | :---: | :---: |
| **Strict Linter** | ESLint / golangci-lint / Ruff | ✅ | ✅ | ✅ | ✅ |
| **DRY Enforcer** | `jscpd` (threshold: 0%) | ✅ | ✅ | ✅ | ✅ |
| **Dead Code Detection** | `knip` | ✅ | — | — | — |
| **Mutation Testing** | `stryker` (threshold: >= 80%) | ✅ | — | — | — |
| **Clean Architecture** | `ArchUnit` | — | ✅ | — | — |
| **GitOps Values** | Tekton / OpenShift values template | ✅ | ✅ | ✅ | ✅ |
| **Health & Info Probes** | Root `/health` (HTTP 200) & `/info` | ✅ | ✅ | ✅ | ✅ |

---

## ⚖️ Open Source & Apache License 2.0 Compliance

SpecForge is licensed under the **[Apache License, Version 2.0](LICENSE)**.

### What Apache 2.0 Grants to You:
* **Commercial Use:** You are free to use SpecForge within commercial and proprietary products without paying royalties.
* **Modification & Sublicensing:** You may modify the source code and distribute derivative works under terms of your choice (including proprietary licenses), provided you satisfy Section 4 conditions.
* **Patent Grant (Section 3):** Contributors grant a perpetual, worldwide, non-exclusive patent license covering their contributions.
* **Defensive Termination (Section 3):** If a party initiates patent litigation alleging that SpecForge infringes their patents, any patent license granted to that party under Apache 2.0 terminates automatically.

### Requirements When Distributing SpecForge or Derivative Works:
1. **Include LICENSE (Section 4(a)):** You must provide recipients with a full copy of the Apache License 2.0.
2. **State Modifications (Section 4(b)):** You must cause any modified files to carry prominent notices stating that you changed the files.
3. **Retain Notices (Section 4(c)):** You must retain all copyright, patent, and trademark notices in the source code.
4. **Distribute NOTICE (Section 4(d)):** You must include a readable copy of the [NOTICE](NOTICE) file in distributions.
5. **Third-Party Attribution:** When distributing binaries, you must include the third-party acknowledgments and disclaimers detailed in [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md) (especially for BSD-3-Clause dependencies like `oauth2` and `pflag`).

### Trademarks (Section 6):
The Apache License 2.0 **does not** grant trademark rights to use the trade names, trademarks, service marks, or product names of the project, except as required for reasonable and customary use in describing the origin of the Work.

---

## 🤝 Contributing

Contributions are warmly welcome! Please review:
* [CONTRIBUTING.md](CONTRIBUTING.md) — Under **Section 5 of the Apache License 2.0**, all contributions submitted intentionally to the project are licensed under Apache 2.0 without additional terms. Contributors retain copyright over their patches.
* [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) — Community standards and enforcement guidelines.
* [SECURITY.md](SECURITY.md) — Responsible vulnerability disclosure instructions.
* [LEGAL_AUDIT.md](LEGAL_AUDIT.md) — Comprehensive dependency and license compatibility matrix.

---

## 📄 License Notice

```text
Copyright 2026 SpecForge Contributors | jefmonjor.dev

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at:

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```

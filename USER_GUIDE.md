# 📖 SpecForge: Complete User & Architecture Guide

> **Engine:** Pure Static Go (`specforge.exe` / `specforge`, ~8.9 MB)  
> **Methodology:** Spec-Driven Development (SDD) & Resilient Test-Driven Development (TDD)  
> **License:** Apache License, Version 2.0  

---

## Table of Contents
1. [Architecture Overview & Philosophy](#1-architecture-overview--philosophy)
2. [Installation & Cross-Platform Binaries](#2-installation--cross-platform-binaries)
3. [Workstation Setup (`specforge init`)](#3-workstation-setup-specforge-init)
4. [Project Setup (`specforge setup`) — Greenfield vs. Brownfield](#4-project-setup-specforge-setup--greenfield-vs-brownfield)
5. [The Contextual Memory Brain (6-File Explicit Context & Auto-Learning)](#5-the-contextual-memory-brain-6-file-explicit-context--auto-learning)
6. [Document Ingestion with Microsoft MarkItDown (`specforge doc` & `ingest`)](#6-document-ingestion-with-microsoft-markitdown-specforge-doc--ingest)
7. [Socratic BDD Interview & Legacy Migration (`specforge interview` & `--from-repo`)](#7-socratic-bdd-interview--legacy-migration-specforge-interview---from-repo)
8. [The `[NEEDS CLARIFICATION]` Business Circuit Breaker](#8-the-needs-clarification-business-circuit-breaker)
9. [The Resilient TDD Assembly Line (`specforge loop` & `--resume`)](#9-the-resilient-tdd-assembly-line-specforge-loop---resume)
10. [Autonomous Visual E2E Testing (`specforge e2e` — TesterArmy Engine)](#10-autonomous-visual-e2e-testing-specforge-e2e--testerarmy-engine)
11. [Adversarial Security Audit (`specforge audit`)](#11-adversarial-security-audit-specforge-audit)
12. [Quality Guardrails Matrix](#12-quality-guardrails-matrix)
13. [Complete Command Reference](#13-complete-command-reference)
14. [Open Source & Apache 2.0 Compliance](#14-open-source--apache-20-compliance)

---

## 1. Architecture Overview & Philosophy

SpecForge enforces disciplined software craftsmanship on top of generative AI coding assistants. Instead of accepting unverified code generation, SpecForge acts as an autonomous engineering supervisor:

```plaintext
      ┌─────────────────────────────────────────────────────────────┐
      │  SPECIFICATION (Gherkin BDD Sealed with SHA-256)            │
      └──────────────────────────────┬──────────────────────────────┘
                                     │
                                     ▼
      ┌─────────────────────────────────────────────────────────────┐
      │  RED PHASE: Test Stubs First (Fails without implementation)  │
      └──────────────────────────────┬──────────────────────────────┘
                                     │
                                     ▼
      ┌─────────────────────────────────────────────────────────────┐
      │  GREEN PHASE: Strict Minimal Implementation (KISS / YAGNI)   │
      └──────────────────────────────┬──────────────────────────────┘
                                     │
                                     ▼
      ┌─────────────────────────────────────────────────────────────┐
      │  REFACTOR PHASE: Quality Guardrails (jscpd, Knip, Stryker)  │
      └──────────────────────────────┬──────────────────────────────┘
                                     │
                                     ▼
      ┌─────────────────────────────────────────────────────────────┐
      │  E2E VALIDATION: Autonomous Visual Testing (chromedp)       │
      └─────────────────────────────────────────────────────────────┘
```

* **Zero Runtime Dependencies:** Single static binary in pure Go. Does not require Go, Node.js, Python, or Playwright installed to run the CLI or the E2E engine.
* **Zero Keys:** Automatically inherits Google Cloud Application Default Credentials (ADC) or local session auth.
* **Deterministic Verification:** Every transition is validated by compilers and linters with exit code enforcement.

---

## 2. Installation & Cross-Platform Binaries

### Download Precompiled Binaries
Download the binary for your platform from GitHub Releases into your path:
* **Windows x64:** `dist/specforge-windows-amd64.exe` (rename to `specforge.exe` or `sdd.exe`)
* **macOS Apple Silicon (M1/M2/M3/M4):** `dist/specforge-darwin-arm64`
* **macOS Intel:** `dist/specforge-darwin-amd64`
* **Linux x64:** `dist/specforge-linux-amd64`

> **macOS Gatekeeper:** Run `xattr -d com.apple.quarantine specforge-darwin-arm64` and `chmod +x specforge-darwin-arm64`.

### Building from Source
```bash
git clone https://github.com/jefmonjor/specforge.git
cd specforge

# Compile for your current OS:
go build -ldflags="-s -w" -trimpath -o specforge.exe .

# Or cross-compile all 4 platforms at once:
powershell -ExecutionPolicy Bypass -File .\build-all.ps1   # Windows
./build-all.sh                                             # Linux / macOS
```

---

## 3. Workstation Setup (`specforge init`)

Run once on a new developer workstation from any directory:

```bash
specforge init
```

* **Zero-Keys ADC Detection:** Automatically locates Google Cloud ADC (`gcloud auth application-default login`) and configures `~/.specforge/config.json`.
* **Zero-Admin PATH Injection:** Modifies Windows User Registry (`HKCU:\Environment\Path`) or appends to `~/.zshrc` without requiring administrator or UAC rights.
* **Non-Interactive Mode:** For automated scripts and CI/CD:
  ```bash
  specforge init --non-interactive --agent gemini --force
  ```

---

## 4. Project Setup (`specforge setup`) — Greenfield vs. Brownfield

Run from the root of your project:

### Scenario A: New Project (Greenfield)
```bash
mkdir my-new-service
cd my-new-service
git init

# Initialize with a complete archetype:
specforge setup --stack react    # React + Vite + TS + Vitest + Knip + Stryker
specforge setup --stack java     # Java 21 + Spring Boot 3 + Maven + ArchUnit
specforge setup --stack go       # Go + Clean Architecture
specforge setup --stack python   # Python + FastAPI + pytest + ruff
```
1. Deploys full source code and build files (`package.json`, `pom.xml`, or `go.mod`), replacing placeholders with your directory name.
2. Pre-configures quality guardrails (`knip.json`, `.jscpd.json`, `stryker.conf.json`).
3. Deploys GitOps values template (`values/dev-app-values.yaml`).
4. Extracts Contextual Memory Brain (`.sdd/agent/`).
5. Updates `.gitignore` to exclude `.specify/`, `.sdd-cache/`, `.sdd/`, and `values/`.

### Scenario B: Existing Project (Brownfield)
```bash
cd /path/to/existing-project
git checkout -b feature/my-new-task
specforge setup
```
1. **100% Code Preservation:** Detects your stack and does not overwrite or modify existing source or build files.
2. Injects `.specify/` standards and `.sdd/agent/` without overwriting existing team lessons.

---

## 5. The Contextual Memory Brain (6-File Explicit Context & Auto-Learning)

SpecForge transforms generic AI assistants into senior software crafters specialized in your architecture using 6 explicit memory files deployed in `.sdd/agent/`:

| Memory File | Role | Content |
| :--- | :--- | :--- |
| **`agente.md`** | **Core Constitution** | Strict TDD, KISS, YAGNI, Clean Architecture, Rich Domain, fail-fast. |
| **`persona.md`** | **Tone & Style** | Direct, concise, technical. Semantic English names. Comments for the "why". |
| **`ng-rules.md`** | **Never-Go Anti-Patterns** | No frameworks in domain, no `any`, no swallowed exceptions, 0% duplicate code, zero dead code. |
| **`glossary.md`** | **Ubiquitous Language** | Definitions of SDD, TDD, YAGNI, GitOps, Probes, and domain concepts. |
| **`references.md`** | **Golden Masters** | Reference implementations of Value Objects, Use Cases, and Health Probes. |
| **`lessons.md`** | **Dynamic Self-Learning** | Dynamic ledger of errors solved on the project. |

### Continuous Autonomous Learning:
* In phase **GREEN**: When a compiler or test error is fixed after a retry, SpecForge automatically records:
  ```text
  - [2026-10-02 10:15:00] [Compilación / Test] Fallo: TypeError en cálculo. Solución: Tipado estricto. No repetir.
  ```
* In phase **REFACTOR**: When a Quality Gate violation is resolved, the rule is recorded in `lessons.md`.
* **Immediate Feedback:** On all subsequent turns, `LoadAgentContext` injects `lessons.md` into `<system_instruction>`. The AI never makes the same architectural mistake twice.

---

## 6. Document Ingestion with Microsoft MarkItDown (`specforge doc` & `ingest`)

SpecForge integrates Microsoft MarkItDown to process heterogeneous documentation:

```bash
# Convert a single document (PDF, Word, or Excel spreadsheet):
specforge doc docs/financial-reconciliation.xlsx

# Or scan and ingest the entire project:
specforge ingest
```

* Automatically detects `.pdf`, `.docx`, `.xlsx`, and `.pptx` files.
* Converts them to clean Markdown tables and formulas.
* Generates `docs/_context/context-pack.md` in ~20 ms, feeding the AI architect with complete ground truth.

---

## 7. Socratic BDD Interview & Legacy Migration (`specforge interview` & `--from-repo`)

```bash
specforge interview --feature "Customer Loan Amortization"
```

The AI architect conducts a structured interview covering the **7 Canonical Sections**:
1. Business Intent (The Why)
2. Ubiquitous Language (Domain Glossary)
3. Domain Invariants (INV-01, INV-02)
4. User Stories (`As a... I want to... So that...`)
5. Acceptance Criteria (Gherkin BDD: `Scenario`, `Scenario Outline` with `Examples:`)
6. Boundaries & Out of Scope (Explicit KISS / YAGNI)
7. Open Questions (`[NEEDS CLARIFICATION]`)

The resulting specification is pre-seeded in `specs/0001-<slug>.md` and sealed with a cryptographic **SHA-256** hash:
```markdown
<!-- seal: sha256:78a274f4cc77316bb1e7ce2c6fca1ff43a290d76489737bbeb2141c626a88b41 -->
```

### Legacy Migration Mode (`--from-repo`):
```bash
specforge interview --from-repo /path/to/legacy-system --feature "Loan Calculation Module"
```
Reverse-engineers legacy codebases (COBOL, Java 6, PHP): extracts the pure business rules (the **WHAT**) and discards obsolete syntax (the **HOW**), creating sealed BDD specs in your new project.

---

## 8. The `[NEEDS CLARIFICATION]` Business Circuit Breaker

If a specification contains unresolved ambiguities or open questions marked with `[NEEDS CLARIFICATION]`:
* **Immediate Hard Halt:** `specforge loop` halts before writing a single line of code or generating tests.
* **Triage Box:** Displays the exact list of open questions pending resolution by the Product Owner.
* **Integrity Guarantee:** Protects against AI hallucinations and ensures code is only built against validated requirements.

---

## 9. The Resilient TDD Assembly Line (`specforge loop` & `--resume`)

```bash
specforge loop
```

### The 3 Phases:
1. **RED:** Generates unit test stubs. Executes test suite. If tests pass without implementation, it aborts for **YAGNI Violation**.
2. **GREEN:** Generates minimal production code. Retries up to 3 times with compiler error feedback.
3. **REFACTOR:** Executes static quality gates. Requests clean refactoring if any gate is breached.

### Fault Tolerance with `--resume`:
State is persisted atomically in `.sdd-state.json`. If execution is interrupted by network failure or battery loss:
```bash
specforge loop --resume
```
Validates the SHA-256 spec seal and resumes at the exact micro-step without wasting tokens.

---

## 10. Autonomous Visual E2E Testing (`specforge e2e` — TesterArmy Engine)

Native Go browser automation built on `chromedp` (Chrome DevTools Protocol):

```bash
specforge e2e --url http://localhost:3000
```

* **No Node.js / Playwright:** Directly launches local Google Chrome or Microsoft Edge.
* **Simplified UI Tree:** Extracts only visible interactive elements with deterministic selectors (`data-sdd-id`).
* **Typed Action Loop (up to 15 steps):** AI emits JSON `VisualAction` (`click`, `type`, `assert`, `wait`).
* **Automated Screenshot on Failure:** If an assertion fails or times out, it captures `docs/e2e/screenshot.png` and exits with code 1.

---

## 11. Adversarial Security Audit (`specforge audit`)

Inspired by Cloudflare's security evaluation harness:

```bash
specforge audit --diff      # Fast pre-merge scan on git diff lines
specforge audit --full      # Comprehensive codebase security audit
```

* **Phase 1: Reconnaissance:** Maps attack surface and entry points.
* **Phase 2: Red Team Hunting:** Identifies high-risk attack classes (Injection, SSRF, Deserialization).
* **Phase 3: Blue Team Verification:** Validates findings, filters false positives, and generates `docs/security/findings.json`.

---

## 12. Quality Guardrails Matrix

| Guardrail | Tool | Command to Run Manually |
| :--- | :--- | :--- |
| **Linters** | ESLint / golangci-lint / Ruff | `npm run lint` / `go vet ./...` / `ruff check .` |
| **DRY Enforcer** | `jscpd` (threshold 0%) | `npx jscpd ./src --threshold 0` |
| **Dead Code** | `knip` | `npx knip` |
| **Mutation Testing** | `stryker` (threshold >= 80%) | `npx stryker run` |
| **Architecture** | `ArchUnit` | `mvn test -Dtest=ArchitectureTest` |

---

## 13. Complete Command Reference

| Command | Purpose | Key Options |
| :--- | :--- | :--- |
| `specforge init` | Workstation zero-keys setup | `--agent`, `--project`, `--non-interactive`, `--force` |
| `specforge setup` | Deploy standards, archetypes, and memory | `--stack [react/java/go/python]`, `--force` |
| `specforge ingest` | Ultra-fast project architecture scanner | `--output`, `--sync`, `--no-convert`, `--print` |
| `specforge doc` | Convert PDF, Word, Excel to Markdown | `--recursive`, `--output` |
| `specforge interview` | Socratic BDD interview & spec sealing | `--feature`, `--from-repo`, `--output`, `--agent` |
| `specforge loop` | TDD assembly line (Red -> Green -> Refactor) | `--resume`, `--spec`, `--agent` |
| `specforge e2e` | Autonomous Visual E2E test engine | `--url`, `--spec`, `--headless`, `--max-steps`, `--screenshot` |
| `specforge audit` | Adversarial security audit | `--diff`, `--full`, `--fail-on [HIGH/MEDIUM]` |
| `specforge consistency`| Deterministic consistency verification | `--FailOn` |
| `specforge build` | Build validation | `--dry-run` |
| `specforge version` | Platform & telemetry info | — |

---

## 14. Open Source & Apache 2.0 Compliance

SpecForge is licensed under the **Apache License, Version 2.0**.
* **Permissive Distribution:** Free for personal, open source, and commercial proprietary integration.
* **Patent Protection (Section 3):** Broad contributor patent grant with automatic defensive termination against patent trolls.
* **Attribution Notice:** See [NOTICE](NOTICE) and [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md).

```text
Copyright 2026 SpecForge Contributors | jefmonjor.dev
```

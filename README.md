# Programming Helper
 
A guided programming web application by **Infinity Cybertech**. A learner describes a software idea and receives structured, senior-mentor-style guidance: clarify the problem, plan small milestones, implement each step independently, submit their own code for review, and reflect on what they learned.
 
> **The learner stays the author of the code.** Programming Helper explains concepts, asks questions, gives progressively stronger hints and reviews the learner's own work. It never writes the solution.
 
---
 
## Table of contents
 
- [Features](#features)
- [The no-solution-code policy](#the-no-solution-code-policy)
- [Tech stack](#tech-stack)
- [Project structure](#project-structure)
- [Getting started](#getting-started)
- [Environment variables](#environment-variables)
- [API overview](#api-overview)
- [Security and privacy](#security-and-privacy)
- [Testing and acceptance targets](#testing-and-acceptance-targets)
- [Roadmap](#roadmap)
- [Contact](#contact)
---
 
## Features
 
**MVP scope**
 
- Individual accounts, onboarding and private projects
- Idea intake, editable milestones and guided learning sessions
- Progressive hints (H1 to H4), text/file submissions and static code review
- Line-linked review findings with evidence labels
- Concept checks, progress journal and Markdown export
- Account controls: sessions, usage limits, export and deletion
**Not in the MVP**
 
- Classrooms, cohorts and institution dashboards
- Repository import, commits, deployment or shell access
- Running user code
- Automatic assignment completion
- Paid subscriptions
Initial language coverage: **JavaScript/TypeScript** and **Go**.
 
---
 
## The no-solution-code policy
 
This is enforced by the application, not just by a prompt. Every model route, export and review uses the same policy version.
 
| Level | Assistance | Learner action |
| --- | --- | --- |
| **H1: Orient** | Restate the goal, name the concept, ask a diagnostic question | Explain the intended behaviour |
| **H2: Focus** | Point to a likely concern, line range or official reference | Inspect the area and form a hypothesis |
| **H3: Structure** | Describe reasoning steps and edge cases in prose | Make an independent attempt |
| **H4: Investigate** | Explain why an approach may fail and propose an experiment | Run it locally and report evidence |
 
Rules:
 
- Hint strength rises only when the learner chooses it.
- There is no "show the answer" level, and no admin or paid switch can bypass the policy.
- Responses are buffered and checked before display (schema validation, code-pattern detector, solution-disclosure classifier, quotation check). One regeneration is allowed; after that a safe conceptual prompt is returned and a policy event is recorded.
---
 
## Tech stack
 
| Layer | Choice |
| --- | --- |
| Frontend | Next.js (App Router), TypeScript |
| Backend API | Go, versioned HTTP API |
| Worker | Go, separate process from the same repository |
| Database | PostgreSQL (recommended) with a durable jobs table |
| Storage | Private object storage for attachments and exports |
| Identity | Managed OpenID Connect provider (authorisation code + PKCE) |
| AI | Provider-neutral adapter, credentials available to backend only |
 
Pin exact dependency versions in lockfiles at kickoff and record the tested Node.js and Go toolchains.
 
---
 
## Project structure
 
```
Programming_Helper/
├── app/                          # Next.js App Router
│   ├── (public)/                 # landing page, sample journey, sign-in
│   ├── dashboard/                # active projects and resume action
│   ├── workspace/
│   │   └── [projectId]/          # mentoring workspace, plan, review, journal
│   ├── settings/                 # sessions, export, deletion, usage
│   ├── layout.tsx
│   └── globals.css
│
├── components/
│   ├── AIMentor.tsx              # conversation, hint ladder, composer
│   ├── CodeEditor.tsx            # read-only source viewer with line highlights
│   ├── FileExplorer.tsx          # submitted files and versions
│   ├── Sidebar.tsx               # milestone rail
│   ├── TerminalPanel.tsx         # learner-pasted output (expected vs actual)
│   ├── FindingCard.tsx           # review finding with evidence label
│   ├── HintLadder.tsx            # H1 to H4 controls
│   └── JobStatus.tsx             # queued, analysing, checking, ready
│
├── lib/
│   ├── api.ts                    # typed client for the Go API
│   ├── types.ts                  # generated from openapi.yaml
│   └── useJobPolling.ts
│
├── backend/                      # Go service
│   ├── cmd/
│   │   ├── api/main.go
│   │   └── worker/main.go
│   ├── internal/
│   │   ├── auth/                 # OIDC, PKCE, server-tracked sessions
│   │   ├── projects/             # projects and milestones
│   │   ├── submissions/          # upload limits, immutable versions
│   │   ├── mentor/               # prompts, AI adapter, orchestration
│   │   ├── policy/               # no-solution-code gate
│   │   ├── jobs/                 # durable jobs, leases, retries
│   │   ├── usage/                # quotas and idempotent ledger
│   │   ├── export/
│   │   └── httpapi/              # routes, middleware, error codes
│   ├── migrations/
│   └── openapi.yaml
│
├── .env.example
└── README.md
```
 
The Go service owns authentication, business rules, project access, persistence and model orchestration. Next.js must not duplicate those rules.
 
---
 
## Getting started
 
### Prerequisites
 
- Node.js (use the version recorded in the repo toolchain file)
- Go (use the version recorded in `backend/go.mod`)
- PostgreSQL
- An OpenID Connect provider and an AI provider account for non-mock environments
### 1. Clone and configure
 
```bash
git clone <your-repository-url> Programming_Helper
cd Programming_Helper
cp .env.example .env
```
 
Fill in `.env` (see [Environment variables](#environment-variables)).
 
### 2. Start the database and run migrations
 
```bash
# Create the database, then apply migrations with your chosen migration tool
cd backend
# example: migrate -path migrations -database "$DATABASE_URL" up
```
 
### 3. Run the Go API and worker
 
```bash
cd backend
go run ./cmd/api
# in a second terminal
go run ./cmd/worker
```
 
### 4. Run the Next.js frontend
 
```bash
# from the repository root
npm install
npm run dev
```
 
Open `http://localhost:3000`.
 
> Serve the browser and API through **one public origin** (for example via a reverse proxy) so that Secure, HttpOnly session cookies and CSRF checks work as designed.
 
---
 
## Environment variables
 
Create `.env` from `.env.example`. Never commit real secrets.
 
```bash
# App
APP_ENV=development
PUBLIC_ORIGIN=http://localhost:3000
NEXT_PUBLIC_API_BASE_URL=/v1
 
# Go API
API_PORT=8080
DATABASE_URL=postgres://user:password@localhost:5432/programming_helper?sslmode=disable
SESSION_COOKIE_NAME=ph_session
SESSION_TTL_MINUTES=60
 
# Identity (OpenID Connect)
OIDC_ISSUER_URL=
OIDC_CLIENT_ID=
OIDC_CLIENT_SECRET=
OIDC_REDIRECT_URL=http://localhost:3000/auth/callback
 
# AI provider (backend only)
AI_PROVIDER=
AI_API_KEY=
AI_MODEL_ID=
POLICY_VERSION=v1
 
# Limits
MAX_FILES_PER_REVIEW=10
MAX_SOURCE_BYTES=102400
 
# Object storage
STORAGE_BUCKET=
STORAGE_REGION=
```
 
---
 
## API overview
 
Documented in `backend/openapi.yaml`. Frontend request types are generated from it.
 
| Endpoint | Purpose |
| --- | --- |
| `POST /v1/projects` | Create a project from title, brief, language and experience |
| `POST /v1/projects/{id}/plan` | Request a milestone plan (returns a job ID); approval is a separate action |
| `PATCH /v1/milestones/{id}` | Update a milestone using the expected version (`409` on conflict) |
| `POST /v1/projects/{id}/messages` | Send a message with a hint level (returns `202` and a job ID) |
| `POST /v1/projects/{id}/submissions` | Create an immutable submission version |
| `POST /v1/submissions/{id}/reviews` | Queue a review (idempotent on retry) |
| `GET /v1/jobs/{id}` | Job state, progress and approved result reference |
| `POST /v1/account/exports` | Start an asynchronous export |
| `DELETE /v1/projects/{id}` | Start project deletion |
 
Error codes: `401` unauthenticated, `403` forbidden, `413` payload too large, `429` rate or quota limit, `503` provider unavailable. Errors include a stable code, a readable message, field errors and a request ID.
 
### Mentoring request flow
 
1. Learner sends a message or submission with project ID, milestone ID and an idempotency key.
2. API validates ownership, size, language, quota and hint level, then stores the request and a durable job in one transaction.
3. Worker loads a bounded project summary and scans for likely secrets.
4. AI adapter sends fixed teaching instructions with untrusted learner content kept separate.
5. Worker validates the schema, checks for solution disclosure and verifies quotations.
6. The validated review and usage are saved transactionally; the client is notified or polls the job.
---
 
## Security and privacy
 
- OIDC with authorisation-code flow and PKCE; short-lived server-tracked sessions in Secure, HttpOnly cookies; no bearer tokens in browser storage
- Origin and CSRF validation on state-changing requests
- Pasted code, file names, comments and model output are treated as **untrusted data**; source is shown as escaped text and mentor content uses a restricted Markdown renderer with raw HTML disabled
- The model has no shell, repository-write or unrestricted browsing tools
- Authorisation is enforced deterministically in the Go API on every request
- Logs hold identifiers, timings, error categories and token totals only, not prompts or source code
- Retention baseline: submissions kept until project deletion; deleted active records purged within 30 days
- Upload limits (proposed): 10 UTF-8 text files and 100 KB combined; archives and binaries are rejected
Learners should remove passwords, API keys, personal data and proprietary material before submitting. The secret scanner is an aid, not a guarantee.
 
---
 
## Testing and acceptance targets
 
These are proposed release targets, not measured results.
 
| Area | Target |
| --- | --- |
| Mentoring boundary | At least 200 solution-seeking prompts with zero executable solution disclosures |
| Learning usefulness | 90% or more of 50 reviewed journeys score at least 4/5 |
| Authorisation | Every endpoint rejects cross-account identifiers in integration tests |
| Reliability | A retried idempotent request yields one job and one usage entry |
| Safe handling | Script-bearing source and Markdown stay inert; bad uploads rejected before provider submission |
| Accessibility | WCAG 2.2 AA; core journeys work by keyboard and screen reader |
| Performance | 50 concurrent learners; API p95 under 500 ms; mentor response p95 under 30 s |
| Recovery | Staging restore within 4 hours; at most 24 hours of potential data loss |
 
Re-run policy and quality tests whenever the model, prompt, retrieval or filters change.
 
---
 
## Roadmap
 
| Package | Focus |
| --- | --- |
| A | Discovery and policy |
| B | Foundation: repo, CI, identity, API skeleton, migrations, design tokens |
| C | Guided learning: intake, milestones, workspace, journal |
| D | AI review: adapter, jobs, quotas, policy gate |
| E | Release hardening: accessibility, security review, export and deletion |
| F | Controlled public pilot |
 
Later (separately scoped): classrooms, read-only repository import, more languages, paid plans.
 
---
 
## Design notes
 
- Primary purple `#6D28D9`, deep purple `#2E1065`, soft purple `#F5F3FF`, ink `#172033`, muted text `#526074`
- Interface font: Inter or similar; code font: JetBrains Mono or similar
- Desktop workspace: milestone rail, mentor conversation and source/review panel; below 768 px, tabs for Task, Mentor and My Code
- Primary touch targets around 44 px; status is never shown by colour alone
---
 
## Contact
 
**Infinity Cybertech**
Website: https://www.infinitycybertech.com/
Email: admin@infinitycybertech.com
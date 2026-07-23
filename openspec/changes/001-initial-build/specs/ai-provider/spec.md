# Capability: ai-provider

## Purpose

Reach any OpenAI-compatible LLM endpoint — cloud BYOK or self-hosted — as the transport beneath the `ai-triage` capability's annotation layer, through one config-driven client rather than a distinct code path per provider. Covers OpenAI, Azure OpenAI, Groq, OpenRouter, and self-hosted Ollama/vLLM/llama.cpp, all of which implement the OpenAI Chat Completions API shape. A distinct Anthropic-native adapter is explicitly out of scope for now (see the dedicated non-goal requirement below) — Claude does not speak the OpenAI-compatible shape, and adding a second adapter is deferred rather than built speculatively.

## ADDED Requirements

### Requirement: Single OpenAI-compatible client, config-driven endpoint
The system SHALL implement all LLM calls for `ai-triage` through a single OpenAI-compatible client, with endpoint, credential, and model selected entirely by configuration (`config.aiProvider.baseUrl`, `.apiKey`, `.model`) — never a distinct code path per provider.

#### Scenario: Switching providers is a config change, not a code change
- **GIVEN** the engine is configured against one OpenAI-compatible provider
- **WHEN** the operator changes `config.aiProvider.baseUrl`/`.apiKey`/`.model` to point at a different OpenAI-compatible provider
- **THEN** `ai-triage` continues to function with no code change

### Requirement: Cloud BYOK providers supported uniformly
Any cloud provider implementing the OpenAI-compatible Chat Completions API (OpenAI, Azure OpenAI, Groq, OpenRouter, and similar) SHALL work by configuring `baseUrl` and `apiKey` alone.

#### Scenario: Provider swap across cloud BYOK options
- **GIVEN** `config.aiProvider` is set for OpenAI
- **WHEN** it is changed to Groq's or OpenRouter's OpenAI-compatible endpoint and API key
- **THEN** `ai-triage` produces annotations without any code difference between the two configurations

### Requirement: Self-hosted model as an optional Docker Compose service
This repo's Docker Compose stack SHALL include an optional `ollama` service, gated behind a Compose profile so it does not start by default — the operator opts in via the guided setup command (see `deployment-packaging`).

#### Scenario: Guided setup offers a local-model path
- **GIVEN** the operator runs the guided setup command
- **WHEN** they choose "use a local open-weight model" instead of a hosted API key
- **THEN** the `ollama` Compose profile is enabled and `config.aiProvider.baseUrl` is set to the in-network Ollama service, with no hosted API key required

### Requirement: Reachable local models without a redundant container
If an OpenAI-compatible server (Ollama or otherwise) is already running on the operator's own machine, configuration SHALL be able to point at it directly rather than requiring the Compose `ollama` service to also run.

#### Scenario: Point at an existing local Ollama instance
- **GIVEN** the operator already runs Ollama natively on their machine
- **WHEN** they set `config.aiProvider.baseUrl` to that instance (e.g., via `host.docker.internal`) during guided setup
- **THEN** the Compose `ollama` service is not started, and `ai-triage` reaches the existing instance directly

### Requirement: Reachability constraint is documented and validated, not silently broken
A locally-hosted model is only reachable when Convex itself runs in the same network as that model (i.e., self-hosted Convex in the same Docker Compose stack). If Convex is instead hosted anywhere Convex's own infrastructure runs the actions (for example, Convex Cloud managed hosting), `config.aiProvider.baseUrl` must be a publicly-reachable endpoint — a local-only address will never be reachable from there. This constraint SHALL be documented, and guided setup SHALL validate reachability of the configured `baseUrl` rather than allowing an unreachable local-only address to be configured silently.

#### Scenario: Setup rejects an unreachable local address when Convex isn't self-hosted alongside it
- **GIVEN** the operator is using a Convex hosting option other than self-hosted-alongside-the-model
- **WHEN** they attempt to set `config.aiProvider.baseUrl` to a local-network-only address
- **THEN** setup flags it as unreachable rather than accepting it and failing silently at triage time

### Requirement: Deterministic timeout and failure handling; annotation stays best-effort
Each LLM call SHALL be bounded by a configurable timeout and retry policy. On failure or timeout, the affected finding's AI annotation SHALL be marked unavailable for that cycle — the deterministic finding pipeline (priority, severity, KEV/EPSS) SHALL NOT block or fail because of an LLM-call failure.

#### Scenario: Self-hosted model times out under load
- **GIVEN** a self-hosted model on modest hardware exceeds the configured timeout
- **WHEN** `ai-triage` processes the finding queue
- **THEN** the finding's priority and severity fields are fully set as normal, its annotation is marked unavailable for this cycle, and the ingestion pipeline completes without error

### Requirement: No distinct Anthropic-native adapter (non-goal, for now)
The system SHALL NOT implement a separate Anthropic Messages API adapter at this time. This capability's contract is the OpenAI-compatible shape only; a dedicated Anthropic-native adapter, if ever wanted, is a future, separately-scoped change — not a silent addition to this one.

#### Scenario: No Anthropic SDK dependency in the provider layer
- **GIVEN** the `ai-provider` implementation
- **WHEN** its dependencies are reviewed
- **THEN** it depends on an OpenAI-compatible client only, with no Anthropic SDK or Anthropic-specific request shaping present

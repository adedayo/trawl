# Tasks: 021-ai-annotation-and-triage

**Not scheduled.** Nothing depends on it, and the product is complete without
it. The one item worth doing early is Phase 0, because it costs almost nothing
and it removes a requirement that currently passes for the wrong reason.

## Phase 0 — Make 014's requirement binding before it has a subject

- [ ] A rendering test that exercises a *present* annotation and asserts it is
      labelled advisory and visually subordinate. Today that requirement passes
      because there is no annotation to subordinate, which is indistinguishable
      in CI from passing because it holds
- [ ] Record in the 014 ledger that the requirement was vacuous until this
      test existed. The ledger's value is that it says what was actually true
      at the time

## Phase 1 — The boundary, before the feature

- [ ] Types such that a provider response cannot reach severity, priority, KEV,
      EPSS or any probability or monetary field. Make it impossible rather than
      forbidden — the 012 boundary test is the precedent, and "must not" in a
      spec has never stopped a change that had a good reason
- [ ] A test that recomputing an annotated finding's priority yields exactly
      the unannotated result
- [ ] A test that fails if the annotation path acquires a write to a computed
      field

## Phase 2 — Provider

- [ ] One OpenAI-compatible client; `baseUrl` / `apiKey` / `model` from
      configuration. Cloud BYOK and self-hosted are one code path because they
      are one protocol, and self-hosting is the only answer for an operator who
      cannot send estate data to a third party
- [ ] No provider is the default, and no finding content leaves an installation
      that has not named one
- [ ] No failover. A silent second provider changes which model produced an
      annotation while its provenance says otherwise
- [ ] Failure is recorded as failure and rendered as *annotation unavailable*,
      which is not the same as an annotation saying there is nothing to say

## Phase 3 — Storage and provenance

- [ ] An annotation record carrying provider, model, prompt version and
      generation time. Decide whether `findings.ai_annotation` is superseded —
      one column cannot carry provenance, nor more than one annotation
- [ ] Annotations from a given model can be identified and invalidated as a
      set, for when a model turns out to be systematically wrong about a class
      of finding
- [ ] Migration through the 018 mechanism

## Phase 4 — Hostile input

- [ ] Findings text is untrusted. Titles, certificate subjects, banners and
      repository contents are attacker-controlled, and any model reading them
      is reading hostile input
- [ ] The model holds no tool, no store access and no network capability. An
      agent that can only produce text cannot be prompt-injected into doing
      anything — the same asymmetry as the dependency triage gate, which can
      withhold auto-merge but never grant it
- [ ] Response shape validated before storage; a non-conforming response is a
      failed annotation, not a stored one
- [ ] A test using a finding containing an explicit instruction to raise its
      own severity, asserting the stored fields are unchanged

## Phase 5 — Triage assistance

- [ ] Grouping of findings sharing an underlying cause, offered as a grouping
      and not as an ordering
- [ ] A test that adding or removing annotations does not change the
      remediation queue order
- [ ] No suppression. A model may state that it believes a finding is a false
      positive; the finding still appears and is still counted
- [ ] Cost and latency surfaced, since this is the first feature where using
      the product costs the operator money per action

## Exit Criteria

An installation with no provider is complete and shows no sign that a feature
is missing. An annotated finding's computed fields are bit-identical to the
unannotated case. A finding containing an instruction to the model changes
nothing. Every rendered annotation is labelled advisory, and a test proves it
against a real annotation rather than against its absence.

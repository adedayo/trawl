# Change: 021-ai-annotation-and-triage

## Status

**Proposed, not scheduled.** Promoted from *Unclaimed work* in
`openspec/STATUS.md`.

## Why

`openspec/project.md` describes a single OpenAI-compatible client, BYOK or
self-hosted, producing annotation and narrative only. The `findings` table
carries an `ai_annotation` column. Change 014 requires that AI annotation be
labelled advisory and visually subordinate.

None of it is built. The column is never written, and no provider is
configured. That means **Change 014's requirement is satisfied vacuously** —
there is no annotation, so nothing fails to be subordinate. A passing
requirement that passes because its subject does not exist is the most
dangerous kind, because it reads in CI exactly like one that holds. It will
stop holding the moment an annotation is written, and nothing is watching for
that moment.

The capability is worth having. An operator facing several hundred findings
needs help with the part machines are bad at defining and good at drafting:
what this means for this organisation, what to say to the team that owns the
asset, which of these thirty look like the same underlying mistake. What they
do not need is a model quietly moving a severity.

The structural risk is specific, and it is why this change is about boundaries
more than about prompts. Trawl's central claim is that its numbers are
computed, traceable and reproducible. An annotation layer sits immediately
adjacent to those numbers. The pressure to let it "just adjust" a priority —
because the model is often right — is exactly the pressure that would end the
claim, and it would end it invisibly, because an adjusted number looks like a
computed one.

There is a second risk the dependency gate already names. An agent that can
promote can be prompt-injected by its input; an agent that can only veto
cannot. Findings contain attacker-controlled text — page titles, certificate
subjects, banners, repository contents. Any model reading them is reading
hostile input, and must not hold a capability worth attacking.

## What Changes

- **A provider client**: one OpenAI-compatible interface, `baseUrl` / `apiKey`
  / `model` from configuration. Cloud BYOK and self-hosted are the same code
  path, because they are the same protocol, and a self-hosted deployment is the
  only answer for an operator who cannot send findings about their estate to a
  third party.
- **Annotation is stored separately and is structurally incapable of
  overwriting a computed field.** Not "must not" — the types should make it
  impossible, in the way Change 012's boundary test makes an unlabelled figure
  impossible rather than discouraged.
- **A no-provider state that is the default and is fully functional.** Every
  surface works with annotation absent. An operator who never configures a
  provider must not encounter a degraded product.
- **Provenance on every annotation**: model, provider, prompt version, and the
  time it was generated. An annotation whose model is unknown cannot be
  reassessed when that model turns out to have been wrong in a systematic way.
- **Triage assistance that groups and drafts, and does not rank.** Clustering
  related findings and drafting remediation narrative are useful. Ordering the
  queue is the deterministic engine's job.
- **Findings text is treated as hostile input.** The model's output is
  constrained to a shape the system validates, and the model holds no
  capability — no tool access, no write path, no ability to suppress a finding.
- **Change 014's subordination requirement becomes non-vacuous**, enforced by a
  test that fails if an annotation is rendered without its advisory label.

## Explicitly Out of Scope

- **No AI in any scoring, ordering, routing or scope decision.** Severity,
  priority, KEV, EPSS, alert routing and scope validation stay deterministic.
- **No agentic action.** The model does not scan, fetch, write to the store, or
  call tools. It reads a prompt and returns text.
- **No suppression.** A model may not mark a finding as a false positive. It
  may state that it believes one is, as annotation, for a human to act on.
- **No model training or fine-tuning on customer data**, and no provider
  defaulting to one that trains on submitted content.
- **No fallback chain across providers.** One configured provider. A silent
  failover changes which model produced an annotation while its provenance
  says otherwise.

## Impact

- **Engine and schema.** An annotation table with provenance; the existing
  `findings.ai_annotation` column is likely superseded by it, since a single
  column cannot carry provenance or more than one annotation.
- **Introduces outbound requests carrying estate data to a third party** when
  a cloud provider is configured. This is the most sensitive egress in the
  product and needs explicit, informed opt-in — not a default with a note.
- **Makes a currently vacuous requirement in 014 binding**, which is the
  cheapest part of this change and should land with the first annotation, not
  after it.
- **Cost and latency become user-visible** for the first time.

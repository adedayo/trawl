# Tasks: 024-vantage-service-reachability

**Precursor:** Vantage Change 016. **Consumer:** Change 007 contact probability.

## Phase 0 - Contract and dependency

- [x] Vantage Change 016 is accepted and released with the structured service
      observation API
- [x] Add Trawl contract tests for the exact Vantage version and observation
      fields consumed
- [x] Confirm no `replace` directive or subprocess dependency is needed

## Phase 1 - Reachability observations

- [x] Translate TCP, TLS, HTTP and STARTTLS observations into Trawl's typed
      service observation model
- [x] Preserve layer-specific states: responding, not_responding, unknown
- [x] Persist endpoint, protocol, profile, evidence and timestamp
- [x] Persist coverage reasons for timeout, cancellation, policy exclusion and
      scope refusal
- [x] Test that DNS resolution alone does not create a responding service

## Phase 2 - Exposure-history integration

- [x] Convert explicit responding service observations into exposure-history
      observations
- [x] Keep observed and inferred durations separate across successful runs and
      assessment gaps
- [x] Close an exposure only on a successful clean result at the relevant layer
- [x] Keep failed, unknown and not_checked results open and inferred
- [x] Test left-censoring and live time-at-risk growth after process restart

## Phase 3 - Scope, safety and operation

- [x] Prove an out-of-scope service receives zero network queries through the
      Vantage transport used by Trawl
- [x] Enforce declared-service profile limits for concurrency, timeout and
      request budget
- [x] Confirm certificate names, redirects and discovered hosts cannot widen
      target scope implicitly
- [x] Add operator-facing documentation for the service probe egress

## Phase 4 - Contact-model handoff

- [ ] Expose reachability coverage and exposure history to Change 007's factor
      computation
- [ ] Reachability unknown suppresses a contact estimate rather than becoming
      zero or a nominal value
- [ ] A responding service affects the exposure-derived contact prior only
- [ ] Add a CI guard proving assessment code cannot write contact posteriors

## Phase 5 - TLS migration

- [x] Replace TLSAudit's supported protocol and certificate evidence with
      Vantage structured observations
- [x] Add SNI, certificate, protocol and cipher golden-vector tests
- [x] Add bounded STARTTLS profiles for supported mail protocols
- [ ] Retire Trawl's dependency on any TLSAudit subprocess or raw output

## Phase 6 - Performance decision

- [ ] Benchmark declared-service TCP connect against any proposed raw-SYN
      candidate-discovery backend
- [ ] Compare wall time, packets, CPU, memory, false negatives, packet loss,
      IPv4/IPv6 and privilege requirements
- [ ] Adopt a raw-SYN backend only if the benchmark demonstrates material value
      for a named profile and the same observation contract remains intact

## Phase 7 - Legacy-tool retirement

- [ ] Inventory the supported TCPScan and TLSAudit workflows and map each to a
      Vantage profile or an explicit unsupported/migration note
- [ ] Remove all supported Trawl dependencies on TCPScan and TLSAudit
- [ ] Add replacement documentation and examples to Vantage and Trawl
- [ ] Publish final deprecation releases for TCPScan and TLSAudit pointing to
      Vantage
- [ ] Complete one release cycle with no unresolved migration defect
- [ ] Mark the legacy repositories deprecated and point their README,
      repository metadata and release pages to Vantage
- [ ] Keep TCPScan and TLSAudit public and read-only as portfolio/history
      projects, or archive them later as optional housekeeping; neither choice
      blocks the technical migration

## Exit Criteria

Trawl can run an authorised declared-service assessment through Vantage, record
layer-specific structured reachability and coverage, populate exposure history
without treating DNS as service response, and feed only the exposure-derived
contact prior. TLS evidence is structured and tested, no subprocess or elevated
privilege is required for the default profile, and a raw-packet backend is
optional rather than an unmeasured operational dependency.

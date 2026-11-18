# Tasks: 007-contact-probability

**Do not start until Change 006 is implemented.**

## Phase 0 — Preconditions
- [x] Change 006 signal registry populated (stage declarations present on every mapped signal; enforced by `TestSignalRegistrySingleStageConsumption` in `pkg/contact/guardrail_test.go`)
- [x] Posture history retention confirmed sufficient to derive first-seen/last-seen per asset-service (`service_observations` is append-only in `pkg/store/sqlite/services.go` and preserves `first_seen`, `last_seen` and `observed_at` indefinitely across runs)
- [x] Source review: published mass-scan, internet-background-radiation and honeypot contact-rate research, per service class (`ambient-rate-evidence.md`: Durumeric et al. 2014, Richter & Berger 2019, Hiesgen et al. 2022, Pauley et al. 2023)

## Phase 1 — Exposure history
- [x] `assetExposureHistory`: first-observed-exposed, last-observed-exposed, still-exposed flag, per asset-service (`pkg/store/contact.go`, `pkg/store/sqlite/contact.go`, `pkg/store/sqlite/contact_test.go`)
- [x] Left-censoring detection and flagging (`ComputeHistory` in `pkg/contact/history.go`, `TestFirstExposureIsLeftCensoredAndBlindTimeIsEstimated`)
- [x] Detection-latency derivation from assessment cadence (`BlindDuration` in `pkg/contact/history.go`, `TestFirstExposureIsLeftCensoredAndBlindTimeIsEstimated`)
- [x] Live time-at-risk computation (grows with wall clock, not frozen at last scan; `History.LiveTimeAtRisk` in `pkg/contact/history.go`, `TestUnremediatedExposurePContactStrictlyIncreasesOverWallClockTime`)
- [x] Split accrual: `observedDuration` over intervals bounded by successful assessments, `inferredDuration` over gaps; the two are stored separately and never summed into a single "measured" figure
- [x] Window closure requires a check that ran and returned clean; `not_checked` and `check_failed` extend the inferred component and record no remediation
- [x] Property test: an exposure assessed continuously reports zero inferred duration; the same exposure with an observation gap reports the gap as inferred, and total time at risk is unchanged
- [x] Blind/aware split: cadence-derived estimate of pre-detection interval, labelled estimated, reported alongside the observed post-detection interval
- [x] `expectedDetectionLatency` per asset class from cadence (`C/2` expected, `C` worst case), reported as a property of the monitoring programme
- [x] Cadence what-if: given the ledger's observed weakness-introduction rate, compute the exposure reduction from a proposed cadence change (`EvaluateCadenceChange` in `pkg/contact/history.go`, `TestCadenceEvaluationAndLatencySeparation`)
- [x] Detection latency and remediation latency stored and reported as separate figures; no combined responsiveness metric (`ComputeLatencySplit` in `pkg/contact/history.go`, `TestCadenceEvaluationAndLatencySeparation`)

## Phase 2 — Factor computation
- [x] Reachability gate from confirmed responding services; unknown suppresses the estimate (`DeriveReachability` and `ComputeContactEstimate` in `pkg/contact/estimate.go`, `TestReachabilityGatesAndSuppressesUnknown`)
- [x] Discoverability multiplier from CT, passive DNS, wildcard zone, zone-transfer exposure, index presence — with cap (`ComputeDiscoverability` in `pkg/contact/estimate.go`, `TestDiscoverabilityHasFloorAndCap`)
- [x] Access-barrier attenuator from auth requirement, allowlist, mTLS, WAF/CDN fronting — bounded strictly below 1 (`ComputeAccessBarrier` in `pkg/contact/estimate.go`, `TestAccessBarrierAttenuatesStrictlyBelowOne`)
- [x] Ambient attention lookup by service class (values sourced from Change 008 packs; a stub table until 008 lands — `LookupAmbientAttention` in `pkg/contact/estimate.go`)
- [x] Pure-function unit tests per factor (`pkg/contact/estimate_test.go`)

## Phase 3 — Contact estimate
- [x] `computeContactEstimate` pure function: λ_contact, P(contact | T), intervals (`ComputeContactEstimate` in `pkg/contact/estimate.go`)
- [x] Window selection by weakness class with recorded rationale (`SelectWindow` in `pkg/contact/estimate.go`)
- [x] Interval propagation from factor uncertainty (`ComputeContactEstimate` in `pkg/contact/estimate.go`)
- [x] Calibration label assignment (`LabelIllustrative` / `LabelSourced` / `LabelCalibrated` in `pkg/contact/estimate.go`)
- [x] Coverage figure attached from Change 006 states (`CoverageFigure` on `ContactPriorEstimate`, `deriveCoverageFigure` in `pkg/service/contact.go`)

## Phase 4 — Instance aggregation
- [x] Instance enumeration from inventory per weakness (`AssessmentService.WeaknessAggregations` in `pkg/service/contact.go`)
- [x] Heterogeneous aggregation `1 − Π(1 − P_i)`, reducing to `1 − (1 − P)^N` when homogeneous (`AggregateInstances` in `pkg/contact/aggregate.go`, `TestHeterogeneousAndHomogeneousInstanceAggregationAndResidual`)
- [x] Remediation coverage from confirmed-absent re-observation (`ConfirmedRemediated` in `pkg/contact/aggregate.go` and `pkg/service/contact_test.go`)
- [x] Residual estimate over uncovered instances (`ResidualPContact` in `pkg/contact/aggregate.go`, `TestHeterogeneousAndHomogeneousInstanceAggregationAndResidual`)

## Phase 4a — Calibration evidence infrastructure

These items establish the evidence path; they do not select an ambient rate or
replace the required source review. Sequenced behind Phases 2 to 4 on
2026-09-23: the calibration path is an upgrade, not a precondition, and the
change is serviceable on sourced class priors alone without it. The two open
items below are deliberately deferred behind the critical path, not
unscheduled.

- [x] Initial service taxonomy with explicit `other-tcp` fallback in `pkg/contact/service_class.go`
- [x] Compare the taxonomy against stored `ServiceObservation` labels and report the percentage mapped to a named class versus `other-tcp` via `cmd/service-class-report`
- [x] Typed calibration observations with keyed source redaction and event validation
- [x] Append-only SQLite storage for observations and reachable-service intervals
- [x] Standalone stdin collector command with `--dry-run`
- [x] Read-only experiment report with per-class/event rates, Garwood-Poisson intervals, observation windows and source-count disclosure
- [ ] Run the isolated sensor experiment and retain its raw-count provenance
- [ ] Review experiment bias, retention and reuse terms before parameter promotion

## Phase 5 — Guardrails as required checks
- [x] **Required CI check**: single-stage signal consumption (`TestSignalRegistrySingleStageConsumption` in `pkg/contact/guardrail_test.go`)
- [x] **Required CI check**: no assessment-derived path can write a contact posterior (`TestNoAssessmentPathCanWriteAContactPosterior` in `pkg/contact/guardrail_test.go`)
- [x] Test: unremediated exposure's P(contact) strictly increases over time (`TestUnremediatedExposurePContactStrictlyIncreasesOverWallClockTime` in `pkg/contact/estimate_test.go`)
- [x] Test: reachability unknown never defaults to zero or to a nominal value (`TestReachabilityGatesAndSuppressesUnknown` in `pkg/contact/estimate_test.go` and `TestContactPanelEndToEndAndFactorRemoval` in `pkg/service/contact_test.go`)

## Phase 6 — Surface
- [x] Per-asset contact panel: factors, sources, time at risk with censoring, interval, label, coverage (`AssessmentService.ContactPanel` in `pkg/service/contact.go`, `assets.ts`, `assets.html`)
- [x] Explicit statement that the figure prices ambient contact only (`contact.AmbientScopeStatement` carried on `ContactPriorEstimate` and `AssetContactPanel`, rendered in `assets.html`)
- [x] Factor-removal recompute action (`RecomputeWithoutFactor` in `pkg/contact/estimate.go`, `toggleExcludedFactor` in `assets.ts`, `TestFactorRemovalRecomputesWithoutTouchingOtherFactors`)

## Exit Criteria

An internet-reachable service with a certificate-transparency-published hostname and no access barrier produces a contact estimate with a decomposed, source-traced factor set, a measured and honestly censored time at risk, a 90% interval, a calibration label and an assessment coverage figure — and removing any single factor recomputes the estimate in one step while leaving every other factor untouched. Contact posteriors remain provably unwritable by any scan-derived path.

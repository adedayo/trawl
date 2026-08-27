# Design: 014-operator-dashboard

## Two audiences, one set of rows

The executive area is a projection, not a subsystem. Every figure it shows is a
stored value that some detail view also reads.

This is a stronger constraint than it sounds, and it is the one most commonly
abandoned under delivery pressure. The tempting shortcut is an analytics
endpoint that computes summary figures its own way — faster to write, and it
denormalises cleanly. The cost arrives later, when the board slide and the
engineer's table disagree by four, nobody can say which is right, and the
tool's credibility goes with it. Change 012 states the same constraint for the
workbench; stating it here means the workbench inherits a surface already built
that way rather than having to retrofit it.

Concretely: the executive area calls the same read models as the tabs, with
different aggregation. There is no `/api/v1/analytics` and no parallel
computation in the Angular layer.

## Why the headline figure is refused rather than caveated

`checkmate-app` leads with an "Overall Posture Score" and it works well there,
because a secrets scan over a repository has effectively total coverage: the
scanner either read the history or it did not, and it knows which.

External attack surface has no such property. Coverage is partial by
construction — checks are withheld by egress policy, nameservers time out,
third-party consent is absent, an asset was discovered an hour ago and has
never been assessed. A single number over that estate is not a slightly worse
measurement of the same quantity. It is a different quantity, and rendering it
in the same typeface as a well-covered one invites precisely the inference the
project's fifth guardrail forbids.

The obvious mitigation — show the number, add a coverage caveat — does not
work, for a reason worth stating plainly: **the number travels and the caveat
does not**. It gets screenshotted into a board pack, pasted into an email,
quoted in a meeting. Any design where the reassurance is separable from its
qualification will, in practice, separate.

So coverage is rendered inside the same element as the figure, and below a
configured floor the figure is not rendered at all. Refusing to answer is a
legitimate output. It is also more informative than a hedged answer, because it
tells the reader something actionable: assess more of the estate.

The floor is configuration rather than a constant because the right value
depends on the estate, and because a probability-affecting constant embedded in
engine code is what Change 008's model packs exist to prevent. The value in
force is displayed, so a deployment cannot quietly lower it to make the number
appear.

## Ordering, not counts

RISK-ARC §5d argues that detection produces order, not work. A dashboard
leading with "147 open findings" invites the wrong conversation — the one about
the backlog's size, which is mostly a function of how hard you looked.

The executive area therefore leads with the ordering and exposes its inputs.
The ranking is already deterministic and stored; the interface's job is to make
its inputs visible so the ordering can be challenged on its merits rather than
trusted or dismissed wholesale. Two people looking at the same finding must see
the same position, which they will, because nothing is computed per session.

## Withheld assessment is a first-class row

Change 006 Phase 7 produces exclusions with reasons attached. Those are not
errors and not noise: they are the deployment's own policy decisions, made
visible. An operator who consented to no third-party service should be able to
see the CT enumeration they are not getting and decide whether that trade is
still right.

Omitting them would reproduce, in the interface, exactly the failure the
fail-closed path was built to avoid — a check that did not run and nobody was
told. And they must lower coverage rather than being excluded from the
denominator, so a restrictive policy reads as less assurance rather than as
less to worry about.

## Live transport

The view layer depends on the existing transport abstraction (`app/src/app/transport/`), which already has Wails and HTTP implementations. The
SSE subscription belongs behind that interface alongside them.

A dropped stream must be visible. A dashboard that silently stops updating is
worse than one that never claimed to be live, because the operator's belief
about freshness is now wrong and nothing on screen contradicts it. Last-updated
time plus an explicit disconnected state is the minimum.

## Component structure

Following Trawl's conventions rather than `checkmate-app`'s: standalone
components, `signal()`/`computed()`, `@if`/`@for`, separate `.html` templates,
Tailwind v4 with spartan/ui, no ad hoc CSS.

The 933-line inline-template component in `checkmate-app` is the pattern to
avoid here, not from taste but because the accessibility floor requires
per-view automated scanning, and views that cannot be mounted independently are
awkward to scan and awkward to test.

Charts: `checkmate-app` uses ngx-charts and d3. Adopting the same is
reasonable, but the decision should be deferred until a view actually needs a
chart. Most of what this change surfaces — coverage, regressions, ordering — is
better served by tables and inline indicators than by graphics, and a charting
dependency added before a chart is needed is surface bought on speculation.

## Relationship to Change 012

| Concern | 014 (this change) | 012 (executive-workbench) |
|---|---|---|
| Question answered | What is exposed, and how much did we look at? | What is it worth, and what should we buy? |
| Figures | Counts, states, coverage, ordering | Expected loss, calibrated probability, control ROI |
| Prerequisites | Changes 005, 006 | Changes 008–011 |
| Audience | Operator and CISO | Board, CISO, engineer |

The prohibition on currency and probability here is what keeps the boundary
enforceable. Without it, the executive area would accrete a "risk score" that
looks like the workbench's output but is not derived from the model packs — a
number with no calibration label, which is the failure mode guardrail 6 names
directly.

## Testing

- **Cross-view consistency**: an executive figure and its detail view agree,
  asserted against the same store rather than against a fixture.
- **Coverage propagation**: a `check_failed` outcome never renders in the
  styling of a passing one — the interface-level counterpart to the store-level
  test already in Change 006 Phase 5.
- **Floor suppression**: below the coverage floor, no headline figure is in the
  rendered output at all.
- **Withheld visibility**: a policy-excluded check appears with its reason, and
  lowers coverage.
- **AI absent**: every view renders completely with no AI provider configured.
- **No currency**: an automated check that no view in this capability emits a
  currency symbol or a probability-labelled figure, so the 012 boundary is
  enforced by CI rather than by review.
- **Accessibility**: per-view automated scanning, with state conveyed by more
  than colour.

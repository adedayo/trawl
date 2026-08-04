# The Third Evidence Class

## What a scanner is actually telling you about risk

> **Status of this document.** Working notes behind Trawl changes 006–012, kept as prose because the argument matters more than the schema. Written to feed a post; the specs are the implementation of it. Where it extends published work, that is marked explicitly.

---

## 1. The hole in the framework is shaped like an attack-surface monitor

"The Probability Behind the Finding" is careful about its own boundary:

> Contact probability (the likelihood an attacker reaches the asset at all) is assessed separately from network exposure and attack-surface analysis and is not part of this framework.

The framework yields **P(exploit | contact)** — the product of applicability, attempt and success stages. To get an unconditional probability you need the contact term, and the post says, correctly, that it comes from somewhere else.

That somewhere else has a name and a product category. Continuous external attack-surface monitoring is exactly the practice of measuring what an attacker can reach, from outside, over time. The two halves were built to fit:

```
attack-surface monitoring  →  P(contact)
three-layer framework      →  P(exploit | contact)
                              ─────────────────────
                              P(exploit)
```

The interesting part is not the multiplication. It is what happens to the *character* of the estimate once a continuous sensor is attached to it — because a sensor supplies a kind of evidence the framework's two layers do not quite describe.

---

## 2. Contact is a rate, and time at risk is the only measurement in the chain

Model contact as a rate rather than a probability:

```
λ_contact = λ_ambient(service class) × discoverability × (1 − access barrier)
P(contact | T) = 1 − exp(−λ_contact · T)
```

gated by reachability: no confirmed responding service, no external contact. A DNS record is not exposure, and treating it as such is the most common overstatement in attack-surface tooling.

Four factors are estimates. The fifth, `T`, is not. **Time at risk is measured.** Trawl knows, to the hour, how long a service was reachable, because it looked, repeatedly, and wrote down what it saw.

That deserves emphasis, because in a chain otherwise built from published base rates and bounded judgment, one term is a direct observation of the organisation's own history. "This administrative interface was internet-reachable for forty-one days" is not an estimate anyone can argue with. It is a fact with a timestamp.

Three honesty rules keep it a fact:

- **Censoring is declared.** An asset exposed before you started watching has a left-censored duration. Report "at least forty-one days".
- **Detection latency is included.** Time at risk starts at first *possible* exposure, not first *observed* exposure. Hourly monitoring adds an hour of uncertainty; quarterly assessment adds up to ninety days. The cadence is part of the measurement.
- **Open exposures keep accruing.** An unremediated finding's contact probability rises every day it stays open. Most tools freeze the number at the last scan; the number should be alive.

This is also where continuous monitoring earns a *quantified* advantage over point-in-time assessment, rather than an asserted one. More on that in §5.

---

## 3. The rule that must not be broken: exposure is a prior, not an observation of attackers

The framework is explicit that contact frequency "is never inferred from scanner findings," and it is right. A scanner seeing an open port has not seen an attacker. Conflating the two would let a tool manufacture evidence of attack from evidence of existence, which is precisely the sin that discredits vendor risk scores.

So the distinction has to be structural, not cultural:

- **Exposure model → a prior on contact.** This is the "network exposure and attack-surface analysis" the framework names as its external supplier. Legitimate, labelled `sourced`.
- **Contact evidence → an update to that prior.** Requires observations of actual attacker contact: honeypot and canary hits, WAF and CDN request logs, authentication logs, DMARC aggregate report volumes.

In Trawl these are two separate stored quantities, and no scan-derived code path can write the second. It is a compile-time and CI-enforced separation, not a convention, because conventions of this kind erode in month eight of a delivery under pressure.

---

## 4. The third evidence class

Here is the extension, and it came out of a simple observation: **a DMARC policy of `p=none` is informative in a way the framework's two layers do not quite capture.**

The framework has:

- **Layer 2** — instance adjustments. Named, directional, bounded shifts on the log-odds. Explicitly: *"Each adjustment moves only the probability, not the uncertainty width."*
- **Layer 3** — Bayesian updates from your own telemetry. Beta-Binomial conjugate updates from counted observations, weighted by sensor reliability.

Layer 2's rule is right *for what it describes*. A human asserting "this is a production credential, not a test key" adds no precision. It is a claim about the instance, and claims move means.

But `p=none` is not a claim. It is a **verified measurement of the mechanism that determines the outcome**. So is `~all` rather than `-all`. So is an absent MTA-STS policy, an unsigned zone, a wildcard record answering for everything, an administrative port answering from the public internet. Nobody asserted these. A sensor went and looked, and the thing it looked at is the causal machinery itself.

That is a third class:

| | Layer 2 — judgment | **Layer 2.5 — measured state** | Layer 3 — outcome telemetry |
|---|---|---|---|
| Observes | an asserted property | the **mechanism**, directly | **realisations** of the event |
| Example | "this is a production credential" | `p=none`, `~all`, no MTA-STS, port 3389 open | rua counts, canary fire, IR confirmation |
| Statistical role | covariate, asserted | covariate, **verified** | outcome, counted |
| Mechanism | logit shift | logit shift **+ precision gain** | conjugate update |
| Moves the mean | yes | yes | yes |
| Moves the width | **no** | **yes** — capped, decaying | yes, uncapped |
| Availability | always | **always, continuously, free** | rarely, only if instrumented |

The final row is the practical argument. Outcome telemetry is strictly the stronger evidence, and most organisations do not have it. A framework that narrows uncertainty *only* on telemetry leaves nearly every estimate at class width indefinitely — which is an accurate description of the state of the art, and not a happy one.

Measured state is the evidence everybody can have, on every asset, refreshed continuously, at no marginal cost.

### Why it legitimately narrows

Law of total variance:

```
Var(θ) = E[ Var(θ | S) ]  +  Var( E[θ | S] )
         within-component     between-component
```

A class prior's width is partly **heterogeneity of the population the source study drew from**. Some of the domains behind a published spoofing base rate enforce DMARC; some publish nothing at all. That mixture is baked into the spread. Observing which sub-population you are in retires the between-component — it was never your uncertainty in the first place.

The board-legible version:

> "The spread in the industry number partly reflects that some firms enforce DMARC and some don't. We know which we are. That part of the spread isn't our uncertainty — it's theirs."

Nothing was assumed. No outcome was observed. And the estimate is legitimately tighter.

Operationally, a precision-gain factor on effective sample size:

```
g        = Var_class / E[Var | S]
ESS_cond = (ESS_class + 1) · g − 1
```

If observing policy disposition removes 40% of the class variance, `g ≈ 1.67`, and an ESS-6 prior behaves like ESS ≈ 10.7. The interval tightens materially; the mean moves by the ordinary logit adjustment. One line of arithmetic, checkable by hand — the same standard the rest of the method holds itself to.

### Four guardrails, without which this is just confidence laundering

**1. Heterogeneity precondition.** You may claim `g > 1` only if the source population was genuinely mixed in that signal. If the study already conditioned on DMARC policy, observing it tells you nothing new and the variance share is zero. Every claim carries a written argument, so it can be attacked.

**2. Deduplication binds on variance, not only on the mean.** The framework forbids counting EPSS and "a Metasploit module exists" twice, because EPSS already ingests exploit availability. The same discipline must apply to precision. SPF `-all` and DMARC `p=reject` are correlated indicators of one underlying maturity; they are not two independent narrowings. Signals belong to dedup groups, and a group claims its share once — the maximum, not the sum.

**3. A hard cap, `g ≤ 2`.** Measured state may at most halve variance. The reason is not squeamishness: you cannot learn more from a mechanism's configuration than the class study that priced that mechanism contained, and there remains irreducible uncertainty about the class prior itself that no amount of scanning touches. The cap enforces a floor on humility.

**4. Repetition is not accumulation.** Scanning the same domain five hundred times does not yield five hundred observations. Repeated measurement of a static configuration is evidence that the state is *stable*, not new evidence about susceptibility. Persistence buys exactly two things: freshness stays at full weight, and the probability of undetected drift between scans falls. It must never inflate effective sample size. In the implementation, there is simply no field in which such an accumulation could be expressed — the safest place to prevent an error is where it cannot be written down.

---

## 5. Freshness decay, and the value of continuous monitoring

Measured state is a timestamped measurement, so its precision gain should decay:

```
g_effective = 1 + (g − 1) · exp(−Δt / τ)
```

with `τ` per signal class. A DNS policy record is stable for months. An open port, a cloud storage ACL, a certificate is not.

This gives three things at once.

**A quantified argument for continuous monitoring.** The annual penetration test's evidence has decayed to nearly nothing by the third quarter. The continuous sensor's has not. That is now a number in a model rather than a claim in a sales deck — and it is the honest form of the claim, because it also concedes that a fresh annual test in month one is worth exactly as much as a fresh continuous observation in month one.

**Correct behaviour for gaps.** No observation, no gain; the interval widens back toward class width over time. `not_checked` and `check_failed` are thereby structurally incapable of masquerading as reassurance.

**A reason to keep the pack honest.** Parameters carry retrieval dates and participate in the same reasoning. A model quoting a 2023 base rate in 2026 says so.

That last point generalises into a discipline worth stating separately: **"we did not look" must never render as "it is fine."** Vantage draws this distinction at the source — `ok`, `not_found`, `not_checked`, `check_failed` — with the observation that "no record published" and "we could not tell" are different statements. For a *risk* model the inverse error is the dangerous one: a check that never ran must never contribute the reassurance of a check that passed. Every aggregate therefore carries an assessment-coverage figure, and every number is quoted with it.

---

## 5b. Persistence upholds the assumption it cannot strengthen

Section 4's fourth guardrail says repetition is not accumulation: scanning a static configuration five hundred times is not five hundred observations, and it must never inflate effective sample size. That guardrail is right, and it is also easy to over-read. It rules out one flow of inference. It does not rule out all of them, and the flow it leaves open is the one that matters most.

The distinction is *which quantity the repetition is evidence about*.

Repeated observation is not evidence about **susceptibility** — the conditional probabilities of attempt, success and impact given contact. Those live in Layer 1 and Layer 3, they are properties of how the world responds to an attack, and no amount of looking at your own DNS records tells you more about them. Confusing the two is exactly how a scanner talks itself into false precision.

Repeated observation *is* evidence about **state**, and state includes the one measured term in the whole chain. Section 2's contact model is `P(contact | T) = 1 − exp(−λ · T)`, and `T` is time at risk. Without continuous observation, `T` is not measured; it is *assumed*. You saw the weakness on 3rd March, you see it again on 3rd September, and everything between is interpolation — you are asserting six months of exposure on the strength of two data points and an assumption that nothing changed in the gap. With continuous observation, each assessment converts an interval of *assumed* exposure into an *observed* one. The window is not inferred to have stayed open. It is watched staying open.

So persistence does not sharpen the estimate. It sustains the premise the estimate is conditioned on.

This is the mirror of Section 5's decay, and it falls out of the same equation. Decay says an observation's evidential weight ages: `g_eff = 1 + (g − 1) · exp(−Δt / τ)`. Re-observation resets `Δt` to zero. Nothing accumulates — `g` is bounded by the same cap it always was — but the decay never gets started. The annual test's evidence is decaying from the day it is filed. The continuous sensor's is held at the top of the curve. Both statements are consequences of one formula, which is the property that makes it worth trusting.

There is a reason this is legitimate rather than a bookkeeping trick, and it is worth being explicit about because it is where the analogy to continuous control validation earns its keep. **Each re-observation is an opportunity for the assumption to be falsified.** The check could have come back `ok`. The record could have been fixed, the port closed, the bucket locked. It was not. The exposure assumption is upheld not because it was repeated but because it survived a test it could have failed. Repeating an unfalsifiable claim adds nothing; repeating a falsifiable check that keeps not failing is a live test that keeps passing.

The same argument is what continuous control validation actually buys, and Section 9 uses it directly. A control's measured coverage decays for precisely the reason a scan's does — configurations drift, exceptions get granted, an agent stops reporting. Continuous validation does not make the control more effective, and it must not be allowed to inflate the efficacy estimate. It holds the coverage figure up against decay, and it does so by repeatedly giving the figure a chance to fall.

Two consequences the model has to honour.

**Gaps must be visible as gaps.** If observation lapses and resumes, the intervening period is inferred, not measured, and it has to be recorded as such. The naive implementation — first-seen to last-seen, treated as one continuous open window — silently launders an assumption into a measurement. Time at risk therefore accrues over *observed* intervals, and unobserved intervals are carried separately and disclosed. An estate monitored quarterly and an estate monitored hourly may report the same `T`; they have not earned it the same way, and the board is entitled to know which one it is looking at.

**Closure needs the same evidence standard as exposure.** A weakness that stops being observed has not been remediated; it has stopped being observed. Only an assessment that *ran* and came back clean closes a window. `not_checked` and `check_failed` leave it open, and the interval stays in the inferred column. This is the "we did not look must never render as it is fine" rule, applied to the time axis rather than the coverage axis.

The compact statement, which is the one to put in front of an executive: **continuous monitoring does not tell you more about how bad a weakness is. It tells you, rather than assumes, how long you have had it.** Under `1 − exp(−λ · T)`, that is not the smaller of the two questions.

---

## 5c. Detection latency is the term you actually control

Section 5b is about weaknesses you already know of. The more interesting case is the one you do not, because that is where the model is not merely uninformed but *wrong*.

Split time at risk at the moment of detection:

```
T = T_blind + T_aware
```

Risk accrues over all of `T`. Your stated exposure only tracks `T_aware`. During `T_blind` the estate is not carrying an unknown risk — it is carrying a **known-to-be-false low estimate**, which is a worse thing to own, because it has been signed off. The error is systematic and one-directional: an undetected weakness never makes your risk position look worse than it is.

That gives detection latency a precise cost. It is not "we found out late." It is *the integral of the gap between your stated position and your actual one*, and it is the only component of the chain a monitoring decision directly moves.

**Cadence sets the expected latency, and the arithmetic is embarrassingly simple.** For a weakness introduced at a uniformly random point between assessments on a cadence `C`, expected blind time is `C/2`, and the worst case is `C`. Quarterly assessment: forty-five days of expected blindness per introduction, ninety at worst. Daily: twelve hours. This is not a modelling subtlety, it is division, and it is the number that should sit next to the monitoring line item in the budget. Under `1 − exp(−λ · T)`, halving `C` removes `C/4` of expected exposure from every weakness the estate introduces over the year — and the estate's introduction rate is itself measurable from the ledger, so the annual figure is computable rather than rhetorical.

**Look at which terms belong to whom.** The contact rate is `λ_contact = λ_ambient · D · (1 − B)`, and `P = 1 − exp(−λ_contact · T)`. Of these, `λ_ambient` is the adversary's parameter. You do not set it, you do not negotiate with it, you can only observe it and be honest about which vintage of it you are quoting. The defender's terms are `D` (discoverability), `B` (access barrier) and `T`. Detection latency is the only lever on `T` that does not require anyone to fix anything first.

### What AI changed, and what it did not

The honest version of this argument is narrower than the marketing version, so it is worth separating what actually moved.

Two things did. **Discovery cost fell**, which pushes `λ_ambient` up — less of the internet is obscure, obscurity was always a weak `D` term, and it is weakening further. And **the interval between disclosure and working exploitation compressed**, which is a change in the *shape* of `λ` over the life of a weakness: the old model, in which a newly published weakness enjoyed a grace period while exploitation was developed, is a poorer approximation than it was.

One thing did not. **None of this changes any conditional probability in Layer 1 or Layer 3.** Given contact, whether an unauthenticated service yields is a property of the service, not of the attacker's tooling. It is tempting — and it would be wrong, and it is precisely the error guardrail 4 exists to prevent — to let "AI changed everything" leak into susceptibility estimates. It changed the rate of arrival, not the physics of the door.

That containment is what makes the claim tractable. A rise in `λ_ambient` propagates through exactly one term, and the consequence is arithmetic rather than atmosphere:

> Under `P = 1 − exp(−λ · T)`, if `λ` rises, the exposure a given cadence buys you falls. A quarterly cadence that was defensible at one arrival rate is not defensible at three times that rate, and *nothing about the estate has to change for the posture to have degraded.*

That is the sentence that does the work. It means "we have not changed anything" is not a defence — it is the failure mode. It also means the model must version and date `λ_ambient` in the packs (Section 8) rather than embedding it as a constant, and that a pack update alone can legitimately move the board's numbers. When it does, the reason has to be legible: **the world got faster** is a different report line from **our estate got worse**, and conflating them destroys the model's credibility in exactly one board meeting.

### The trap: better instrumentation makes risk go up

Here is the part that will bite, and it needs to be said *before* anyone buys the monitoring, not after.

Improve detection and the reported risk position gets **worse**. `T_blind` collapses into `T_aware`, weaknesses that were being carried silently become carried visibly, and the exposure number rises. Nothing in the estate deteriorated. The instrument got better.

Every incentive at this point is wrong. The team that instruments heavily reports higher exposure than the team that does not look, and if the reporting cannot distinguish the two, the rational move is to look less. A risk model that punishes measurement will be gamed, and it will deserve to be.

The fix is the one Section 7 already insists on for disagreements: **name the cause of the change.** A movement in the reported position is either

- a **world change** — a weakness genuinely introduced, a control genuinely relaxed; or
- a **belief correction** — the same world, better observed.

These are different events, they have different owners, and they must never be summed into one trend line. A belief correction is not a deterioration in security; it is a deterioration in the *previous report*, and the previous report is what was wrong. Presented that way it becomes an argument for the instrumentation rather than against it — the increase is the size of the error you were previously carrying unknowingly, which is the most direct evidence anyone will ever get of what the monitoring was worth.

This also supplies the retrospective form of the value-of-information argument in Section 8. Section 8 ranks instrumentation by *expected* uncertainty reduction. Belief corrections measure what it *actually* delivered. A sensor that has never produced a belief correction is either watching a stable corner of the estate or is not working, and the ledger should be able to tell you which.

### Situational awareness is the honest name for all of this

Strip the machinery away and the claim is modest. None of it predicts an attack. What it does is keep a stated position in sync with a real one, and bound how far the two can drift before something forces a correction.

That framing also fixes where the value sits, which matters because it is easy to oversell. Monitoring pays in two currencies, and only one of them is automatic:

- **Better decisions, immediately.** The estimate stops being wrong. This lands the moment the sensor reports, and it lands whether or not anyone acts.
- **Lower `T`, but only via remediation.** Detection shortens exposure *only if something follows it*. Detection latency and remediation latency are separate terms, and buying monitoring while leaving the queue unworked shrinks your ignorance and nothing else.

Both belong in the ledger, separately. An organisation with a two-day detection latency and a ninety-day remediation queue has bought the first currency and none of the second, and it should be able to see that about itself — because the fix is then obviously not more scanning.

---

## 5d. Detection produces order, not work

The objection to everything in 5c is not intellectual, it is operational, and it is entirely legitimate: an engineering team with a backlog it cannot clear does not experience better detection as better information. It experiences it as more work. Every incentive points at looking less, and no amount of Bayesian argument survives contact with a team that is already underwater.

The answer is not to tell them the feeling is irrational. The answer is that the premise underneath it is wrong.

**Detection does not create work. The work already existed.** The weakness was there. The exposure was being carried. The loss distribution was what it was. Nothing about the estate's risk changed when the scanner reported — only the *visibility* of it changed. What detection actually produces is not a longer queue. It is a **correctly ordered** one.

That reframes the choice. It is not between more work and less work; the volume of latent weakness is set by the estate, not by the sensor. It is between **ordered work and arbitrarily ordered work**, at identical cost. And that is a choice no overwhelmed team should want to lose, because it is precisely the constraint of limited capacity that makes ordering valuable. A team with infinite capacity does not need prioritisation. A team with none needs it more than anyone.

### Unaccounted is not the same as unprioritised

There is a distinction here that the whole argument rests on, and it is worth being pedantic about.

A **known, deprioritised** weakness is priced, ranked, and deliberately not being worked. Its cost is in the model. The board's expected loss figure includes it. Someone decided.

An **unknown** weakness is priced at **zero**. Not "unpriced" — zero is a number, it is in the sum, and it is wrong. This is the substance of the dice observation: the adversary samples from the true distribution regardless of what your register says. Your backlog ordering does not appear anywhere in `λ`. The world is indifferent to whether you got to it.

So `T_blind` is not "risk we have not reached yet." It is **risk we asserted was absent.** The difference between those two is the difference between a queue and a false statement, and only one of them is a governance position you can defend.

Which produces the reframe that actually helps the overwhelmed team:

> **A finding does not obligate you to fix it. It obligates you to decide about it.**

The unexamined assumption driving the head-in-the-sand incentive is that detection implies remediation — that every finding is a debt. It is not. Most findings should terminate in an explicit, priced, dated, owned **acceptance**, and that is not a failure state, it is the correct output. Deciding is cheap. Fixing is expensive. Conflating them is what makes detection feel like punishment.

And note what this transformation costs in remediation capacity: nothing. You cannot *accept* a risk you do not know about, because acceptance requires knowledge. Detection converts unacceptable-because-unknown into accepted-because-decided, and it does so without a single engineering hour. That is real governance value delivered entirely outside the constrained resource.

### The prioritisation result

Now the constrained problem. Capacity fixes `k` items per period. Risk reduction depends entirely on *which* `k`. So:

> **The value of a detection is the displacement it causes in the queue, not the count it adds to the backlog.**

If a scan finds a hundred items and none outrank the existing top `k`, that scan changed nothing this period — and that is an honest, checkable conclusion, not a failure. If it finds one item that displaces the top of the queue, the scan paid for itself in that single item.

This immediately condemns the metrics the industry actually uses. *Findings discovered* and *findings closed* are volume measures, and volume measures are exactly what create the perverse incentive: they make detection look like cost and closure look like progress regardless of what was closed. **Queue displacement rate** — how often new detection changes what gets worked — inverts the incentive, because a sensor that never displaces the queue is either watching a stable corner of the estate or is aimed at the wrong thing, and either way the finding is about the *sensor*, not the team.

The ranking key is `Δ E[loss] / unit remediation cost`, computed through the chain the framework already gives us, not from a severity label. Several consequences fall out that will irritate people, which is usually a sign the model is doing work:

- A **critical on an asset behind a strong access barrier can rank below a medium on an exposed one**, because `B` enters as `(1 − B)` in the contact rate. Severity is a property of the weakness; risk is a property of the weakness *in position*.
- An **old medium can outrank a new critical**, because `T` is in `1 − exp(−λT)` and forty days of accrued exposure is not the same as two.
- **Fifteen instances is not fifteen times one instance** — aggregation is `1 − Π(1 − Pᵢ)`, which saturates, so the tenth instance of a thing is worth much less than the first.
- Most pointedly: **fixing a high-severity weakness with negligible contact probability retires almost no risk.** It is tidying. It may still be worth doing for other reasons, but those reasons must be stated rather than smuggled in under the word "critical."

This is what "material susceptibility that denies threat actors" means once it is made computable: rank by the reduction achieved in the product, not by the alarm level of the label.

### Where denial is cheapest, stated correctly

There is a tempting claim here that is wrong, and it is worth killing precisely because it sounds right.

The tempting claim: act early in the chain, because contact-stage controls stop everything downstream. But the chain is **multiplicative**. A 50% reduction at the contact stage and a 50% reduction at the success stage have *identical* effect on that scenario's expected loss. There is no arithmetic advantage to earliness.

The real advantage is **breadth of reuse**. Contact-stage terms are properties of an asset's exposure, and the same asset participates in many scenarios — so one reduction is credited across all of them. Success-stage terms are typically scenario-specific. Early-stage denial therefore wins on *how many places the same intervention counts*, not on where in the sequence it sits. That distinction matters because it is testable: the model can compute the breadth, and where a late-stage control happens to be shared across many scenarios it should and will rank above an early-stage one that is not. "Shift left" is a heuristic that this model can replace with a number, and sometimes contradict.

### The counterweight

This argument is abusable, and the abuse is obvious: *we ranked it low, therefore we are fine.* Three constraints keep it honest.

**Accepted risk must be summed and shown.** A register of individually-reasonable acceptances whose aggregate is never totalled is a backlog with better vocabulary. The sum belongs on the board's page, as currency, next to the risk being actively retired.

**Acceptance has a half-life.** This is not a policy nicety, it is forced by the model: rank depends on `T`, and `T` grows on its own. An item correctly accepted at thirty days may breach the threshold at four hundred with nothing else having changed. Acceptances must therefore be re-evaluated on the model's terms and automatically re-raised when their rank moves materially — otherwise the acceptance decision silently becomes a permanent one it was never authorised to be.

**Capacity itself must be visible as the constraint it is.** If the queue's top items are stable and unworked period after period, the problem is not detection, prioritisation or scoring. It is capacity, and the model should say so plainly rather than allowing better ranking to disguise a resourcing decision that belongs to the board. The three dials in Section 12 tell you how bad, how sure and how fresh. This is the fourth question — *how much can we actually do* — and it is the only one where the answer is a budget rather than an estimate.

The compact form, for the team rather than the board: **you are not being asked to fix more. You are being asked to fix the right ones, and to say out loud what you are not fixing.**

---

## 5e. The incentives, and the perennial capacity conversation

Everything above only matters if it changes what an organisation rewards. So it is worth being explicit about the incentive structure, because the current one is actively hostile to good practice and most of the damage comes from four metrics that look reasonable.

### The metrics audit

| Metric | Behaviour it actually rewards |
|---|---|
| Findings discovered | Looking less. Every detection worsens the number. |
| Findings closed | Closing trivia. A hundred cosmetic items beats one exposed database. |
| Mean time to remediate | Fixing easy things quickly — and *not detecting hard things at all*, since an undetected item never enters the average. |
| Backlog size | Head in the sand, again, plus deletion of stale-but-real findings. |

The MTTR one deserves a moment, because it is the most respectable of the four and the most corrosive. **MTTR improves if you never detect the difficult ones.** A team optimising it rationally avoids instrumenting the messy, legacy, hard-to-fix corners of the estate — which is precisely where the exposure lives.

The replacements follow from Sections 5c and 5d, and each is chosen because gaming it requires doing the right thing:

- **Loss retired per unit of capacity.** Gaming it means finding cheaper high-impact fixes.
- **Queue displacement rate.** Gaming it means aiming sensors where they change decisions.
- **Total accepted risk, priced.** Gaming it means either fixing things or being visibly honest.
- **Shadow price of capacity.** Gaming it means arguing for resources with arithmetic.

### Reprioritisation is work, and it is often the highest-return work available

Blind burndown — oldest first, easiest first, whatever the ticketing tool surfaces — optimises the number of items removed. Nothing in the estate cares about the number of items removed.

Take two teams with identical capacity. One burns down in arrival order; the other re-ranks each period and works the head of the ranked queue. The second retires strictly more expected loss, at identical cost. The difference is not effort. It is ordering.

Which means **re-ranking competes with remediation for capacity, and frequently wins.** When a queue is badly ordered, an hour spent re-ranking moves more expected loss than an hour spent fixing. That is an uncomfortable thing to say to a team that measures itself by throughput, and it is the single most fundable observation in this document, because re-ranking is cheap, requires no change window, breaks nothing, and can be done by someone who is not the scarce engineer.

But it has a limit, and pretending otherwise would repeat the mistake this whole document is trying to avoid. **A queue that re-ranks continuously never finishes anything.** Work in progress gets abandoned mid-flight, context is lost, and the throughput cost is real. So the head of the queue is committed for a window; re-ranking happens at the window boundary; and only a material breach — an item whose computed loss jumps above the committed head — interrupts within a window. Ranking stability is therefore a property to be *measured*, not maximised: a ranking that never changes is not seeing anything new, and one that changes constantly is not letting anything complete.

### A growing backlog is not a failure

This is the part to say directly to engineering, because it is arithmetic and it is liberating.

If weaknesses are introduced faster than fixed capacity can remove them, **the backlog must grow.** That is not a performance signal, it is a consequence of an introduction rate and a capacity, neither of which the team controls. Managing a team against backlog length is managing them against subtraction they were never resourced to do.

The substitution that makes this workable:

> **A backlog is an inventory, not a debt. What matters is the price of it, not the length of it.**

And the two can move in opposite directions, which is the whole point. Count can rise while priced risk falls — a team that clears the exposed database and accumulates forty cosmetic findings has a longer backlog and a materially better position. Reporting length alone makes that team look worse. Reporting price makes it look like what it is.

So the questions worth asking about a backlog are not about its size:

- Is the **risk-weighted** total rising or falling?
- Is the **head** being worked, or is it stable and untouched?
- When detection **displaces** the head, does the plan actually change?

A growing backlog with a falling priced total, a worked head and responsive displacement is a well-run programme. It will look terrible on every conventional dashboard.

### Capacity as the optimisation it has always been

Stated plainly, the remediation problem is: maximise total expected loss retired, subject to total remediation cost not exceeding capacity. A knapsack. Two consequences matter more than the formalism.

**Cheapness is a first-class property.** Ranking by reduction *per unit cost* means a trivially cheap fix with modest impact can and should outrank an expensive one with larger impact. Engineers know this intuitively and are routinely overruled by severity labels. The model backs them.

**The dual is the number the organisation actually wants.** The shadow price of capacity — the expected loss reduction available from one more unit of remediation capacity, at the current margin — is the direct, quantitative answer to *should we fund another engineer, or a contractor, or a quarter of hardening work?* It declines as capacity grows, which is exactly the property needed, because it tells you where more capacity **stops paying**. A security function that can say

> "The next engineer-week retires £X of expected loss. The tenth retires £Y. Below £Z it is no longer the best use of the money."

is having a different conversation from one asking for headcount because the backlog is scary.

### Features versus fixes, honestly

This is the perennial trade-off, and it is worth being careful here, because the temptation is to overclaim and the overclaim would be caught immediately.

The model does **not** tell you whether to build the feature. Feature value estimates are usually worse calibrated than these risk estimates, and putting two numbers side by side does not make them commensurable just because both are in currency.

What it does is narrower and, I think, more useful: **it supplies the price of not fixing.** That term was previously absent from the conversation entirely, represented instead by an adjective. The trade-off was never features versus fixes. It was features versus an unpriced fear — and an unpriced fear loses every argument it is in, whatever its actual size, because a roadmap item has a business case and a severity label does not.

Two things follow, and the second is the one that de-escalates the conflict.

**The comparison becomes explicit rather than implicit.** Deferring the fix is still allowed. It is now a decision with a number, a date and an owner, recorded as accepted risk and totalled on the board's page. Nobody is prevented from shipping. They are prevented from shipping *silently*.

**The decision does not change hands.** The product owner or the board still decides. Security supplies the price; it does not win the argument, and should stop trying to. That reframing matters more than any equation in this document:

> Security stops being the department of no and becomes the department of price.

The department of no is structurally doomed — it has authority nobody granted it and evidence nobody accepts. The department of price has neither problem, because it is not asking anyone to do anything. It is making one term of an existing decision visible.

### The obvious gaming risk, and the one control that closes it

If a team both estimates expected loss and is measured on loss retired, it can inflate the value of whatever it happens to have fixed. This is a real risk and it is not solved by good intentions.

It is solved structurally: **the price must be fixed before the work is selected, not after it is done.** Credit for a remediation is computed from the ranking parameters and pack version that were in force when the item entered the committed queue head, recorded at commitment time. Retrospective revaluation of completed work is not permitted, and any change in the underlying parameters shows up — correctly — as a parameter change under Section 5c's attribution rules rather than as loss retired.

Which is the same discipline as everywhere else here. Say what you expect, then record what happened, and never let the second quietly rewrite the first.

---

## 6. The selection effect, and why stage attribution saves you

Here is the trap, and it is a good one.

A domain at `p=reject` with reporting enabled shows very few *successfully delivered* forged messages. Naively, that reads as low susceptibility. It is nothing of the kind: it is evidence the control is working. Feed it in carelessly and a CISO concludes "we enforce DMARC, so nobody targets us" — exactly backwards, and the kind of inference that ends up in a post-incident review.

Correct stage attribution dissolves it:

| Signal | Stage | Class |
|---|---|---|
| rua aggregate volumes — messages failing authentication | **Attempt** | outcome (counted; policy-independent) |
| ruf forensic samples | **Attempt**, higher fidelity | outcome |
| DMARC disposition `none` / `quarantine` / `reject` | **Success** | measured state |
| `sp` subdomain policy, `pct` rollout, per-domain coverage | **Success** | measured state |
| SPF `~all` vs `-all`, DKIM key presence and strength | **Success** | measured state |
| MX absent | **Applicability** | measured state |

Attackers attempt spoofing whether or not you reject. Disposition governs what happens *after* the attempt. Report volumes therefore give attempt-stage counts independent of policy, and disposition gives success-stage state. The selection effect vanishes, not by adjustment, but by putting each signal where it belongs.

The general rule: **every signal declares exactly one consuming stage.** A signal claimed by two stages is a build failure. This is the same discipline as the EPSS deduplication rule, applied across stages rather than within one.

### DMARC reports are the most under-used Layer 3 source in existence

Worth saying plainly. Aggregate reports are a daily, free, counted record of attempted abuse of your domains — genuine `k out of n` observations, arriving on their own, at organisations that believe they have no attack telemetry. Most enterprises either do not publish a `rua` address, or publish one into a mailbox nobody parses.

The framework says one canary changes the picture, because it upgrades a stage estimate from a category starting point to an observation from your own environment. Aggregate reports are the same upgrade, already paid for, waiting in a DNS record most domains do not have.

---

## 7. Disagreement is a finding, not an averaging problem

When outcome evidence contradicts measured state — `p=reject` published, yet reports show forged mail being delivered — the wrong move is to average them into a compromise number.

The right move is two moves:

1. **Telemetry governs the estimate.** It observes outcomes; state observes only mechanism.
2. **The divergence is raised as a control-efficacy finding.** Usually the cause is an enforcement gap: a missing subdomain policy, a rollout percentage below 100, an unenrolled sending domain, a lookalike outside the assessed set.

This falls out for free once both evidence classes live in one ledger, and it converts the risk model into a detector. A model that only produces numbers is a reporting tool. A model that notices when its own inputs disagree is an instrument.

---

## 8. Uncertainty as a first-class object, and the value of information

Once uncertainty is a stored quantity rather than a caveat, a new question becomes askable: **which missing sensor would most narrow the interval currently driving the decision?**

The portfolio-ROI method already brushes against this. When the top two controls' return intervals overlap, it observes that the overlap is itself decision information — collect more cost data, or fund both. Generalise that and you get a ranking of *instrumentation* alongside the ranking of controls, priced the same way: expected narrowing per pound, with a flag for whether the narrowing could actually change the funding order.

The output reads like this:

> "Three of your scenarios rest on class priors because you have no attempt telemetry. Publishing report addresses on nineteen domains — no licence cost, one DNS change — would begin outcome-updating the business-email-compromise attempt stage within thirty days."

Uncertainty reduction that cannot change a decision ranks below uncertainty reduction that can. That distinction is what keeps this from becoming measurement for its own sake.

---

## 9. Measured coverage: the control you bought versus the control you have

The portfolio method takes each control's effectiveness as an estimate. Fair — efficacy research is not something a scanner can do.

But efficacy is not the whole story. **A control only delivers where it is actually deployed**, and deployment is often externally observable: DMARC at enforcement across *n* of *m* sending domains, MTA-STS in enforce mode, zones signed, TLS configured, patch latency on internet-reachable services.

So: `effective TMP = TMP × measured coverage`, and one sentence becomes available that no purely top-down model can produce:

> "You are paying for this control and receiving 68% of it. Closing the gap costs nothing in licence terms and is worth £X a year."

Two corollaries follow immediately.

**Control drift is an ROI event.** A policy relaxed from enforcement on one sending domain is not a medium-severity ticket in a queue. It is a measurable increase in expected annual loss, and presenting it that way changes how a team triages posture regressions.

**Asserted coverage must be marked as such.** Endpoint detection, backups and awareness training are not externally observable. Those carry attested coverage with a source and a date, visually distinguished from measured coverage. The distinction is the point: a model that quietly treats an attestation and a measurement as the same kind of thing has given up the only advantage it had.

---

## 10. Crossing from findings to loss events, without cheating

The hardest honest problem in the whole chain. Bottom-up gives per-instance exploitation probabilities. Top-down gives per-scenario loss frequencies anchored to industry data and pulled by your own incident history. **A compromised asset is not a loss event**, and substituting one for the other would be indefensible.

Two bridges, both declared:

**Bridge A — exposure as a mitigation-like modifier (default).** Attack surface sits left of boom. The aggregate susceptibility of a scenario's entry-path assets acts as an exposure factor on the event rate, structurally identical to a control's threat-mitigation potential, with a **neutral baseline**: at your own established exposure level, the rate is unchanged. Only deviation moves it. The industry anchor already prices a firm operating at typical exposure; multiplying it by an absolute susceptibility figure would double-count.

This keeps the anchor doing what anchors are good at, keeps the update to one line, and makes a quarter of remediation visible in expected annual loss — which is the thing every CISO has wanted from a risk model and never got.

**Bridge B — entry-path pseudo-observation (opt-in).** Aggregate exploitation probability over the window enters the frequency update as a low-weight pseudo-observation, at or below the weight the framework assigns to vendor and lab evidence. Powerful, easy to abuse, therefore off by default, weight-capped, withdrawable, and refused entirely for scenarios whose entry path is not externally observable. An insider-misuse scenario has no external entry path, and pretending otherwise would be the worst available failure.

Both bridges are declared on every figure they touch. A board reading an expected-loss number is entitled to know whether it moved because of an incident, an industry revision, or a change in measured exposure.

---

## 11. What the ledger has to become

The framework's two-part evidence ledger — priors and adjustments in part one, updates in part two — is usually read as a reporting requirement. It should be read as a **storage model**.

If the ledger is a view rendered over a database of scores, it rots: the numbers and their justifications drift apart, and within a year the justification is archaeology. If the ledger *is* the database, then auditability is free and the chain of custody is a property of the system rather than a claim about it.

The middle evidence class adds fields, and they earn their place. Each row carries its class, its signal identifier, its observation state, its observation time, its decay constant, its log-odds delta, its claimed variance share, its dedup group, and whether a cap bound it. Outcome rows carry the counts, the sensor quality, the evidence weight and — importantly — the distribution parameters *before and after*, so any update can be re-checked in a single line without re-running the engine.

Then "remove the challenged modifier and recompute" still works in one step, and now does the right thing in both dimensions: withdrawing a measured-state row moves the mean **and widens the interval back**. In the published framework, removal only moves the mean, because Layer 2 adjustments never touched the width. Once the third class exists, withdrawal has to restore the uncertainty it removed. That is a small consequence of the extension, and a satisfying one — it means the ledger is genuinely reversible.

---

## 12. The three dials

If there is one framing to take to a board, it is this. A risk estimate has three dials, and most tools show only the first:

- **How bad** — the mean.
- **How sure** — the interval.
- **How fresh** — when anyone last actually looked.

Scanning moves the second and third continuously, for free, on every asset. Telemetry moves all three but only where you have instrumented. Judgment moves the first alone.

Which turns the quarterly board read into something more defensible than a heat map:

> The ranking is stable. The intervals are narrowing. Here is what narrowed them — measurement, not assumption.

There is a fourth question, and it is deliberately not a dial, because the answer is a budget rather than an estimate: **how much can we actually do.** Sections 5c and 5d put it there. Detection sets how much of the estate you can see, ranking sets which part of it you work, and capacity sets how much gets worked at all. Only the third is a resourcing decision, and a model that lets better detection and better ranking disguise an unresourced queue has quietly answered a board question on the board's behalf. So the read has a second half:

> This is what we retired. This is what we are knowingly carrying, priced. This is what we could work with the capacity we have.

---

## Lines worth keeping

Collected for reuse — in the posts, in the product's copy, and in the room. Grouped by the argument each one carries, with its section.

### On what a scanner is entitled to claim

> Exposure is a prior, not an observation of attackers. (§3)

> A scan tells you the door is unlocked. It does not tell you anyone tried the handle. (§3)

> "We did not look" must never render as "it is fine." (§5)

> A check that never ran must never contribute the reassurance of a check that passed. (§5)

> Measured state does not tell you what happened. It tells you which part of the reference population you belong to. (§4)

### On continuous monitoring

> Continuous monitoring does not tell you more about how bad a weakness is. It tells you, rather than assumes, how long you have had it. (§5b)

> The window is not inferred to have stayed open. It is watched staying open. (§5b)

> Persistence does not sharpen the estimate. It sustains the premise the estimate is conditioned on. (§5b)

> The exposure assumption is upheld not because it was repeated, but because it survived a test it could have failed. (§5b)

> Repeating an unfalsifiable claim adds nothing. Repeating a falsifiable check that keeps not failing is a live test that keeps passing. (§5b)

> A weakness that stops being observed has not been remediated. It has stopped being observed. (§5b)

> The annual test's evidence is decaying from the day it is filed. (§5)

### On detection latency and the adversary's clock

> During the blind interval you are not carrying an unknown risk. You are carrying a known-to-be-false low estimate that has been signed off. (§5c)

> An undetected weakness never makes your risk position look worse than it is. (§5c)

> Quarterly assessment is forty-five days of expected blindness per weakness introduced. That is division, not modelling. (§5c)

> The ambient contact rate is the adversary's parameter. You do not set it and you do not negotiate with it. (§5c)

> If λ rises, the exposure a given cadence buys you falls — and nothing about the estate has to change for the posture to have degraded. (§5c)

> "We have not changed anything" is not a defence. It is the failure mode. (§5c)

> AI changed the rate of arrival, not the physics of the door. (§5c)

> The world got faster is a different report line from our estate got worse. (§5c)

### On the measurement trap

> Improve the instrument and the reported risk goes up. Nothing deteriorated. (§5c)

> A belief correction is not a deterioration in security. It is a deterioration in the previous report, and the previous report is what was wrong. (§5c)

> A risk model that punishes measurement will be gamed, and it will deserve to be. (§5c)

> The increase is the size of the error you were previously carrying unknowingly — the most direct evidence anyone will get of what the monitoring was worth. (§5c)

> A sensor that has never produced a belief correction is either watching a stable corner of the estate or is not working. (§5c)

### On prioritisation

> Detection does not create work. The work already existed. (§5d)

> The choice is not between more work and less work. It is between ordered work and arbitrarily ordered work, at identical cost. (§5d)

> A team with infinite capacity does not need prioritisation. A team with none needs it more than anyone. (§5d)

> An unknown weakness is priced at zero. Zero is a number, it is in the sum, and it is wrong. (§5d)

> Threat actors sample from the true distribution. Your backlog ordering does not appear anywhere in λ. (§5d)

> T_blind is not risk we have not reached yet. It is risk we asserted was absent. (§5d)

> A finding does not obligate you to fix it. It obligates you to decide about it. (§5d)

> You cannot accept a risk you do not know about, because acceptance requires knowledge. (§5d)

> The value of a detection is the displacement it causes in the queue, not the count it adds to the backlog. (§5d)

> Severity is a property of the weakness. Risk is a property of the weakness in position. (§5d)

> Fixing a high-severity weakness with negligible contact probability is tidying. (§5d)

> Shift left is a heuristic this model can replace with a number, and sometimes contradict. (§5d)

> A register of individually-reasonable acceptances whose aggregate is never totalled is a backlog with better vocabulary. (§5d)

> Acceptance has a half-life, because time at risk grows on its own. (§5d)

> You are not being asked to fix more. You are being asked to fix the right ones, and to say out loud what you are not fixing. (§5d)

### On incentives and capacity

> MTTR improves if you never detect the difficult ones. (§5e)

> Nothing in the estate cares about the number of items removed. (§5e)

> When a queue is badly ordered, an hour spent re-ranking moves more expected loss than an hour spent fixing. (§5e)

> A queue that re-ranks continuously never finishes anything. Ranking stability is a property to be measured, not maximised. (§5e)

> A backlog is an inventory, not a debt. What matters is the price of it, not the length of it. (§5e)

> Managing a team against backlog length is managing them against subtraction they were never resourced to do. (§5e)

> A growing backlog with a falling priced total, a worked head and responsive displacement is a well-run programme. It will look terrible on every conventional dashboard. (§5e)

> The next engineer-week retires £X of expected loss. The tenth retires £Y. Below £Z it is no longer the best use of the money. (§5e)

> The trade-off was never features versus fixes. It was features versus an unpriced fear — and an unpriced fear loses every argument it is in, whatever its actual size. (§5e)

> Nobody is prevented from shipping. They are prevented from shipping silently. (§5e)

> Security stops being the department of no and becomes the department of price. (§5e)

> The department of no has authority nobody granted it and evidence nobody accepts. (§5e)

> The price must be fixed before the work is selected, not after it is done. (§5e)

> Say what you expect, then record what happened, and never let the second quietly rewrite the first. (§5e)

### On the model's own honesty

> Disagreement is a finding, not an averaging problem. (§7)

> You are paying for this control and receiving 68% of it. (§9)

> A model that quietly treats an attestation and a measurement as the same kind of thing has given up the only advantage it had. (§9)

> A compromised asset is not a loss event. (§10)

> The safest place to prevent an error is where it cannot be expressed. (§4)

> The ranking is stable. The intervals are narrowing. Here is what narrowed them — measurement, not assumption. (§12)

> How bad, how sure, how fresh — and then the question that is a budget rather than an estimate: how much can we actually do. (§12)

---

## Open questions

Genuinely open, and worth resolving before or in public.

1. **Calibrating `g` empirically.** Variance shares are currently reasoned arguments about source-population heterogeneity. With enough instances and outcomes they could be estimated. The chicken-and-egg is obvious, and the honest answer today is that these are `sourced`, not `calibrated`.
2. **Is `g ≤ 2` the right cap?** It is a defensible working heuristic, chosen for humility rather than derived. It should be published as such and revised against evidence.
3. **Contact-model calibration.** Ambient contact rates by service class rest on published mass-scan and honeypot research. Honeypot deployment alongside a monitored estate would calibrate them directly, and is the single most valuable experiment available here.
4. **Cross-instance calibration.** The framework identifies a structural obstacle: non-CVE weaknesses are instances, not categories, so there is no shared label and no common dataset. A fleet of tools recording predictions and outcomes against a shared *class* taxonomy is at least a partial route — not a solution, but an instrument that makes the problem approachable. Whether that can be done without pooling data anyone would object to pooling is unresolved.
5. **Correlated contact.** One actor sweeping an address range contacts many assets at once. Independence understates aggregate contact. The direction of the error is known and disclosed; the magnitude is not.

---

## Provenance

Extends, and does not replace:

- *The Probability Behind the Finding: Calibrated Exploit Estimates Beyond CVEs* — three-layer estimate, evidence ledger, calibration labels, EPSS at the attempt stage, deduplication.
- *Security Investment as Board Strategy: Pricing Protection on Both Sides of the Boom* — scenario as unit of analysis, the two closed-form updates, six-form overlay, marginal control pricing, the board's three decisions.

New here: the contact-probability model as a measured rate with time at risk; **Layer 2.5, measured state as an evidence class that narrows uncertainty**, with its four guardrails and freshness decay; stage attribution as the resolution of the enforcement selection effect; divergence between state and outcome as a control-efficacy finding; measured control coverage and effective mitigation; value-of-information ranking for instrumentation; and the exposure bridge with a neutral baseline.

Implementation: Trawl changes 006–012.

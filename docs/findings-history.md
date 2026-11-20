# Findings history

What Trawl records about each finding over time, how a finding is resolved, and
what the history does and does not tell you.

## What is recorded

Every assessment, service probe, scan import, repository secret scan and
threat-feed refresh is a **run**, kept in an append-only ledger with its time,
outcome and **coverage**: how many of the checks it attempted reached a
conclusion.

Every change to a finding is an **event**, also append-only:

| Event | Meaning |
|---|---|
| `first_seen` | The finding was observed for the first time. |
| `changed` | Its severity moved. |
| `priority_changed` | Its computed priority moved, usually because a feed refresh changed what is known about exploitation. |
| `resolved` | A later run concluded the check behind it and no longer raised it. |
| `reopened` | It was resolved and has been observed again. Its first-seen time is unchanged. |
| `baseline` | It was already stored when history began. See below. |

Each event carries the time, the run that observed it, the scanner library and
signal registry versions, and a **cause**:

- `estate`: the thing being assessed changed.
- `library`: the scanner library changed between observations. A severity that
  moves because Trawl was upgraded is not a change in your estate.
- `feed`: a threat-feed refresh changed what is known. The event names the feed
  snapshot.

## What a finding's identity is

A finding is the same finding across runs when it asserts the same thing:

- A measured-state signal is one signal on one asset.
- A service finding is one service on one host, port and layer.
- A template finding is one template matching one place on one asset.
- A secret is one secret in one file of one repository. The line number is not
  part of the identity, because history rewrites and unrelated edits move a
  secret without making it a different secret.

Scanning an unchanged estate twice therefore produces no new findings and no
new events.

## When a finding resolves

A finding resolves only when a **later run concluded the check that raises it**
and the check no longer raised it. If the check could not conclude (it failed,
or was not performed), the finding stays open and is shown as **not
re-assessed**, with the reason. A gap in evidence is never read as a fix.

Two cases need care:

- **DKIM.** Selectors cannot be listed from DNS, so finding none proves nothing
  unless the posture says the absence was conclusive. An inconclusive DKIM
  result never resolves anything.
- **Template (nuclei) findings.** A worker payload does not say which templates
  ran, so a template's absence is not evidence that it was re-checked. These do
  not resolve on absence.

A secret resolves when a scan of its repository that ran to the end no longer
finds it. A cancelled scan resolves nothing.

Resolved findings stop counting against a domain's control posture. Existing
installations will see posture improve at the next assessment for weaknesses
that were fixed earlier but never cleared, because nothing used to clear them.

## What history does not cover

- **Nothing before the upgrade.** Findings stored before history began each
  receive one `baseline` event at their recorded first-seen time. It says only
  that the finding was known by then. Earlier change is not reconstructed.
- **Runs are not comparable by count alone.** A run that attempted fewer checks
  will show fewer findings. Every run, trend point and comparison carries its
  coverage, and a comparison states when coverage fell.
- **Gaps are gaps.** The trend is drawn from recorded runs only. Days with no
  run are not interpolated.

## Retention

Events are never pruned. They are small, and they are the record.

Per-run summaries (the data behind trend lines) are derived, and can be pruned.
Set the setting `history_summary_retention_days` to a number of days to keep;
absent, empty or zero keeps them indefinitely. The run ledger itself is not
pruned.

Removing an asset discards its events with it, as it discards everything else
recorded against the asset. **Erase discovered data** clears the whole history.

## The explanation text

The plain-English explanation of each finding comes from reviewed text that
ships with Trawl (`config/narratives/`). It is rendered when you view a finding
and is never stored against it, so a correction reaches existing findings. It
contains no monetary figures and no probabilities of compromise, and it never
describes a control as in place when it was not assessed. The version of the
wording you were shown is recorded in the view.

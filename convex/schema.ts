import { defineSchema, defineTable } from "convex/server";
import { v } from "convex/values";

/**
 * Trawl — Convex Schema v1
 *
 * Tables match the schema sketch in openspec/changes/001-initial-build/design.md.
 * Validators on every table, as required by project conventions.
 *
 * Key design decisions:
 * - `emailAuthPosture` is separate from `findings` (no CPE/CVE, different priority function)
 * - `secretFindings` stores `redactedRef`, never raw secret values
 * - `postureSnapshots` + `regressions` back the shared posture-regression mechanism
 * - `config` holds only instance-specific values, never hardcoded defaults
 * - Findings use append-only status transitions (open → resolved → reopened), not overwrites
 */

// ─── Shared value types ────────────────────────────────────────────────────────

const assetType = v.union(
  v.literal("ip"),
  v.literal("domain"),
  v.literal("repository"),
);

const assetStatus = v.union(
  v.literal("pending"),     // Discovered, awaiting human review
  v.literal("active"),      // Approved for scanning
  v.literal("inactive"),    // Manually deactivated
  v.literal("rejected"),    // Human-rejected discovery candidate
);

const findingStatus = v.union(
  v.literal("open"),
  v.literal("resolved"),
  v.literal("reopened"),
);

const confidenceLevel = v.union(
  v.literal("high"),
  v.literal("medium"),
  v.literal("low"),
);

const priorityLevel = v.union(
  v.literal("critical"),
  v.literal("high"),
  v.literal("medium"),
  v.literal("low"),
  v.literal("info"),
);

const regressionStatus = v.union(
  v.literal("provisional"),   // First observation of degradation
  v.literal("confirmed"),     // Second consecutive observation confirms it
  v.literal("restored"),      // Attribute returned to previous (or better) value
);

const regressionDirection = v.union(
  v.literal("degraded"),
  v.literal("improved"),
);

const alertCategory = v.union(
  v.literal("new_asset"),
  v.literal("new_finding"),
  v.literal("kev_match"),
  v.literal("regression"),
  v.literal("secret_found"),
  v.literal("email_auth_issue"),
);

// ─── Tables ────────────────────────────────────────────────────────────────────

export default defineSchema({
  /**
   * Canonical asset record with lifecycle state.
   * Keyed on (type, value) for dedup — no two assets with the same type+value.
   */
  assets: defineTable({
    type: assetType,
    value: v.string(),              // IP address, domain name, or repository URL
    source: v.string(),             // How it was discovered (e.g., "seed", "subfinder", "ct-logs")
    confidence: confidenceLevel,
    status: assetStatus,
    firstSeen: v.number(),          // Unix timestamp ms
    lastSeen: v.number(),           // Unix timestamp ms
  })
    .index("by_type_value", ["type", "value"])
    .index("by_status", ["status"])
    .index("by_type_status", ["type", "status"]),

  /**
   * Scan execution records — one per scan-worker job run per asset.
   */
  scans: defineTable({
    assetId: v.id("assets"),
    jobRunId: v.string(),           // Unique per job execution
    rawOutputRef: v.optional(v.string()),  // Reference to raw output (storage or inline)
    startedAt: v.number(),
    completedAt: v.optional(v.number()),
    partial: v.boolean(),           // True if scan was interrupted/incomplete
  })
    .index("by_asset", ["assetId"])
    .index("by_job_run", ["jobRunId"]),

  /**
   * Vulnerability/exposure findings from scan results.
   * Status transitions are append-only (tracked via statusHistory).
   */
  findings: defineTable({
    assetId: v.id("assets"),
    scanId: v.optional(v.id("scans")),
    cpe: v.optional(v.string()),
    cveIds: v.array(v.string()),
    kev: v.boolean(),               // True if any CVE is in CISA KEV
    epss: v.optional(v.number()),   // Highest EPSS score among matched CVEs
    cvss: v.optional(v.number()),   // Highest CVSS score among matched CVEs
    priority: priorityLevel,        // Deterministic, pure-function output
    status: findingStatus,
    statusHistory: v.array(v.object({
      from: findingStatus,
      to: findingStatus,
      at: v.number(),
    })),
    aiAnnotation: v.optional(v.object({
      summary: v.string(),
      remediation: v.string(),
      generatedAt: v.number(),
      model: v.string(),
    })),
    dedupKey: v.string(),           // Stable identity for idempotent ingestion
    firstSeen: v.number(),
    lastSeen: v.number(),
  })
    .index("by_asset", ["assetId"])
    .index("by_priority", ["priority"])
    .index("by_status", ["status"])
    .index("by_dedup_key", ["dedupKey"])
    .index("by_kev", ["kev"]),

  /**
   * CISA KEV reference data — pulled by scheduled function.
   */
  referenceKev: defineTable({
    cveId: v.string(),
    dateAdded: v.string(),
    vendorProject: v.string(),
    product: v.string(),
    requiredAction: v.string(),
    dueDate: v.optional(v.string()),
  })
    .index("by_cve", ["cveId"]),

  /**
   * NVD CVE reference data — pulled by scheduled function.
   */
  referenceCve: defineTable({
    cveId: v.string(),
    cvss: v.optional(v.number()),
    cpeMatches: v.array(v.string()),
    publishedAt: v.string(),
    description: v.optional(v.string()),
  })
    .index("by_cve", ["cveId"]),

  /**
   * EPSS score reference data — pulled by scheduled function.
   */
  referenceEpss: defineTable({
    cveId: v.string(),
    score: v.number(),              // 0.0–1.0 probability
    updatedAt: v.string(),
  })
    .index("by_cve", ["cveId"]),

  /**
   * Email authentication posture per domain.
   * Separate from findings — no CPE/CVE, different priority function.
   */
  emailAuthPosture: defineTable({
    domainAssetId: v.id("assets"),
    spf: v.object({
      record: v.optional(v.string()),
      valid: v.boolean(),
      mechanisms: v.optional(v.array(v.string())),
    }),
    dkim: v.object({
      found: v.boolean(),
      selectors: v.optional(v.array(v.string())),
    }),
    dmarc: v.object({
      record: v.optional(v.string()),
      policy: v.optional(v.string()),       // "none" | "quarantine" | "reject"
      pct: v.optional(v.number()),
      alignment: v.optional(v.string()),
    }),
    bimi: v.optional(v.object({
      record: v.optional(v.string()),
      valid: v.boolean(),
    })),
    mtaSts: v.optional(v.object({
      found: v.boolean(),
      mode: v.optional(v.string()),
    })),
    tlsRpt: v.optional(v.object({
      record: v.optional(v.string()),
      valid: v.boolean(),
    })),
    caa: v.optional(v.object({
      records: v.optional(v.array(v.string())),
      found: v.boolean(),
    })),
    priority: priorityLevel,
    checkedAt: v.number(),
  })
    .index("by_domain", ["domainAssetId"]),

  /**
   * Secret findings from public repository scanning.
   * `redactedRef` only — raw secret values are never stored.
   */
  secretFindings: defineTable({
    repoAssetId: v.id("assets"),
    provider: v.string(),           // e.g., "aws", "github", "generic"
    verified: v.optional(v.boolean()),  // Only populated when secretVerificationEnabled
    scopeGuess: v.optional(v.string()), // Estimated scope/permissions
    filePath: v.string(),
    commitSha: v.string(),
    redactedRef: v.string(),        // Masked/hashed reference, NEVER the raw value
    priority: priorityLevel,
    status: findingStatus,
    lastScannedSha: v.string(),     // Per-repo watermark for incremental rescanning
    firstSeen: v.number(),
    lastSeen: v.number(),
  })
    .index("by_repo", ["repoAssetId"])
    .index("by_priority", ["priority"])
    .index("by_status", ["status"]),

  /**
   * Posture snapshots — every check-producing capability writes dated,
   * structured snapshots per (asset, attribute).
   */
  postureSnapshots: defineTable({
    assetId: v.id("assets"),
    attribute: v.string(),          // e.g., "tls_version", "dmarc_policy", "open_ports"
    value: v.string(),              // JSON-serialized comparable value
    capturedAt: v.number(),
  })
    .index("by_asset_attribute", ["assetId", "attribute"])
    .index("by_captured", ["capturedAt"]),

  /**
   * Regressions — diffs between consecutive snapshots.
   * Requires two consecutive confirming observations before promoting to "confirmed".
   */
  regressions: defineTable({
    assetId: v.id("assets"),
    attribute: v.string(),
    previousValue: v.string(),
    newValue: v.string(),
    direction: regressionDirection,
    category: v.string(),           // e.g., "tls_downgrade", "dmarc_weakened", "port_opened"
    status: regressionStatus,
    firstObservedAt: v.number(),
    confirmedAt: v.optional(v.number()),
    restoredAt: v.optional(v.number()),
  })
    .index("by_asset_attribute", ["assetId", "attribute"])
    .index("by_status", ["status"]),

  /**
   * Alert records — dedup-keyed to prevent double-firing.
   */
  alerts: defineTable({
    targetId: v.string(),           // Asset ID or finding ID as string
    targetType: v.union(
      v.literal("asset"),
      v.literal("finding"),
      v.literal("regression"),
      v.literal("secret"),
      v.literal("email_auth"),
    ),
    category: alertCategory,
    sentAt: v.number(),
    dedupKey: v.string(),           // Stable key to prevent duplicate alerts
    channel: v.string(),            // e.g., "slack", "webhook"
    success: v.boolean(),           // Whether delivery succeeded
  })
    .index("by_dedup", ["dedupKey"])
    .index("by_category", ["category"]),

  /**
   * Instance configuration — data, not code.
   * One document per instance. All operator-specific values live here.
   */
  config: defineTable({
    instanceName: v.string(),
    seedDomains: v.array(v.string()),
    seedCidrs: v.array(v.string()),
    seedRepos: v.array(v.string()),
    webhookUrl: v.optional(v.string()),
    staleAfterDays: v.number(),
    triageThreshold: priorityLevel,
    secretVerificationEnabled: v.boolean(),
    maxRepoCloneSizeMb: v.number(),
    aiProvider: v.object({
      baseUrl: v.string(),
      apiKey: v.optional(v.string()),  // Stored in secrets manager; optional here for local dev
      model: v.string(),
      timeoutMs: v.number(),
    }),
  }),
});

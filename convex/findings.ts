import { query, mutation, internalMutation } from "./_generated/server";
import { v } from "convex/values";

/**
 * Pure function: Deterministic priority calculation
 * Enforces guardrail 3: Priority/severity is a pure function of KEV/EPSS/CVSS.
 */
export function computePriority(kev: boolean, epss?: number, cvss?: number): "critical" | "high" | "medium" | "low" | "info" {
  if (kev) {
    return "critical"; // KEV override: active exploitation always critical
  }

  const epssVal = epss ?? 0;
  const cvssVal = cvss ?? 0;

  if (epssVal >= 0.7 || cvssVal >= 9.0) {
    return "critical";
  }
  if (epssVal >= 0.4 || cvssVal >= 7.0) {
    return "high";
  }
  if (cvssVal >= 4.0) {
    return "medium";
  }
  if (cvssVal > 0 || epssVal > 0) {
    return "low";
  }

  return "info";
}

/**
 * Pure function: Stable deduplication key generation
 */
export function buildDedupKey(assetValue: string, identifier: string): string {
  return `${assetValue.toLowerCase().trim()}::${identifier.toLowerCase().trim()}`;
}

// Query: List findings with optional severity and status filter
export const listFindings = query({
  args: {
    priority: v.optional(v.union(v.literal("critical"), v.literal("high"), v.literal("medium"), v.literal("low"), v.literal("info"))),
    status: v.optional(v.union(v.literal("open"), v.literal("resolved"), v.literal("reopened"))),
  },
  handler: async (ctx, args) => {
    let findings = await ctx.db.query("findings").collect();

    if (args.priority) {
      findings = findings.filter((f) => f.priority === args.priority);
    }
    if (args.status) {
      findings = findings.filter((f) => f.status === args.status);
    }

    return findings;
  },
});

// Mutation: Ingest raw scan payload from worker job
export const ingestScanResults = internalMutation({
  args: {
    jobRunId: v.string(),
    naabu: v.optional(v.array(v.any())),
    httpx: v.optional(v.array(v.any())),
    nuclei: v.optional(v.array(v.any())),
  },
  handler: async (ctx, args) => {
    const now = Date.now();

    // Process nuclei vulnerability findings if present
    if (args.nuclei && Array.isArray(args.nuclei)) {
      for (const item of args.nuclei) {
        if (!item || typeof item !== "object") continue;

        const host = item.host || item.matched || item.ip || "unknown";
        const templateId = item["template-id"] || item.info?.name || "unknown-template";
        const cveIds: string[] = item.info?.classification?.["cve-id"] || [];

        // Check if KEV match
        let isKev = false;
        if (cveIds.length > 0) {
          for (const cve of cveIds) {
            const kevRecord = await ctx.db
              .query("referenceKev")
              .withIndex("by_cve", (q) => q.eq("cveId", cve))
              .first();
            if (kevRecord) {
              isKev = true;
              break;
            }
          }
        }

        const cvssScore = item.info?.classification?.["cvss-score"] || item.info?.severity === "critical" ? 9.0 : 5.0;
        const priority = computePriority(isKev, undefined, cvssScore);

        // Find or create asset
        let asset = await ctx.db
          .query("assets")
          .withIndex("by_type_value", (q) => q.eq("type", "domain").eq("value", host))
          .first();

        if (!asset) {
          const assetId = await ctx.db.insert("assets", {
            type: "domain",
            value: host,
            source: "scan-worker",
            confidence: "high",
            status: "active",
            firstSeen: now,
            lastSeen: now,
          });
          asset = (await ctx.db.get(assetId))!;
        } else {
          await ctx.db.patch(asset._id, { lastSeen: now });
        }

        // Deduplication key
        const dedupKey = buildDedupKey(host, templateId);

        const existingFinding = await ctx.db
          .query("findings")
          .withIndex("by_dedup_key", (q) => q.eq("dedupKey", dedupKey))
          .first();

        if (existingFinding) {
          await ctx.db.patch(existingFinding._id, {
            lastSeen: now,
            kev: isKev || existingFinding.kev,
            priority: priority,
          });
        } else {
          await ctx.db.insert("findings", {
            assetId: asset._id,
            cveIds: cveIds,
            kev: isKev,
            cvss: cvssScore,
            priority: priority,
            status: "open",
            statusHistory: [{ from: "open", to: "open", at: now }],
            dedupKey: dedupKey,
            firstSeen: now,
            lastSeen: now,
          });
        }
      }
    }

    return { success: true, processedAt: now };
  },
});

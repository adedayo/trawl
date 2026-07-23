import { describe, it, expect } from "vitest";
import { computePriority, buildDedupKey } from "./findings";

describe("Deterministic Priority Scoring", () => {
  it("should enforce KEV override rule: active exploitation is always CRITICAL", () => {
    expect(computePriority(true, 0.1, 3.0)).toBe("critical");
    expect(computePriority(true, 0.0, 0.0)).toBe("critical");
  });

  it("should assign CRITICAL when EPSS >= 0.7 or CVSS >= 9.0", () => {
    expect(computePriority(false, 0.8, 5.0)).toBe("critical");
    expect(computePriority(false, 0.2, 9.5)).toBe("critical");
  });

  it("should assign HIGH when EPSS >= 0.4 or CVSS >= 7.0", () => {
    expect(computePriority(false, 0.5, 6.0)).toBe("high");
    expect(computePriority(false, 0.2, 7.5)).toBe("high");
  });

  it("should assign MEDIUM when CVSS >= 4.0", () => {
    expect(computePriority(false, 0.1, 5.5)).toBe("medium");
  });

  it("should assign LOW when scores exist below MEDIUM thresholds", () => {
    expect(computePriority(false, 0.05, 2.0)).toBe("low");
  });

  it("should assign INFO when no scores or KEV are present", () => {
    expect(computePriority(false, 0.0, 0.0)).toBe("info");
    expect(computePriority(false, undefined, undefined)).toBe("info");
  });
});

describe("Deduplication Key Generation", () => {
  it("should generate consistent lower-cased dedup keys", () => {
    expect(buildDedupKey("API.Example.COM", "CVE-2024-3094")).toBe("api.example.com::cve-2024-3094");
    expect(buildDedupKey("  example.com  ", "  http-title  ")).toBe("example.com::http-title");
  });
});

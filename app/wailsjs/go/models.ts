export namespace contact {
	
	export class CalibrationReportRow {
	    ExperimentID: string;
	    ServiceClass: string;
	    EventType: string;
	    EventCount: number;
	    ReachableServiceDays: number;
	    EndpointCount: number;
	    UniqueSourceCount: number;
	    RatePerServiceDay: number;
	    RateLowPerServiceDay: number;
	    RateHighPerServiceDay: number;
	    IntervalMethod: string;
	    Confidence: number;
	    ObservationStart: string;
	    ObservationEnd: string;
	    DeduplicationRule: string;
	
	    static createFrom(source: any = {}) {
	        return new CalibrationReportRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ExperimentID = source["ExperimentID"];
	        this.ServiceClass = source["ServiceClass"];
	        this.EventType = source["EventType"];
	        this.EventCount = source["EventCount"];
	        this.ReachableServiceDays = source["ReachableServiceDays"];
	        this.EndpointCount = source["EndpointCount"];
	        this.UniqueSourceCount = source["UniqueSourceCount"];
	        this.RatePerServiceDay = source["RatePerServiceDay"];
	        this.RateLowPerServiceDay = source["RateLowPerServiceDay"];
	        this.RateHighPerServiceDay = source["RateHighPerServiceDay"];
	        this.IntervalMethod = source["IntervalMethod"];
	        this.Confidence = source["Confidence"];
	        this.ObservationStart = source["ObservationStart"];
	        this.ObservationEnd = source["ObservationEnd"];
	        this.DeduplicationRule = source["DeduplicationRule"];
	    }
	}
	export class CoverageFigure {
	    checksConcluded: number;
	    checksApplicable: number;
	
	    static createFrom(source: any = {}) {
	        return new CoverageFigure(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.checksConcluded = source["checksConcluded"];
	        this.checksApplicable = source["checksApplicable"];
	    }
	}
	export class FactorTrace {
	    id: string;
	    name: string;
	    value: number;
	    source: string;
	    evidenceClass: string;
	    observedAt?: string;
	    state: string;
	
	    static createFrom(source: any = {}) {
	        return new FactorTrace(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.value = source["value"];
	        this.source = source["source"];
	        this.evidenceClass = source["evidenceClass"];
	        this.observedAt = source["observedAt"];
	        this.state = source["state"];
	    }
	}
	export class TimeAtRiskSummary {
	    observedDays: number;
	    inferredDays: number;
	    estimatedBlindDays: number;
	    expectedDetectionDays: number;
	    leftCensored: boolean;
	    stillExposed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TimeAtRiskSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.observedDays = source["observedDays"];
	        this.inferredDays = source["inferredDays"];
	        this.estimatedBlindDays = source["estimatedBlindDays"];
	        this.expectedDetectionDays = source["expectedDetectionDays"];
	        this.leftCensored = source["leftCensored"];
	        this.stillExposed = source["stillExposed"];
	    }
	}
	export class Interval {
	    point: number;
	    low: number;
	    high: number;
	
	    static createFrom(source: any = {}) {
	        return new Interval(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.point = source["point"];
	        this.low = source["low"];
	        this.high = source["high"];
	    }
	}
	export class ContactPriorEstimate {
	    assetId: string;
	    serviceId: string;
	    serviceClass: string;
	    weaknessClass: string;
	    available: boolean;
	    suppressedReason?: string;
	    lambdaContact: Interval;
	    windowDays: number;
	    windowRationale: string;
	    pContact: Interval;
	    timeAtRisk: TimeAtRiskSummary;
	    factors: FactorTrace[];
	    excludedFactors?: string[];
	    coverage: CoverageFigure;
	    label: string;
	    scopeStatement: string;
	    isPrior: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ContactPriorEstimate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.assetId = source["assetId"];
	        this.serviceId = source["serviceId"];
	        this.serviceClass = source["serviceClass"];
	        this.weaknessClass = source["weaknessClass"];
	        this.available = source["available"];
	        this.suppressedReason = source["suppressedReason"];
	        this.lambdaContact = this.convertValues(source["lambdaContact"], Interval);
	        this.windowDays = source["windowDays"];
	        this.windowRationale = source["windowRationale"];
	        this.pContact = this.convertValues(source["pContact"], Interval);
	        this.timeAtRisk = this.convertValues(source["timeAtRisk"], TimeAtRiskSummary);
	        this.factors = this.convertValues(source["factors"], FactorTrace);
	        this.excludedFactors = source["excludedFactors"];
	        this.coverage = this.convertValues(source["coverage"], CoverageFigure);
	        this.label = source["label"];
	        this.scopeStatement = source["scopeStatement"];
	        this.isPrior = source["isPrior"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class InstanceAggregation {
	    weaknessClass: string;
	    totalInstances: number;
	    confirmedRemediated: number;
	    uncoveredInstances: number;
	    unknownReachability: number;
	    available: boolean;
	    allInstancesPContact: Interval;
	    residualPContact: Interval;
	    coverage: CoverageFigure;
	    label: string;
	    scopeStatement: string;
	    isPrior: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InstanceAggregation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.weaknessClass = source["weaknessClass"];
	        this.totalInstances = source["totalInstances"];
	        this.confirmedRemediated = source["confirmedRemediated"];
	        this.uncoveredInstances = source["uncoveredInstances"];
	        this.unknownReachability = source["unknownReachability"];
	        this.available = source["available"];
	        this.allInstancesPContact = this.convertValues(source["allInstancesPContact"], Interval);
	        this.residualPContact = this.convertValues(source["residualPContact"], Interval);
	        this.coverage = this.convertValues(source["coverage"], CoverageFigure);
	        this.label = source["label"];
	        this.scopeStatement = source["scopeStatement"];
	        this.isPrior = source["isPrior"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace service {
	
	export class AssessOptions {
	    profile?: string;
	    discover: boolean;
	    discoverActive: boolean;
	    discoverBudget?: number;
	    probeAddresses: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AssessOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = source["profile"];
	        this.discover = source["discover"];
	        this.discoverActive = source["discoverActive"];
	        this.discoverBudget = source["discoverBudget"];
	        this.probeAddresses = source["probeAddresses"];
	    }
	}
	export class AssetContactPanel {
	    assetId: string;
	    assetValue: string;
	    reachability: string;
	    estimates: contact.ContactPriorEstimate[];
	    scopeStatement: string;
	
	    static createFrom(source: any = {}) {
	        return new AssetContactPanel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.assetId = source["assetId"];
	        this.assetValue = source["assetValue"];
	        this.reachability = source["reachability"];
	        this.estimates = this.convertValues(source["estimates"], contact.ContactPriorEstimate);
	        this.scopeStatement = source["scopeStatement"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CheckView {
	    checkId: string;
	    state: string;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new CheckView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.checkId = source["checkId"];
	        this.state = source["state"];
	        this.reason = source["reason"];
	    }
	}
	export class SignalView {
	    signalId: string;
	    checkId: string;
	    condition: string;
	    weaknessClass: string;
	    scenario: string;
	    stage: string;
	    control: string;
	    direction: string;
	    state: string;
	    severity: string;
	    evidence: string;
	    mapped: boolean;
	    description?: string;
	    remediation?: string;
	    references?: string[];
	    detail?: string;
	    registryVersion: string;
	    libraryVersion: string;
	    observedAt: string;
	    firstSeen: string;
	
	    static createFrom(source: any = {}) {
	        return new SignalView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.signalId = source["signalId"];
	        this.checkId = source["checkId"];
	        this.condition = source["condition"];
	        this.weaknessClass = source["weaknessClass"];
	        this.scenario = source["scenario"];
	        this.stage = source["stage"];
	        this.control = source["control"];
	        this.direction = source["direction"];
	        this.state = source["state"];
	        this.severity = source["severity"];
	        this.evidence = source["evidence"];
	        this.mapped = source["mapped"];
	        this.description = source["description"];
	        this.remediation = source["remediation"];
	        this.references = source["references"];
	        this.detail = source["detail"];
	        this.registryVersion = source["registryVersion"];
	        this.libraryVersion = source["libraryVersion"];
	        this.observedAt = source["observedAt"];
	        this.firstSeen = source["firstSeen"];
	    }
	}
	export class ControlView {
	    control: string;
	    posture: string;
	    coverage: store.CoverageSummary;
	    checks: CheckView[];
	    signals: SignalView[];
	
	    static createFrom(source: any = {}) {
	        return new ControlView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.control = source["control"];
	        this.posture = source["posture"];
	        this.coverage = this.convertValues(source["coverage"], store.CoverageSummary);
	        this.checks = this.convertValues(source["checks"], CheckView);
	        this.signals = this.convertValues(source["signals"], SignalView);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ScenarioView {
	    scenario: string;
	    coverage: store.CoverageSummary;
	    aggravating: number;
	    significant: number;
	    mitigating: number;
	    supported: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ScenarioView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scenario = source["scenario"];
	        this.coverage = this.convertValues(source["coverage"], store.CoverageSummary);
	        this.aggravating = source["aggravating"];
	        this.significant = source["significant"];
	        this.mitigating = source["mitigating"];
	        this.supported = source["supported"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DomainAssessment {
	    assetId: string;
	    domain: string;
	    outcome: string;
	    error?: string;
	    coverage: store.CoverageSummary;
	    coverageFraction: number;
	    controls: ControlView[];
	    scenarios: ScenarioView[];
	    unmapped: SignalView[];
	    attribution: store.AssetAttribution[];
	    attributionProvenance: store.AttributionProvenance[];
	    serviceExposures: store.AssetExposureHistory[];
	    serviceObservations: store.ServiceObservation[];
	    registryVersion: string;
	    libraryVersion: string;
	    assessedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new DomainAssessment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.assetId = source["assetId"];
	        this.domain = source["domain"];
	        this.outcome = source["outcome"];
	        this.error = source["error"];
	        this.coverage = this.convertValues(source["coverage"], store.CoverageSummary);
	        this.coverageFraction = source["coverageFraction"];
	        this.controls = this.convertValues(source["controls"], ControlView);
	        this.scenarios = this.convertValues(source["scenarios"], ScenarioView);
	        this.unmapped = this.convertValues(source["unmapped"], SignalView);
	        this.attribution = this.convertValues(source["attribution"], store.AssetAttribution);
	        this.attributionProvenance = this.convertValues(source["attributionProvenance"], store.AttributionProvenance);
	        this.serviceExposures = this.convertValues(source["serviceExposures"], store.AssetExposureHistory);
	        this.serviceObservations = this.convertValues(source["serviceObservations"], store.ServiceObservation);
	        this.registryVersion = source["registryVersion"];
	        this.libraryVersion = source["libraryVersion"];
	        this.assessedAt = source["assessedAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace store {
	
	export class Asset {
	    id: string;
	    type: string;
	    value: string;
	    status: string;
	    discoverySource: string;
	    confidence: number;
	    // Go type: time
	    firstSeen: any;
	    // Go type: time
	    lastSeen: any;
	    metadata: string;
	
	    static createFrom(source: any = {}) {
	        return new Asset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.value = source["value"];
	        this.status = source["status"];
	        this.discoverySource = source["discoverySource"];
	        this.confidence = source["confidence"];
	        this.firstSeen = this.convertValues(source["firstSeen"], null);
	        this.lastSeen = this.convertValues(source["lastSeen"], null);
	        this.metadata = source["metadata"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AssetAttribution {
	    assetId: string;
	    host: string;
	    role?: string;
	    address: string;
	    provider?: string;
	    region?: string;
	    jurisdiction?: string;
	    source?: string;
	    libraryVersion?: string;
	    // Go type: time
	    observedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new AssetAttribution(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.assetId = source["assetId"];
	        this.host = source["host"];
	        this.role = source["role"];
	        this.address = source["address"];
	        this.provider = source["provider"];
	        this.region = source["region"];
	        this.jurisdiction = source["jurisdiction"];
	        this.source = source["source"];
	        this.libraryVersion = source["libraryVersion"];
	        this.observedAt = this.convertValues(source["observedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AssetExposureHistory {
	    assetId: string;
	    service: string;
	    firstObserved: string;
	    lastObserved: string;
	    stillExposed: boolean;
	    leftCensored: boolean;
	    observedDurationSeconds: number;
	    inferredDurationSeconds: number;
	    blindDurationSeconds: number;
	    expectedBlindSeconds: number;
	    worstBlindSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new AssetExposureHistory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.assetId = source["assetId"];
	        this.service = source["service"];
	        this.firstObserved = source["firstObserved"];
	        this.lastObserved = source["lastObserved"];
	        this.stillExposed = source["stillExposed"];
	        this.leftCensored = source["leftCensored"];
	        this.observedDurationSeconds = source["observedDurationSeconds"];
	        this.inferredDurationSeconds = source["inferredDurationSeconds"];
	        this.blindDurationSeconds = source["blindDurationSeconds"];
	        this.expectedBlindSeconds = source["expectedBlindSeconds"];
	        this.worstBlindSeconds = source["worstBlindSeconds"];
	    }
	}
	export class AttributionProvenance {
	    assetId: string;
	    provider: string;
	    url: string;
	    // Go type: time
	    fetchedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new AttributionProvenance(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.assetId = source["assetId"];
	        this.provider = source["provider"];
	        this.url = source["url"];
	        this.fetchedAt = this.convertValues(source["fetchedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CalibrationExperiment {
	    id: string;
	    label: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new CalibrationExperiment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CoverageSummary {
	    total: number;
	    ok: number;
	    notFound: number;
	    notChecked: number;
	    checkFailed: number;
	    assessedOnly: number;
	
	    static createFrom(source: any = {}) {
	        return new CoverageSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.ok = source["ok"];
	        this.notFound = source["notFound"];
	        this.notChecked = source["notChecked"];
	        this.checkFailed = source["checkFailed"];
	        this.assessedOnly = source["assessedOnly"];
	    }
	}
	export class EmailControl {
	    state: string;
	    detail?: string;
	    reason?: string;
	    conclusive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EmailControl(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.detail = source["detail"];
	        this.reason = source["reason"];
	        this.conclusive = source["conclusive"];
	    }
	}
	export class EmailPosture {
	    domain: string;
	    spf: EmailControl;
	    dkim: EmailControl;
	    dmarc: EmailControl;
	    mtaSts: EmailControl;
	    tlsRpt: EmailControl;
	    bimi: EmailControl;
	    caa: EmailControl;
	    dmarcPolicy: string;
	    dmarcSubdomainPolicy?: string;
	    dmarcPercent: number;
	    dmarcAlignmentSpf?: string;
	    dmarcAlignmentDkim?: string;
	    dmarcReporting: boolean;
	    spfAllMechanism?: string;
	    spfLookups: number;
	    dkimSelectorsExamined?: string[];
	    dkimSelectorsFound?: string[];
	    priority: string;
	    // Go type: time
	    lastChecked: any;
	
	    static createFrom(source: any = {}) {
	        return new EmailPosture(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.domain = source["domain"];
	        this.spf = this.convertValues(source["spf"], EmailControl);
	        this.dkim = this.convertValues(source["dkim"], EmailControl);
	        this.dmarc = this.convertValues(source["dmarc"], EmailControl);
	        this.mtaSts = this.convertValues(source["mtaSts"], EmailControl);
	        this.tlsRpt = this.convertValues(source["tlsRpt"], EmailControl);
	        this.bimi = this.convertValues(source["bimi"], EmailControl);
	        this.caa = this.convertValues(source["caa"], EmailControl);
	        this.dmarcPolicy = source["dmarcPolicy"];
	        this.dmarcSubdomainPolicy = source["dmarcSubdomainPolicy"];
	        this.dmarcPercent = source["dmarcPercent"];
	        this.dmarcAlignmentSpf = source["dmarcAlignmentSpf"];
	        this.dmarcAlignmentDkim = source["dmarcAlignmentDkim"];
	        this.dmarcReporting = source["dmarcReporting"];
	        this.spfAllMechanism = source["spfAllMechanism"];
	        this.spfLookups = source["spfLookups"];
	        this.dkimSelectorsExamined = source["dkimSelectorsExamined"];
	        this.dkimSelectorsFound = source["dkimSelectorsFound"];
	        this.priority = source["priority"];
	        this.lastChecked = this.convertValues(source["lastChecked"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FeedSnapshot {
	    id: string;
	    feed: string;
	    sourceUrl: string;
	    // Go type: time
	    retrievedAt: any;
	    contentDigest: string;
	    recordCount: number;
	
	    static createFrom(source: any = {}) {
	        return new FeedSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.feed = source["feed"];
	        this.sourceUrl = source["sourceUrl"];
	        this.retrievedAt = this.convertValues(source["retrievedAt"], null);
	        this.contentDigest = source["contentDigest"];
	        this.recordCount = source["recordCount"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FeedStatus {
	    feed: string;
	    snapshot?: FeedSnapshot;
	    // Go type: time
	    lastFailureAt?: any;
	    lastError?: string;
	    stale: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FeedStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.feed = source["feed"];
	        this.snapshot = this.convertValues(source["snapshot"], FeedSnapshot);
	        this.lastFailureAt = this.convertValues(source["lastFailureAt"], null);
	        this.lastError = source["lastError"];
	        this.stale = source["stale"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FindingEnrichment {
	    findingId: string;
	    feed: string;
	    cve: string;
	    state: string;
	    snapshotId?: string;
	    epss?: number;
	    kevListed?: boolean;
	    // Go type: time
	    checkedAt?: any;
	    snapshot?: FeedSnapshot;
	
	    static createFrom(source: any = {}) {
	        return new FindingEnrichment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.findingId = source["findingId"];
	        this.feed = source["feed"];
	        this.cve = source["cve"];
	        this.state = source["state"];
	        this.snapshotId = source["snapshotId"];
	        this.epss = source["epss"];
	        this.kevListed = source["kevListed"];
	        this.checkedAt = this.convertValues(source["checkedAt"], null);
	        this.snapshot = this.convertValues(source["snapshot"], FeedSnapshot);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Finding {
	    id: string;
	    assetId: string;
	    title: string;
	    description: string;
	    severity: string;
	    status: string;
	    priority: string;
	    cve?: string;
	    epss?: number;
	    kevListed: boolean;
	    category: string;
	    proof: string;
	    aiAnnotation?: string;
	    // Go type: time
	    firstSeen: any;
	    // Go type: time
	    lastSeen: any;
	    enrichments?: FindingEnrichment[];
	
	    static createFrom(source: any = {}) {
	        return new Finding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.assetId = source["assetId"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.severity = source["severity"];
	        this.status = source["status"];
	        this.priority = source["priority"];
	        this.cve = source["cve"];
	        this.epss = source["epss"];
	        this.kevListed = source["kevListed"];
	        this.category = source["category"];
	        this.proof = source["proof"];
	        this.aiAnnotation = source["aiAnnotation"];
	        this.firstSeen = this.convertValues(source["firstSeen"], null);
	        this.lastSeen = this.convertValues(source["lastSeen"], null);
	        this.enrichments = this.convertValues(source["enrichments"], FindingEnrichment);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Regression {
	    id: string;
	    assetId: string;
	    attributeType: string;
	    previousValue: string;
	    currentValue: string;
	    consecutiveFails: number;
	    // Go type: time
	    confirmedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Regression(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.assetId = source["assetId"];
	        this.attributeType = source["attributeType"];
	        this.previousValue = source["previousValue"];
	        this.currentValue = source["currentValue"];
	        this.consecutiveFails = source["consecutiveFails"];
	        this.confirmedAt = this.convertValues(source["confirmedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SecretFinding {
	    id: string;
	    assetId: string;
	    repoUrl: string;
	    ruleId: string;
	    secretType: string;
	    redactedRef: string;
	    filePath: string;
	    startLine: number;
	    verified: boolean;
	    isReused: boolean;
	    // Go type: time
	    firstSeen: any;
	
	    static createFrom(source: any = {}) {
	        return new SecretFinding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.assetId = source["assetId"];
	        this.repoUrl = source["repoUrl"];
	        this.ruleId = source["ruleId"];
	        this.secretType = source["secretType"];
	        this.redactedRef = source["redactedRef"];
	        this.filePath = source["filePath"];
	        this.startLine = source["startLine"];
	        this.verified = source["verified"];
	        this.isReused = source["isReused"];
	        this.firstSeen = this.convertValues(source["firstSeen"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ServiceObservation {
	    id: string;
	    assetId: string;
	    host: string;
	    port: number;
	    service: string;
	    transport: string;
	    protocol: string;
	    layer: string;
	    state: string;
	    coverage: string;
	    evidence?: string;
	    profile: string;
	    // Go type: time
	    observedAt: any;
	    // Go type: time
	    firstSeen: any;
	    // Go type: time
	    lastSeen: any;
	
	    static createFrom(source: any = {}) {
	        return new ServiceObservation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.assetId = source["assetId"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.service = source["service"];
	        this.transport = source["transport"];
	        this.protocol = source["protocol"];
	        this.layer = source["layer"];
	        this.state = source["state"];
	        this.coverage = source["coverage"];
	        this.evidence = source["evidence"];
	        this.profile = source["profile"];
	        this.observedAt = this.convertValues(source["observedAt"], null);
	        this.firstSeen = this.convertValues(source["firstSeen"], null);
	        this.lastSeen = this.convertValues(source["lastSeen"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace version {
	
	export class Info {
	    version: string;
	    commit?: string;
	    buildDate?: string;
	    goVersion: string;
	    platform: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.commit = source["commit"];
	        this.buildDate = source["buildDate"];
	        this.goVersion = source["goVersion"];
	        this.platform = source["platform"];
	    }
	}

}


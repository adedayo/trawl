export namespace service {
	
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
	export class EmailPosture {
	    domain: string;
	    spfValid: boolean;
	    dkimFound: boolean;
	    dmarcPolicy: string;
	    priority: string;
	    // Go type: time
	    lastChecked: any;
	    mtaStsFound: boolean;
	    mtaStsMode: string;
	    dnssecValid: boolean;
	    daneValid: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EmailPosture(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.domain = source["domain"];
	        this.spfValid = source["spfValid"];
	        this.dkimFound = source["dkimFound"];
	        this.dmarcPolicy = source["dmarcPolicy"];
	        this.priority = source["priority"];
	        this.lastChecked = this.convertValues(source["lastChecked"], null);
	        this.mtaStsFound = source["mtaStsFound"];
	        this.mtaStsMode = source["mtaStsMode"];
	        this.dnssecValid = source["dnssecValid"];
	        this.daneValid = source["daneValid"];
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
	        this.priority = source["priority"];
	        this.cve = source["cve"];
	        this.epss = source["epss"];
	        this.kevListed = source["kevListed"];
	        this.category = source["category"];
	        this.proof = source["proof"];
	        this.aiAnnotation = source["aiAnnotation"];
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

}


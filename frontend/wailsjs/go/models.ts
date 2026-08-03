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


export namespace appshell {

	export class AppMetadata {
	    name: string;
	    productLine: string;
	    platform: string;
	    architecture: string;
	    goVersion: string;
	    wailsVersion: string;

	    static createFrom(source: any = {}) {
	        return new AppMetadata(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.productLine = source["productLine"];
	        this.platform = source["platform"];
	        this.architecture = source["architecture"];
	        this.goVersion = source["goVersion"];
	        this.wailsVersion = source["wailsVersion"];
	    }
	}
	export class SliceStatus {
	    slice: string;
	    status: string;
	    summary: string;

	    static createFrom(source: any = {}) {
	        return new SliceStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.slice = source["slice"];
	        this.status = source["status"];
	        this.summary = source["summary"];
	    }
	}
	export class StateModelSummary {
	    primaryFlow: string[];
	    overlayViews: string[];
	    singleActiveLiveSession: boolean;

	    static createFrom(source: any = {}) {
	        return new StateModelSummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.primaryFlow = source["primaryFlow"];
	        this.overlayViews = source["overlayViews"];
	        this.singleActiveLiveSession = source["singleActiveLiveSession"];
	    }
	}
	export class LayoutRegion {
	    id: string;
	    title: string;
	    purpose: string;

	    static createFrom(source: any = {}) {
	        return new LayoutRegion(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.purpose = source["purpose"];
	    }
	}
	export class LayoutDefinition {
	    regions: LayoutRegion[];

	    static createFrom(source: any = {}) {
	        return new LayoutDefinition(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.regions = this.convertValues(source["regions"], LayoutRegion);
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
	export class BootstrapData {
	    app: AppMetadata;
	    contract: contracts.ContractManifest;
	    layout: LayoutDefinition;
	    stateModel: StateModelSummary;
	    epicZero: SliceStatus[];

	    static createFrom(source: any = {}) {
	        return new BootstrapData(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.app = this.convertValues(source["app"], AppMetadata);
	        this.contract = this.convertValues(source["contract"], contracts.ContractManifest);
	        this.layout = this.convertValues(source["layout"], LayoutDefinition);
	        this.stateModel = this.convertValues(source["stateModel"], StateModelSummary);
	        this.epicZero = this.convertValues(source["epicZero"], SliceStatus);
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
	export class BootstrapResponse {
	    ok: boolean;
	    data?: BootstrapData;
	    error?: contracts.ErrorEnvelope;

	    static createFrom(source: any = {}) {
	        return new BootstrapResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], BootstrapData);
	        this.error = this.convertValues(source["error"], contracts.ErrorEnvelope);
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


	export class ProbeAcknowledgement {
	    eventId: string;
	    eventName: string;
	    emittedAt: string;
	    classification: string;

	    static createFrom(source: any = {}) {
	        return new ProbeAcknowledgement(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.eventId = source["eventId"];
	        this.eventName = source["eventName"];
	        this.emittedAt = source["emittedAt"];
	        this.classification = source["classification"];
	    }
	}
	export class ProbeResponse {
	    ok: boolean;
	    data?: ProbeAcknowledgement;
	    error?: contracts.ErrorEnvelope;

	    static createFrom(source: any = {}) {
	        return new ProbeResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], ProbeAcknowledgement);
	        this.error = this.convertValues(source["error"], contracts.ErrorEnvelope);
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

export namespace contracts {

	export class EndpointTLSSettings {
	    mode: string;
	    serverNameOverride?: string;
	    caCert?: string;
	    clientCert?: string;
	    clientKey?: string;

	    static createFrom(source: any = {}) {
	        return new EndpointTLSSettings(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.serverNameOverride = source["serverNameOverride"];
	        this.caCert = source["caCert"];
	        this.clientCert = source["clientCert"];
	        this.clientKey = source["clientKey"];
	    }
	}
	export class EndpointPreset {
	    id?: string;
	    name?: string;
	    target: string;
	    authority?: string;
	    tls: EndpointTLSSettings;
	    connectTimeoutMs?: number;
	    requestTimeoutMs?: number;
	    streamIdleTimeoutMs?: number;
	    metadataDefaults?: Record<string, string>;

	    static createFrom(source: any = {}) {
	        return new EndpointPreset(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.target = source["target"];
	        this.authority = source["authority"];
	        this.tls = this.convertValues(source["tls"], EndpointTLSSettings);
	        this.connectTimeoutMs = source["connectTimeoutMs"];
	        this.requestTimeoutMs = source["requestTimeoutMs"];
	        this.streamIdleTimeoutMs = source["streamIdleTimeoutMs"];
	        this.metadataDefaults = source["metadataDefaults"];
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
	export class CatalogLoadFromReflectionInput {
	    endpoint: EndpointPreset;

	    static createFrom(source: any = {}) {
	        return new CatalogLoadFromReflectionInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpoint = this.convertValues(source["endpoint"], EndpointPreset);
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
	export class ErrorEnvelope {
	    code: string;
	    category: string;
	    message: string;
	    details?: Record<string, string>;

	    static createFrom(source: any = {}) {
	        return new ErrorEnvelope(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.category = source["category"];
	        this.message = source["message"];
	        this.details = source["details"];
	    }
	}
	export class DiagnosticsUpdateEvent {
	    id: string;
	    source: string;
	    level: string;
	    code: string;
	    category: string;
	    message: string;
	    nextStep?: string;
	    details?: Record<string, string>;
	    ts: string;

	    static createFrom(source: any = {}) {
	        return new DiagnosticsUpdateEvent(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.source = source["source"];
	        this.level = source["level"];
	        this.code = source["code"];
	        this.category = source["category"];
	        this.message = source["message"];
	        this.nextStep = source["nextStep"];
	        this.details = source["details"];
	        this.ts = source["ts"];
	    }
	}
	export class CatalogMessageRef {
	    name: string;
	    fullName: string;
	    isWellKnown: boolean;

	    static createFrom(source: any = {}) {
	        return new CatalogMessageRef(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.fullName = source["fullName"];
	        this.isWellKnown = source["isWellKnown"];
	    }
	}
	export class CatalogMethod {
	    name: string;
	    fullName: string;
	    rpcType: string;
	    requestType: CatalogMessageRef;
	    responseType: CatalogMessageRef;

	    static createFrom(source: any = {}) {
	        return new CatalogMethod(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.fullName = source["fullName"];
	        this.rpcType = source["rpcType"];
	        this.requestType = this.convertValues(source["requestType"], CatalogMessageRef);
	        this.responseType = this.convertValues(source["responseType"], CatalogMessageRef);
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
	export class CatalogService {
	    name: string;
	    fullName: string;
	    methods: CatalogMethod[];

	    static createFrom(source: any = {}) {
	        return new CatalogService(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.fullName = source["fullName"];
	        this.methods = this.convertValues(source["methods"], CatalogMethod);
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
	export class ReflectionCatalogResult {
	    endpoint: EndpointPreset;
	    services: CatalogService[];
	    wellKnownTypes?: CatalogMessageRef[];
	    diagnostic?: DiagnosticsUpdateEvent;
	    loadedAt: string;
	    durationMs: number;

	    static createFrom(source: any = {}) {
	        return new ReflectionCatalogResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpoint = this.convertValues(source["endpoint"], EndpointPreset);
	        this.services = this.convertValues(source["services"], CatalogService);
	        this.wellKnownTypes = this.convertValues(source["wellKnownTypes"], CatalogMessageRef);
	        this.diagnostic = this.convertValues(source["diagnostic"], DiagnosticsUpdateEvent);
	        this.loadedAt = source["loadedAt"];
	        this.durationMs = source["durationMs"];
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
	export class CatalogLoadFromReflectionResponse {
	    ok: boolean;
	    data?: ReflectionCatalogResult;
	    error?: ErrorEnvelope;

	    static createFrom(source: any = {}) {
	        return new CatalogLoadFromReflectionResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], ReflectionCatalogResult);
	        this.error = this.convertValues(source["error"], ErrorEnvelope);
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



	export class ModuleContract {
	    name: string;
	    responsibility: string;

	    static createFrom(source: any = {}) {
	        return new ModuleContract(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.responsibility = source["responsibility"];
	    }
	}
	export class TransitionRule {
	    from: string;
	    event: string;
	    to: string[];
	    notes?: string;

	    static createFrom(source: any = {}) {
	        return new TransitionRule(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.event = source["event"];
	        this.to = source["to"];
	        this.notes = source["notes"];
	    }
	}
	export class ContractManifest {
	    version: string;
	    boundMethods: string[];
	    eventNames: string[];
	    errorCategories: string[];
	    topLevelViews: string[];
	    overlays: string[];
	    streamStates: string[];
	    terminalStreamStates: string[];
	    sessionConditions: string[];
	    transitions: TransitionRule[];
	    modules: ModuleContract[];

	    static createFrom(source: any = {}) {
	        return new ContractManifest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.boundMethods = source["boundMethods"];
	        this.eventNames = source["eventNames"];
	        this.errorCategories = source["errorCategories"];
	        this.topLevelViews = source["topLevelViews"];
	        this.overlays = source["overlays"];
	        this.streamStates = source["streamStates"];
	        this.terminalStreamStates = source["terminalStreamStates"];
	        this.sessionConditions = source["sessionConditions"];
	        this.transitions = this.convertValues(source["transitions"], TransitionRule);
	        this.modules = this.convertValues(source["modules"], ModuleContract);
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

	export class EndpointCheck {
	    stage: string;
	    outcome: string;
	    message: string;
	    details?: Record<string, string>;

	    static createFrom(source: any = {}) {
	        return new EndpointCheck(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stage = source["stage"];
	        this.outcome = source["outcome"];
	        this.message = source["message"];
	        this.details = source["details"];
	    }
	}


	export class EndpointTestInput {
	    endpoint: EndpointPreset;

	    static createFrom(source: any = {}) {
	        return new EndpointTestInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpoint = this.convertValues(source["endpoint"], EndpointPreset);
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
	export class EndpointTestResult {
	    endpoint: EndpointPreset;
	    transportReachable: boolean;
	    tlsConfigured: boolean;
	    tlsOk: boolean;
	    grpcReady: boolean;
	    grpcReadyProven: boolean;
	    checks: EndpointCheck[];
	    diagnostic?: DiagnosticsUpdateEvent;
	    testedAt: string;
	    durationMs: number;

	    static createFrom(source: any = {}) {
	        return new EndpointTestResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpoint = this.convertValues(source["endpoint"], EndpointPreset);
	        this.transportReachable = source["transportReachable"];
	        this.tlsConfigured = source["tlsConfigured"];
	        this.tlsOk = source["tlsOk"];
	        this.grpcReady = source["grpcReady"];
	        this.grpcReadyProven = source["grpcReadyProven"];
	        this.checks = this.convertValues(source["checks"], EndpointCheck);
	        this.diagnostic = this.convertValues(source["diagnostic"], DiagnosticsUpdateEvent);
	        this.testedAt = source["testedAt"];
	        this.durationMs = source["durationMs"];
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
	export class EndpointTestResponse {
	    ok: boolean;
	    data?: EndpointTestResult;
	    error?: ErrorEnvelope;

	    static createFrom(source: any = {}) {
	        return new EndpointTestResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], EndpointTestResult);
	        this.error = this.convertValues(source["error"], ErrorEnvelope);
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


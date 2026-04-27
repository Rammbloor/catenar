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
	
	export class CallOptions {
	    requestTimeoutMs?: number;
	    streamIdleTimeoutMs?: number;
	
	    static createFrom(source: any = {}) {
	        return new CallOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requestTimeoutMs = source["requestTimeoutMs"];
	        this.streamIdleTimeoutMs = source["streamIdleTimeoutMs"];
	    }
	}
	export class CallInvokeUnaryInput {
	    catalogSource?: string;
	    endpointId: string;
	    method: string;
	    environmentRef?: string;
	    metadata?: Record<string, string>;
	    body: any;
	    callOptions?: CallOptions;
	
	    static createFrom(source: any = {}) {
	        return new CallInvokeUnaryInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.catalogSource = source["catalogSource"];
	        this.endpointId = source["endpointId"];
	        this.method = source["method"];
	        this.environmentRef = source["environmentRef"];
	        this.metadata = source["metadata"];
	        this.body = source["body"];
	        this.callOptions = this.convertValues(source["callOptions"], CallOptions);
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
	export class StreamStatus {
	    code: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new StreamStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.message = source["message"];
	    }
	}
	export class CallInvokeUnaryResult {
	    callId: string;
	    sessionId: string;
	    endpointId: string;
	    method: string;
	    rpcType: string;
	    finalState: string;
	    requestBody: any;
	    responseBody?: any;
	    headers?: Record<string, Array<string>>;
	    trailers?: Record<string, Array<string>>;
	    status: StreamStatus;
	    diagnostic?: DiagnosticsUpdateEvent;
	    startedAt: string;
	    finishedAt: string;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new CallInvokeUnaryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.callId = source["callId"];
	        this.sessionId = source["sessionId"];
	        this.endpointId = source["endpointId"];
	        this.method = source["method"];
	        this.rpcType = source["rpcType"];
	        this.finalState = source["finalState"];
	        this.requestBody = source["requestBody"];
	        this.responseBody = source["responseBody"];
	        this.headers = source["headers"];
	        this.trailers = source["trailers"];
	        this.status = this.convertValues(source["status"], StreamStatus);
	        this.diagnostic = this.convertValues(source["diagnostic"], DiagnosticsUpdateEvent);
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
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
	export class CallInvokeUnaryResponse {
	    ok: boolean;
	    data?: CallInvokeUnaryResult;
	    error?: ErrorEnvelope;
	
	    static createFrom(source: any = {}) {
	        return new CallInvokeUnaryResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], CallInvokeUnaryResult);
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
	
	
	export class ProtoSource {
	    type: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new ProtoSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.path = source["path"];
	    }
	}
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
	export class CatalogLoadFromProtoSourcesInput {
	    endpoint: EndpointPreset;
	    protoSources: ProtoSource[];
	    importPaths?: string[];
	
	    static createFrom(source: any = {}) {
	        return new CatalogLoadFromProtoSourcesInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpoint = this.convertValues(source["endpoint"], EndpointPreset);
	        this.protoSources = this.convertValues(source["protoSources"], ProtoSource);
	        this.importPaths = source["importPaths"];
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
	export class ProtoCatalogResult {
	    endpoint: EndpointPreset;
	    protoSources: ProtoSource[];
	    importPaths?: string[];
	    services: CatalogService[];
	    wellKnownTypes?: CatalogMessageRef[];
	    requestTemplates?: Record<string, any>;
	    diagnostic?: DiagnosticsUpdateEvent;
	    loadedAt: string;
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new ProtoCatalogResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpoint = this.convertValues(source["endpoint"], EndpointPreset);
	        this.protoSources = this.convertValues(source["protoSources"], ProtoSource);
	        this.importPaths = source["importPaths"];
	        this.services = this.convertValues(source["services"], CatalogService);
	        this.wellKnownTypes = this.convertValues(source["wellKnownTypes"], CatalogMessageRef);
	        this.requestTemplates = source["requestTemplates"];
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
	export class CatalogLoadFromProtoSourcesResponse {
	    ok: boolean;
	    data?: ProtoCatalogResult;
	    error?: ErrorEnvelope;
	
	    static createFrom(source: any = {}) {
	        return new CatalogLoadFromProtoSourcesResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], ProtoCatalogResult);
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
	export class ReflectionCatalogResult {
	    endpoint: EndpointPreset;
	    services: CatalogService[];
	    wellKnownTypes?: CatalogMessageRef[];
	    requestTemplates?: Record<string, any>;
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
	        this.requestTemplates = source["requestTemplates"];
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
	
	
	export class HistoryCallSummary {
	    callId: string;
	    sessionId?: string;
	    workspaceId?: string;
	    method: string;
	    rpcType: string;
	    endpointId: string;
	    state: string;
	    grpcStatusCode?: string;
	    startedAt: string;
	    finishedAt?: string;
	    durationMs?: number;
	    requestCount: number;
	    responseCount: number;
	    truncated: boolean;
	    errorCategory?: string;
	    errorCode?: string;
	    summaryPath?: string;
	    sessionLogPath?: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryCallSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.callId = source["callId"];
	        this.sessionId = source["sessionId"];
	        this.workspaceId = source["workspaceId"];
	        this.method = source["method"];
	        this.rpcType = source["rpcType"];
	        this.endpointId = source["endpointId"];
	        this.state = source["state"];
	        this.grpcStatusCode = source["grpcStatusCode"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.durationMs = source["durationMs"];
	        this.requestCount = source["requestCount"];
	        this.responseCount = source["responseCount"];
	        this.truncated = source["truncated"];
	        this.errorCategory = source["errorCategory"];
	        this.errorCode = source["errorCode"];
	        this.summaryPath = source["summaryPath"];
	        this.sessionLogPath = source["sessionLogPath"];
	    }
	}
	export class HistoryLogGRPC {
	    method?: string;
	    rpcType?: string;
	    statusCode?: string;
	    metadata?: Record<string, Array<string>>;
	
	    static createFrom(source: any = {}) {
	        return new HistoryLogGRPC(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.method = source["method"];
	        this.rpcType = source["rpcType"];
	        this.statusCode = source["statusCode"];
	        this.metadata = source["metadata"];
	    }
	}
	export class HistoryLogPreview {
	    json?: any;
	
	    static createFrom(source: any = {}) {
	        return new HistoryLogPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.json = source["json"];
	    }
	}
	export class HistoryLogEvent {
	    callId: string;
	    sessionId?: string;
	    seq: number;
	    kind: string;
	    direction?: string;
	    messageIndex?: number;
	    sizeBytes?: number;
	    preview?: HistoryLogPreview;
	    grpc?: HistoryLogGRPC;
	    details?: Record<string, string>;
	    ts: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryLogEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.callId = source["callId"];
	        this.sessionId = source["sessionId"];
	        this.seq = source["seq"];
	        this.kind = source["kind"];
	        this.direction = source["direction"];
	        this.messageIndex = source["messageIndex"];
	        this.sizeBytes = source["sizeBytes"];
	        this.preview = this.convertValues(source["preview"], HistoryLogPreview);
	        this.grpc = this.convertValues(source["grpc"], HistoryLogGRPC);
	        this.details = source["details"];
	        this.ts = source["ts"];
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
	export class HistoryGetResult {
	    summary: HistoryCallSummary;
	    requestBody: any;
	    responseBody?: any;
	    headers?: Record<string, Array<string>>;
	    trailers?: Record<string, Array<string>>;
	    status: StreamStatus;
	    events: HistoryLogEvent[];
	
	    static createFrom(source: any = {}) {
	        return new HistoryGetResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.summary = this.convertValues(source["summary"], HistoryCallSummary);
	        this.requestBody = source["requestBody"];
	        this.responseBody = source["responseBody"];
	        this.headers = source["headers"];
	        this.trailers = source["trailers"];
	        this.status = this.convertValues(source["status"], StreamStatus);
	        this.events = this.convertValues(source["events"], HistoryLogEvent);
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
	export class HistoryGetResponse {
	    ok: boolean;
	    data?: HistoryGetResult;
	    error?: ErrorEnvelope;
	
	    static createFrom(source: any = {}) {
	        return new HistoryGetResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], HistoryGetResult);
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
	
	export class HistoryListInput {
	    limit?: number;
	
	    static createFrom(source: any = {}) {
	        return new HistoryListInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.limit = source["limit"];
	    }
	}
	export class HistoryListResult {
	    calls: HistoryCallSummary[];
	
	    static createFrom(source: any = {}) {
	        return new HistoryListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.calls = this.convertValues(source["calls"], HistoryCallSummary);
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
	export class HistoryListResponse {
	    ok: boolean;
	    data?: HistoryListResult;
	    error?: ErrorEnvelope;
	
	    static createFrom(source: any = {}) {
	        return new HistoryListResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], HistoryListResult);
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


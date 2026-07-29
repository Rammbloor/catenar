export namespace appshell {

	export class AppMetadata {
	    name: string;
	    version: string;
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
	        this.version = source["version"];
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
	    workspace?: contracts.WorkspaceSnapshot;
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
	        this.workspace = this.convertValues(source["workspace"], contracts.WorkspaceSnapshot);
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

	export class CallCancelInput {
	    sessionId: string;

	    static createFrom(source: any = {}) {
	        return new CallCancelInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
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
	export class CallCancelResult {
	    callId: string;
	    sessionId: string;
	    state: string;
	    requestedAt: string;

	    static createFrom(source: any = {}) {
	        return new CallCancelResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.callId = source["callId"];
	        this.sessionId = source["sessionId"];
	        this.state = source["state"];
	        this.requestedAt = source["requestedAt"];
	    }
	}
	export class CallCancelResponse {
	    ok: boolean;
	    data?: CallCancelResult;
	    error?: ErrorEnvelope;

	    static createFrom(source: any = {}) {
	        return new CallCancelResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], CallCancelResult);
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

	export class CallHalfCloseInput {
	    sessionId: string;

	    static createFrom(source: any = {}) {
	        return new CallHalfCloseInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	    }
	}
	export class CallHalfCloseResult {
	    callId: string;
	    sessionId: string;
	    state: string;
	    requestedAt: string;

	    static createFrom(source: any = {}) {
	        return new CallHalfCloseResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.callId = source["callId"];
	        this.sessionId = source["sessionId"];
	        this.state = source["state"];
	        this.requestedAt = source["requestedAt"];
	    }
	}
	export class CallHalfCloseResponse {
	    ok: boolean;
	    data?: CallHalfCloseResult;
	    error?: ErrorEnvelope;

	    static createFrom(source: any = {}) {
	        return new CallHalfCloseResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], CallHalfCloseResult);
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


	export class StreamMessage {
	    body: any;

	    static createFrom(source: any = {}) {
	        return new StreamMessage(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.body = source["body"];
	    }
	}
	export class CallSendMessageInput {
	    sessionId: string;
	    message: StreamMessage;

	    static createFrom(source: any = {}) {
	        return new CallSendMessageInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.message = this.convertValues(source["message"], StreamMessage);
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
	export class CallSendMessageResult {
	    callId: string;
	    sessionId: string;
	    state: string;
	    messageIndex: number;
	    seq: number;
	    sentAt: string;

	    static createFrom(source: any = {}) {
	        return new CallSendMessageResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.callId = source["callId"];
	        this.sessionId = source["sessionId"];
	        this.state = source["state"];
	        this.messageIndex = source["messageIndex"];
	        this.seq = source["seq"];
	        this.sentAt = source["sentAt"];
	    }
	}
	export class CallSendMessageResponse {
	    ok: boolean;
	    data?: CallSendMessageResult;
	    error?: ErrorEnvelope;

	    static createFrom(source: any = {}) {
	        return new CallSendMessageResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], CallSendMessageResult);
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

	export class StreamRequestSpec {
	    mode: string;
	    messages?: StreamMessage[];

	    static createFrom(source: any = {}) {
	        return new StreamRequestSpec(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.messages = this.convertValues(source["messages"], StreamMessage);
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
	export class CallStartStreamInput {
	    catalogSource?: string;
	    endpointId: string;
	    method: string;
	    rpcType: string;
	    environmentRef?: string;
	    metadata?: Record<string, string>;
	    requestSpec?: StreamRequestSpec;
	    callOptions?: CallOptions;

	    static createFrom(source: any = {}) {
	        return new CallStartStreamInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.catalogSource = source["catalogSource"];
	        this.endpointId = source["endpointId"];
	        this.method = source["method"];
	        this.rpcType = source["rpcType"];
	        this.environmentRef = source["environmentRef"];
	        this.metadata = source["metadata"];
	        this.requestSpec = this.convertValues(source["requestSpec"], StreamRequestSpec);
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
	export class CallStartStreamResult {
	    callId: string;
	    sessionId: string;
	    endpointId: string;
	    method: string;
	    rpcType: string;
	    state: string;
	    startedAt: string;

	    static createFrom(source: any = {}) {
	        return new CallStartStreamResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.callId = source["callId"];
	        this.sessionId = source["sessionId"];
	        this.endpointId = source["endpointId"];
	        this.method = source["method"];
	        this.rpcType = source["rpcType"];
	        this.state = source["state"];
	        this.startedAt = source["startedAt"];
	    }
	}
	export class CallStartStreamResponse {
	    ok: boolean;
	    data?: CallStartStreamResult;
	    error?: ErrorEnvelope;

	    static createFrom(source: any = {}) {
	        return new CallStartStreamResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], CallStartStreamResult);
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

	export class CatalogField {
	    name: string;
	    jsonName: string;
	    type: string;
	    repeated?: boolean;
	    required?: boolean;
	    oneof?: string;
	    fields?: CatalogField[];

	    static createFrom(source: any = {}) {
	        return new CatalogField(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.jsonName = source["jsonName"];
	        this.type = source["type"];
	        this.repeated = source["repeated"];
	        this.required = source["required"];
	        this.oneof = source["oneof"];
	        this.fields = this.convertValues(source["fields"], CatalogField);
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
	    fields?: CatalogField[];

	    static createFrom(source: any = {}) {
	        return new CatalogMessageRef(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.fullName = source["fullName"];
	        this.isWellKnown = source["isWellKnown"];
	        this.fields = this.convertValues(source["fields"], CatalogField);
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
	export class DiagnosticsExportInput {
	    path?: string;
	    callIds?: string[];
	    includeHistory?: boolean;

	    static createFrom(source: any = {}) {
	        return new DiagnosticsExportInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.callIds = source["callIds"];
	        this.includeHistory = source["includeHistory"];
	    }
	}
	export class DiagnosticsExportResult {
	    path: string;
	    fileCount: number;
	    includedCalls?: string[];
	    exportedAt: string;

	    static createFrom(source: any = {}) {
	        return new DiagnosticsExportResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.fileCount = source["fileCount"];
	        this.includedCalls = source["includedCalls"];
	        this.exportedAt = source["exportedAt"];
	    }
	}
	export class DiagnosticsExportResponse {
	    ok: boolean;
	    data?: DiagnosticsExportResult;
	    error?: ErrorEnvelope;

	    static createFrom(source: any = {}) {
	        return new DiagnosticsExportResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], DiagnosticsExportResult);
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


	export class GitHubSyncActionInput {
	    overwrite: boolean;

	    static createFrom(source: any = {}) {
	        return new GitHubSyncActionInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.overwrite = source["overwrite"];
	    }
	}
	export class WorkspaceSavedRequestSummary {
	    id: string;
	    name: string;
	    path: string;
	    method: string;
	    rpcType: string;
	    endpointRef: string;
	    environmentRef?: string;

	    static createFrom(source: any = {}) {
	        return new WorkspaceSavedRequestSummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.method = source["method"];
	        this.rpcType = source["rpcType"];
	        this.endpointRef = source["endpointRef"];
	        this.environmentRef = source["environmentRef"];
	    }
	}
	export class WorkspaceEventRetentionSettings {
	    maxEventsPerCall?: number;
	    maxBytesPerCall?: number;

	    static createFrom(source: any = {}) {
	        return new WorkspaceEventRetentionSettings(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.maxEventsPerCall = source["maxEventsPerCall"];
	        this.maxBytesPerCall = source["maxBytesPerCall"];
	    }
	}
	export class WorkspaceSettings {
	    redactDefaults: boolean;
	    customSecretKeys?: string[];
	    eventRetention?: WorkspaceEventRetentionSettings;

	    static createFrom(source: any = {}) {
	        return new WorkspaceSettings(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.redactDefaults = source["redactDefaults"];
	        this.customSecretKeys = source["customSecretKeys"];
	        this.eventRetention = this.convertValues(source["eventRetention"], WorkspaceEventRetentionSettings);
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
	export class WorkspaceEnvironment {
	    values?: Record<string, string>;

	    static createFrom(source: any = {}) {
	        return new WorkspaceEnvironment(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.values = source["values"];
	    }
	}
	export class WorkspaceSnapshot {
	    id: string;
	    version: number;
	    name: string;
	    path: string;
	    manifestPath: string;
	    endpoints: EndpointPreset[];
	    protoSources: ProtoSource[];
	    importPaths?: string[];
	    environments?: Record<string, WorkspaceEnvironment>;
	    settings?: WorkspaceSettings;
	    savedRequests?: WorkspaceSavedRequestSummary[];
	    backupPaths?: string[];

	    static createFrom(source: any = {}) {
	        return new WorkspaceSnapshot(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.version = source["version"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.manifestPath = source["manifestPath"];
	        this.endpoints = this.convertValues(source["endpoints"], EndpointPreset);
	        this.protoSources = this.convertValues(source["protoSources"], ProtoSource);
	        this.importPaths = source["importPaths"];
	        this.environments = this.convertValues(source["environments"], WorkspaceEnvironment, true);
	        this.settings = this.convertValues(source["settings"], WorkspaceSettings);
	        this.savedRequests = this.convertValues(source["savedRequests"], WorkspaceSavedRequestSummary);
	        this.backupPaths = source["backupPaths"];
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
	export class GitHubWorkspaceLink {
	    repositoryUrl: string;
	    branch: string;
	    workspacePath: string;
	    lastSyncedCommit?: string;
	    lastSyncedAt?: string;

	    static createFrom(source: any = {}) {
	        return new GitHubWorkspaceLink(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repositoryUrl = source["repositoryUrl"];
	        this.branch = source["branch"];
	        this.workspacePath = source["workspacePath"];
	        this.lastSyncedCommit = source["lastSyncedCommit"];
	        this.lastSyncedAt = source["lastSyncedAt"];
	    }
	}
	export class GitHubSyncStatus {
	    linked: boolean;
	    localChanges: boolean;
	    remoteChanges: boolean;
	    conflict: boolean;
	    initialSyncRequired: boolean;
	    remoteCommit?: string;
	    tokenConfigured: boolean;
	    link?: GitHubWorkspaceLink;
	    workspace?: WorkspaceSnapshot;

	    static createFrom(source: any = {}) {
	        return new GitHubSyncStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.linked = source["linked"];
	        this.localChanges = source["localChanges"];
	        this.remoteChanges = source["remoteChanges"];
	        this.conflict = source["conflict"];
	        this.initialSyncRequired = source["initialSyncRequired"];
	        this.remoteCommit = source["remoteCommit"];
	        this.tokenConfigured = source["tokenConfigured"];
	        this.link = this.convertValues(source["link"], GitHubWorkspaceLink);
	        this.workspace = this.convertValues(source["workspace"], WorkspaceSnapshot);
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
	export class GitHubSyncResponse {
	    ok: boolean;
	    data?: GitHubSyncStatus;
	    error?: ErrorEnvelope;

	    static createFrom(source: any = {}) {
	        return new GitHubSyncResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], GitHubSyncStatus);
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

	export class GitHubWorkspaceCredentialInput {
	    accessToken: string;

	    static createFrom(source: any = {}) {
	        return new GitHubWorkspaceCredentialInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accessToken = source["accessToken"];
	    }
	}

	export class GitHubWorkspaceLinkInput {
	    repositoryUrl: string;
	    branch: string;
	    workspacePath: string;
	    accessToken?: string;

	    static createFrom(source: any = {}) {
	        return new GitHubWorkspaceLinkInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repositoryUrl = source["repositoryUrl"];
	        this.branch = source["branch"];
	        this.workspacePath = source["workspacePath"];
	        this.accessToken = source["accessToken"];
	    }
	}
	export class HistoryCallSummary {
	    callId: string;
	    sessionId?: string;
	    workspaceId?: string;
	    environmentRef?: string;
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
	        this.environmentRef = source["environmentRef"];
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
	    endpointId?: string;
	    workspaceId?: string;
	    environmentRef?: string;

	    static createFrom(source: any = {}) {
	        return new HistoryListInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.limit = source["limit"];
	        this.endpointId = source["endpointId"];
	        this.workspaceId = source["workspaceId"];
	        this.environmentRef = source["environmentRef"];
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




	export class MaterialFileRecord {
	    backend: string;
	    namespace: string;
	    key: string;
	    path: string;
	    kind: string;
	    createdAt?: string;
	    updatedAt?: string;

	    static createFrom(source: any = {}) {
	        return new MaterialFileRecord(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.backend = source["backend"];
	        this.namespace = source["namespace"];
	        this.key = source["key"];
	        this.path = source["path"];
	        this.kind = source["kind"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class MaterialRegisterFileInput {
	    namespace: string;
	    key: string;
	    path: string;
	    kind: string;

	    static createFrom(source: any = {}) {
	        return new MaterialRegisterFileInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.namespace = source["namespace"];
	        this.key = source["key"];
	        this.path = source["path"];
	        this.kind = source["kind"];
	    }
	}
	export class MaterialRegisterFileResult {
	    ref: string;
	    record: MaterialFileRecord;

	    static createFrom(source: any = {}) {
	        return new MaterialRegisterFileResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ref = source["ref"];
	        this.record = this.convertValues(source["record"], MaterialFileRecord);
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
	export class MaterialRegisterFileResponse {
	    ok: boolean;
	    data?: MaterialRegisterFileResult;
	    error?: ErrorEnvelope;

	    static createFrom(source: any = {}) {
	        return new MaterialRegisterFileResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], MaterialRegisterFileResult);
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





	export class RequestDeleteInput {
	    id?: string;
	    path?: string;

	    static createFrom(source: any = {}) {
	        return new RequestDeleteInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	    }
	}
	export class RequestDeleteResult {
	    workspace: WorkspaceSnapshot;
	    deletedId: string;

	    static createFrom(source: any = {}) {
	        return new RequestDeleteResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspace = this.convertValues(source["workspace"], WorkspaceSnapshot);
	        this.deletedId = source["deletedId"];
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
	export class RequestDeleteResponse {
	    ok: boolean;
	    data?: RequestDeleteResult;
	    error?: ErrorEnvelope;

	    static createFrom(source: any = {}) {
	        return new RequestDeleteResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], RequestDeleteResult);
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

	export class RequestGetInput {
	    id?: string;
	    path?: string;

	    static createFrom(source: any = {}) {
	        return new RequestGetInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	    }
	}
	export class SavedRequestSpec {
	    mode: string;
	    body?: any;
	    messages?: StreamMessage[];

	    static createFrom(source: any = {}) {
	        return new SavedRequestSpec(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.body = source["body"];
	        this.messages = this.convertValues(source["messages"], StreamMessage);
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
	export class WorkspaceSavedRequest {
	    id: string;
	    name: string;
	    path: string;
	    method: string;
	    rpcType: string;
	    endpointRef: string;
	    environmentRef?: string;
	    metadataTemplate?: Record<string, string>;
	    callOptions?: CallOptions;
	    requestSpec: SavedRequestSpec;

	    static createFrom(source: any = {}) {
	        return new WorkspaceSavedRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.method = source["method"];
	        this.rpcType = source["rpcType"];
	        this.endpointRef = source["endpointRef"];
	        this.environmentRef = source["environmentRef"];
	        this.metadataTemplate = source["metadataTemplate"];
	        this.callOptions = this.convertValues(source["callOptions"], CallOptions);
	        this.requestSpec = this.convertValues(source["requestSpec"], SavedRequestSpec);
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
	export class RequestGetResult {
	    workspace: WorkspaceSnapshot;
	    savedRequest: WorkspaceSavedRequest;

	    static createFrom(source: any = {}) {
	        return new RequestGetResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspace = this.convertValues(source["workspace"], WorkspaceSnapshot);
	        this.savedRequest = this.convertValues(source["savedRequest"], WorkspaceSavedRequest);
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
	export class RequestGetResponse {
	    ok: boolean;
	    data?: RequestGetResult;
	    error?: ErrorEnvelope;

	    static createFrom(source: any = {}) {
	        return new RequestGetResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], RequestGetResult);
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

	export class RequestSaveInput {
	    id: string;
	    name?: string;
	    method: string;
	    rpcType: string;
	    endpointRef: string;
	    environmentRef?: string;
	    metadataTemplate?: Record<string, string>;
	    callOptions?: CallOptions;
	    requestSpec: SavedRequestSpec;

	    static createFrom(source: any = {}) {
	        return new RequestSaveInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.method = source["method"];
	        this.rpcType = source["rpcType"];
	        this.endpointRef = source["endpointRef"];
	        this.environmentRef = source["environmentRef"];
	        this.metadataTemplate = source["metadataTemplate"];
	        this.callOptions = this.convertValues(source["callOptions"], CallOptions);
	        this.requestSpec = this.convertValues(source["requestSpec"], SavedRequestSpec);
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
	export class RequestSaveResult {
	    workspace: WorkspaceSnapshot;
	    savedRequest: WorkspaceSavedRequestSummary;

	    static createFrom(source: any = {}) {
	        return new RequestSaveResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspace = this.convertValues(source["workspace"], WorkspaceSnapshot);
	        this.savedRequest = this.convertValues(source["savedRequest"], WorkspaceSavedRequestSummary);
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
	export class RequestSaveResponse {
	    ok: boolean;
	    data?: RequestSaveResult;
	    error?: ErrorEnvelope;

	    static createFrom(source: any = {}) {
	        return new RequestSaveResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], RequestSaveResult);
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






	export class UpdateCheckResult {
	    currentVersion: string;
	    latestVersion: string;
	    updateAvailable: boolean;
	    releaseUrl?: string;
	    downloadUrl?: string;
	    downloadName?: string;
	    publishedAt?: string;

	    static createFrom(source: any = {}) {
	        return new UpdateCheckResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.updateAvailable = source["updateAvailable"];
	        this.releaseUrl = source["releaseUrl"];
	        this.downloadUrl = source["downloadUrl"];
	        this.downloadName = source["downloadName"];
	        this.publishedAt = source["publishedAt"];
	    }
	}
	export class UpdateCheckResponse {
	    ok: boolean;
	    data?: UpdateCheckResult;
	    error?: ErrorEnvelope;

	    static createFrom(source: any = {}) {
	        return new UpdateCheckResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], UpdateCheckResult);
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

	export class WorkspaceActiveResponse {
	    ok: boolean;
	    data?: WorkspaceSnapshot;
	    error?: ErrorEnvelope;

	    static createFrom(source: any = {}) {
	        return new WorkspaceActiveResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], WorkspaceSnapshot);
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
	export class WorkspaceCloseResponse {
	    ok: boolean;
	    error?: ErrorEnvelope;

	    static createFrom(source: any = {}) {
	        return new WorkspaceCloseResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
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
	export class WorkspaceCreateInput {
	    path: string;
	    name?: string;
	    endpoints?: EndpointPreset[];
	    protoSources?: ProtoSource[];
	    importPaths?: string[];
	    environments?: Record<string, WorkspaceEnvironment>;
	    settings?: WorkspaceSettings;

	    static createFrom(source: any = {}) {
	        return new WorkspaceCreateInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.endpoints = this.convertValues(source["endpoints"], EndpointPreset);
	        this.protoSources = this.convertValues(source["protoSources"], ProtoSource);
	        this.importPaths = source["importPaths"];
	        this.environments = this.convertValues(source["environments"], WorkspaceEnvironment, true);
	        this.settings = this.convertValues(source["settings"], WorkspaceSettings);
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


	export class WorkspaceValidationIssue {
	    field: string;
	    code: string;
	    category: string;
	    message: string;
	    path?: string;

	    static createFrom(source: any = {}) {
	        return new WorkspaceValidationIssue(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.field = source["field"];
	        this.code = source["code"];
	        this.category = source["category"];
	        this.message = source["message"];
	        this.path = source["path"];
	    }
	}
	export class WorkspaceResult {
	    workspace: WorkspaceSnapshot;
	    issues?: WorkspaceValidationIssue[];

	    static createFrom(source: any = {}) {
	        return new WorkspaceResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspace = this.convertValues(source["workspace"], WorkspaceSnapshot);
	        this.issues = this.convertValues(source["issues"], WorkspaceValidationIssue);
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
	export class WorkspaceResponse {
	    ok: boolean;
	    data?: WorkspaceResult;
	    error?: ErrorEnvelope;

	    static createFrom(source: any = {}) {
	        return new WorkspaceResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], WorkspaceResult);
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

	export class WorkspaceSaveInput {
	    name?: string;
	    endpoints?: EndpointPreset[];
	    protoSources?: ProtoSource[];
	    importPaths?: string[];
	    environments?: Record<string, WorkspaceEnvironment>;
	    settings?: WorkspaceSettings;

	    static createFrom(source: any = {}) {
	        return new WorkspaceSaveInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.endpoints = this.convertValues(source["endpoints"], EndpointPreset);
	        this.protoSources = this.convertValues(source["protoSources"], ProtoSource);
	        this.importPaths = source["importPaths"];
	        this.environments = this.convertValues(source["environments"], WorkspaceEnvironment, true);
	        this.settings = this.convertValues(source["settings"], WorkspaceSettings);
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




	export class WorkspaceValidateInput {
	    name?: string;
	    endpoints?: EndpointPreset[];
	    protoSources?: ProtoSource[];
	    importPaths?: string[];
	    environments?: Record<string, WorkspaceEnvironment>;
	    settings?: WorkspaceSettings;

	    static createFrom(source: any = {}) {
	        return new WorkspaceValidateInput(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.endpoints = this.convertValues(source["endpoints"], EndpointPreset);
	        this.protoSources = this.convertValues(source["protoSources"], ProtoSource);
	        this.importPaths = source["importPaths"];
	        this.environments = this.convertValues(source["environments"], WorkspaceEnvironment, true);
	        this.settings = this.convertValues(source["settings"], WorkspaceSettings);
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
	export class WorkspaceValidateResult {
	    workspace?: WorkspaceSnapshot;
	    issues: WorkspaceValidationIssue[];

	    static createFrom(source: any = {}) {
	        return new WorkspaceValidateResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.workspace = this.convertValues(source["workspace"], WorkspaceSnapshot);
	        this.issues = this.convertValues(source["issues"], WorkspaceValidationIssue);
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
	export class WorkspaceValidateResponse {
	    ok: boolean;
	    data?: WorkspaceValidateResult;
	    error?: ErrorEnvelope;

	    static createFrom(source: any = {}) {
	        return new WorkspaceValidateResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.data = this.convertValues(source["data"], WorkspaceValidateResult);
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

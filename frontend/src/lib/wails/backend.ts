import type {
  BootstrapResponse,
  ProbeResponse,
  BootstrapData,
  ProbeAcknowledgement,
  ErrorCategory,
  CatalogLoadFromReflectionInput,
  CatalogLoadFromReflectionResponse,
  CatalogLoadFromProtoSourcesInput,
  CatalogLoadFromProtoSourcesResponse,
  ProtoCatalogResult,
  ReflectionCatalogResult,
  CallInvokeUnaryInput,
  CallInvokeUnaryResponse,
  CallInvokeUnaryResult,
  HistoryListInput,
  HistoryListResponse,
  HistoryListResult,
  HistoryGetResponse,
  HistoryGetResult,
  EndpointTestInput,
  EndpointTestResponse,
  EndpointTestResult,
  RequestSaveInput,
  RequestSaveResponse,
  RequestSaveResult,
  WorkspaceCreateInput,
  WorkspaceResponse,
  WorkspaceResult,
  WorkspaceSaveInput,
  WorkspaceValidateInput,
  WorkspaceValidateResponse,
  WorkspaceValidateResult,
} from '../contracts'
import {
  CallInvokeUnary,
  CatalogLoadFromReflection,
  CatalogLoadFromProtoSources,
  EndpointTest,
  HistoryGet,
  HistoryList,
  RequestSave,
  ShellBootstrap,
  ShellEmitDiagnosticsProbe,
  WorkspaceCreate,
  WorkspaceOpen,
  WorkspaceSave,
  WorkspaceValidate,
} from '../../../wailsjs/go/main/App'

export class BackendResponseError extends Error {
  code?: string
  category?: ErrorCategory
  details?: Record<string, string>

  constructor(error: {
    code?: string
    category?: ErrorCategory
    message: string
    details?: Record<string, string>
  }) {
    super(error.code ? `${error.code}: ${error.message}` : error.message)
    this.name = 'BackendResponseError'
    this.code = error.code
    this.category = error.category
    this.details = error.details
  }
}

function unwrapResponse<
  T extends {
    ok: boolean
    data?: unknown
    error?: {
      code?: string
      category?: ErrorCategory
      message: string
      details?: Record<string, string>
    }
  },
>(
  response: T,
): unknown {
  if (!response.ok) {
    throw new BackendResponseError(response.error ?? { message: 'Unknown Wails bridge failure.' })
  }

  return response.data
}

export async function fetchBootstrap(): Promise<BootstrapData> {
  const response = (await ShellBootstrap()) as BootstrapResponse
  return unwrapResponse(response) as BootstrapData
}

export async function emitDiagnosticsProbe(): Promise<ProbeAcknowledgement> {
  const response = (await ShellEmitDiagnosticsProbe()) as ProbeResponse
  return unwrapResponse(response) as ProbeAcknowledgement
}

export async function createWorkspace(input: WorkspaceCreateInput): Promise<WorkspaceResult> {
  const response = (await WorkspaceCreate(input as Parameters<typeof WorkspaceCreate>[0])) as WorkspaceResponse
  return unwrapResponse(response) as WorkspaceResult
}

export async function openWorkspace(path: string): Promise<WorkspaceResult> {
  const response = (await WorkspaceOpen(path)) as WorkspaceResponse
  return unwrapResponse(response) as WorkspaceResult
}

export async function saveWorkspace(input: WorkspaceSaveInput): Promise<WorkspaceResult> {
  const response = (await WorkspaceSave(input as Parameters<typeof WorkspaceSave>[0])) as WorkspaceResponse
  return unwrapResponse(response) as WorkspaceResult
}

export async function validateWorkspace(input: WorkspaceValidateInput): Promise<WorkspaceValidateResult> {
  const response = (await WorkspaceValidate(
    input as Parameters<typeof WorkspaceValidate>[0],
  )) as WorkspaceValidateResponse
  return unwrapResponse(response) as WorkspaceValidateResult
}

export async function saveRequest(input: RequestSaveInput): Promise<RequestSaveResult> {
  const response = (await RequestSave(input as Parameters<typeof RequestSave>[0])) as RequestSaveResponse
  return unwrapResponse(response) as RequestSaveResult
}

export async function testEndpoint(input: EndpointTestInput): Promise<EndpointTestResult> {
  const response = (await EndpointTest(input as Parameters<typeof EndpointTest>[0])) as EndpointTestResponse
  return unwrapResponse(response) as EndpointTestResult
}

export async function loadCatalogFromReflection(input: CatalogLoadFromReflectionInput): Promise<ReflectionCatalogResult> {
  const response = (await CatalogLoadFromReflection(
    input as Parameters<typeof CatalogLoadFromReflection>[0],
  )) as CatalogLoadFromReflectionResponse
  return unwrapResponse(response) as ReflectionCatalogResult
}

export async function loadCatalogFromProtoSources(input: CatalogLoadFromProtoSourcesInput): Promise<ProtoCatalogResult> {
  const response = (await CatalogLoadFromProtoSources(
    input as Parameters<typeof CatalogLoadFromProtoSources>[0],
  )) as CatalogLoadFromProtoSourcesResponse
  return unwrapResponse(response) as ProtoCatalogResult
}

export async function invokeUnary(input: CallInvokeUnaryInput): Promise<CallInvokeUnaryResult> {
  const response = (await CallInvokeUnary(input as Parameters<typeof CallInvokeUnary>[0])) as CallInvokeUnaryResponse
  return unwrapResponse(response) as CallInvokeUnaryResult
}

export async function listHistory(input: HistoryListInput): Promise<HistoryListResult> {
  const response = (await HistoryList(input as Parameters<typeof HistoryList>[0])) as HistoryListResponse
  return unwrapResponse(response) as HistoryListResult
}

export async function getHistory(callId: string): Promise<HistoryGetResult> {
  const response = (await HistoryGet(callId)) as HistoryGetResponse
  return unwrapResponse(response) as HistoryGetResult
}

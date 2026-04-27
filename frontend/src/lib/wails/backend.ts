import type {
  BootstrapResponse,
  ProbeResponse,
  BootstrapData,
  ProbeAcknowledgement,
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
} from '../contracts'
import {
  CallInvokeUnary,
  CatalogLoadFromReflection,
  CatalogLoadFromProtoSources,
  EndpointTest,
  HistoryGet,
  HistoryList,
  ShellBootstrap,
  ShellEmitDiagnosticsProbe,
} from '../../../wailsjs/go/main/App'

function unwrapResponse<T extends { ok: boolean; data?: unknown; error?: { code?: string; message: string } }>(
  response: T,
): unknown {
  if (!response.ok) {
    const message = response.error?.message ?? 'Unknown Wails bridge failure.'
    throw new Error(response.error?.code ? `${response.error.code}: ${message}` : message)
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

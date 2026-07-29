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
  CallStartStreamInput,
  CallStartStreamResponse,
  CallStartStreamResult,
  CallSendMessageInput,
  CallSendMessageResponse,
  CallSendMessageResult,
  CallHalfCloseInput,
  CallHalfCloseResponse,
  CallHalfCloseResult,
  CallCancelInput,
  CallCancelResponse,
  CallCancelResult,
  HistoryListInput,
  HistoryListResponse,
  HistoryListResult,
  HistoryGetResponse,
  HistoryGetResult,
  DiagnosticsExportInput,
  DiagnosticsExportResponse,
  DiagnosticsExportResult,
  EndpointTestInput,
  EndpointTestResponse,
  EndpointTestResult,
  MaterialRegisterFileInput,
  MaterialRegisterFileResponse,
  MaterialRegisterFileResult,
  RequestGetInput,
  RequestGetResponse,
  RequestGetResult,
  RequestDeleteInput,
  RequestDeleteResponse,
  RequestDeleteResult,
  RequestSaveInput,
  RequestSaveResponse,
  RequestSaveResult,
  WorkspaceCreateInput,
  WorkspaceCloseResponse,
  WorkspaceActiveResponse,
  WorkspaceSnapshot,
  WorkspaceResponse,
  WorkspaceResult,
  WorkspaceSaveInput,
  WorkspaceValidateInput,
  WorkspaceValidateResponse,
  WorkspaceValidateResult,
  GitHubWorkspaceLinkInput,
  GitHubWorkspaceCredentialInput,
  GitHubSyncActionInput,
  GitHubSyncResponse,
  GitHubSyncStatus,
  UpdateCheckResponse,
  UpdateCheckResult,
} from '../contracts'
import {
  CallCancel,
  CallHalfClose,
  CallInvokeUnary,
  CallSendMessage,
  CallStartStream,
  CatalogLoadFromReflection,
  CatalogLoadFromProtoSources,
  DialogSelectProtoDirectory,
  DialogSelectProtoFiles,
  DiagnosticsExport,
  DialogSelectWorkspaceCreatePath,
  DialogSelectWorkspaceOpenPath,
  EndpointTest,
  HistoryGet,
  HistoryList,
  GitHubWorkspaceLink,
  GitHubWorkspaceSetToken,
  GitHubWorkspacePull,
  GitHubWorkspacePush,
  GitHubWorkspaceStatus,
  GitHubWorkspaceUnlink,
  MaterialRegisterFile,
  RequestGet,
  RequestDelete,
  RequestSave,
  ShellBootstrap,
  ShellEmitDiagnosticsProbe,
  UpdateCheck,
  WorkspaceCreate,
  WorkspaceClose,
  WorkspaceDelete,
  WorkspaceOpen,
  WorkspaceActive,
  WorkspaceSave,
  WorkspaceValidate,
  DialogSelectMaterialFile,
} from '../../../wailsjs/go/main/App'

export class BackendResponseError extends Error {
  code?: string
  category?: ErrorCategory
  details?: Record<string, string>
  backendMessage: string

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
    this.backendMessage = error.message
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

export async function checkUpdates(): Promise<UpdateCheckResult> {
  const response = (await UpdateCheck()) as UpdateCheckResponse
  return unwrapResponse(response) as UpdateCheckResult
}

export async function createWorkspace(input: WorkspaceCreateInput): Promise<WorkspaceResult> {
  const response = (await WorkspaceCreate(input as Parameters<typeof WorkspaceCreate>[0])) as WorkspaceResponse
  return unwrapResponse(response) as WorkspaceResult
}

export async function openWorkspace(path: string): Promise<WorkspaceResult> {
  const response = (await WorkspaceOpen(path)) as WorkspaceResponse
  return unwrapResponse(response) as WorkspaceResult
}

export async function getActiveWorkspace(): Promise<WorkspaceSnapshot | null> {
  const response = (await WorkspaceActive()) as WorkspaceActiveResponse
  if (!response.ok) {
    throw new BackendResponseError(response.error ?? { message: 'Could not read the active workspace.' })
  }
  return response.data ?? null
}

export async function closeWorkspace(): Promise<void> {
  const response = (await WorkspaceClose()) as WorkspaceCloseResponse
  unwrapResponse(response)
}

export async function deleteWorkspace(): Promise<void> {
  const response = (await WorkspaceDelete()) as WorkspaceCloseResponse
  unwrapResponse(response)
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

export async function linkGitHubWorkspace(input: GitHubWorkspaceLinkInput): Promise<GitHubSyncStatus> {
  const response = (await GitHubWorkspaceLink(input as Parameters<typeof GitHubWorkspaceLink>[0])) as GitHubSyncResponse
  return unwrapResponse(response) as GitHubSyncStatus
}

export async function setGitHubWorkspaceToken(input: GitHubWorkspaceCredentialInput): Promise<GitHubSyncStatus> {
  const response = (await GitHubWorkspaceSetToken(input as Parameters<typeof GitHubWorkspaceSetToken>[0])) as GitHubSyncResponse
  return unwrapResponse(response) as GitHubSyncStatus
}

export async function getGitHubWorkspaceStatus(): Promise<GitHubSyncStatus> {
  const response = (await GitHubWorkspaceStatus()) as GitHubSyncResponse
  return unwrapResponse(response) as GitHubSyncStatus
}

export async function pullGitHubWorkspace(input: GitHubSyncActionInput = { overwrite: false }): Promise<GitHubSyncStatus> {
  const response = (await GitHubWorkspacePull(input as Parameters<typeof GitHubWorkspacePull>[0])) as GitHubSyncResponse
  return unwrapResponse(response) as GitHubSyncStatus
}

export async function pushGitHubWorkspace(input: GitHubSyncActionInput = { overwrite: false }): Promise<GitHubSyncStatus> {
  const response = (await GitHubWorkspacePush(input as Parameters<typeof GitHubWorkspacePush>[0])) as GitHubSyncResponse
  return unwrapResponse(response) as GitHubSyncStatus
}

export async function unlinkGitHubWorkspace(): Promise<GitHubSyncStatus> {
  const response = (await GitHubWorkspaceUnlink()) as GitHubSyncResponse
  return unwrapResponse(response) as GitHubSyncStatus
}

export async function selectWorkspaceCreatePath(): Promise<string> {
  return (await DialogSelectWorkspaceCreatePath()) as string
}

export async function selectWorkspaceOpenPath(): Promise<string> {
  return (await DialogSelectWorkspaceOpenPath()) as string
}

export async function saveRequest(input: RequestSaveInput): Promise<RequestSaveResult> {
  const response = (await RequestSave(input as Parameters<typeof RequestSave>[0])) as RequestSaveResponse
  return unwrapResponse(response) as RequestSaveResult
}

export async function getSavedRequest(input: RequestGetInput): Promise<RequestGetResult> {
  const response = (await RequestGet(input as Parameters<typeof RequestGet>[0])) as RequestGetResponse
  return unwrapResponse(response) as RequestGetResult
}

export async function deleteSavedRequest(input: RequestDeleteInput): Promise<RequestDeleteResult> {
  const response = (await RequestDelete(input as Parameters<typeof RequestDelete>[0])) as RequestDeleteResponse
  return unwrapResponse(response) as RequestDeleteResult
}

export async function testEndpoint(input: EndpointTestInput): Promise<EndpointTestResult> {
  const response = (await EndpointTest(input as Parameters<typeof EndpointTest>[0])) as EndpointTestResponse
  return unwrapResponse(response) as EndpointTestResult
}

export async function registerMaterialFile(input: MaterialRegisterFileInput): Promise<MaterialRegisterFileResult> {
  const response = (await MaterialRegisterFile(
    input as Parameters<typeof MaterialRegisterFile>[0],
  )) as MaterialRegisterFileResponse
  return unwrapResponse(response) as MaterialRegisterFileResult
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

export async function selectProtoFiles(): Promise<string[]> {
  return (await DialogSelectProtoFiles()) as string[]
}

export async function selectProtoDirectory(): Promise<string> {
  return (await DialogSelectProtoDirectory()) as string
}

export async function selectMaterialFile(): Promise<string> {
  return (await DialogSelectMaterialFile()) as string
}

export async function invokeUnary(input: CallInvokeUnaryInput): Promise<CallInvokeUnaryResult> {
  const response = (await CallInvokeUnary(input as Parameters<typeof CallInvokeUnary>[0])) as CallInvokeUnaryResponse
  return unwrapResponse(response) as CallInvokeUnaryResult
}

export async function startStream(input: CallStartStreamInput): Promise<CallStartStreamResult> {
  const response = (await CallStartStream(input as Parameters<typeof CallStartStream>[0])) as CallStartStreamResponse
  return unwrapResponse(response) as CallStartStreamResult
}

export async function sendStreamMessage(input: CallSendMessageInput): Promise<CallSendMessageResult> {
  const response = (await CallSendMessage(input as Parameters<typeof CallSendMessage>[0])) as CallSendMessageResponse
  return unwrapResponse(response) as CallSendMessageResult
}

export async function halfCloseStream(input: CallHalfCloseInput): Promise<CallHalfCloseResult> {
  const response = (await CallHalfClose(input as Parameters<typeof CallHalfClose>[0])) as CallHalfCloseResponse
  return unwrapResponse(response) as CallHalfCloseResult
}

export async function cancelStream(input: CallCancelInput): Promise<CallCancelResult> {
  const response = (await CallCancel(input as Parameters<typeof CallCancel>[0])) as CallCancelResponse
  return unwrapResponse(response) as CallCancelResult
}

export async function listHistory(input: HistoryListInput): Promise<HistoryListResult> {
  const response = (await HistoryList(input as Parameters<typeof HistoryList>[0])) as HistoryListResponse
  return unwrapResponse(response) as HistoryListResult
}

export async function getHistory(callId: string): Promise<HistoryGetResult> {
  const response = (await HistoryGet(callId)) as HistoryGetResponse
  return unwrapResponse(response) as HistoryGetResult
}

export async function exportDiagnostics(input: DiagnosticsExportInput): Promise<DiagnosticsExportResult> {
  const response = (await DiagnosticsExport(
    input as Parameters<typeof DiagnosticsExport>[0],
  )) as DiagnosticsExportResponse
  return unwrapResponse(response) as DiagnosticsExportResult
}

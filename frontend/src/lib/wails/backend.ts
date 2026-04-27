import type {
  BootstrapResponse,
  ProbeResponse,
  BootstrapData,
  ProbeAcknowledgement,
  EndpointTestInput,
  EndpointTestResponse,
  EndpointTestResult,
} from '../contracts'
import { EndpointTest, ShellBootstrap, ShellEmitDiagnosticsProbe } from '../../../wailsjs/go/main/App'

function unwrapResponse<T extends { ok: boolean; data?: unknown; error?: { message: string } }>(
  response: T,
): unknown {
  if (!response.ok) {
    throw new Error(response.error?.message ?? 'Unknown Wails bridge failure.')
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

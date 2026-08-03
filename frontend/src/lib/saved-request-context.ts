export interface EndpointScopedSavedRequest {
  id: string
  endpointRef: string
}

export type AppliedSavedRequests<T extends EndpointScopedSavedRequest> = Record<string, T>

export function applySavedRequestToEndpoint<T extends EndpointScopedSavedRequest>(
  applied: AppliedSavedRequests<T>,
  request: T,
): AppliedSavedRequests<T> {
  return {
    ...applied,
    [request.endpointRef]: request,
  }
}

export function appliedSavedRequestForEndpoint<T extends EndpointScopedSavedRequest>(
  applied: AppliedSavedRequests<T>,
  endpointRef: string | undefined,
): T | null {
  return endpointRef ? applied[endpointRef] ?? null : null
}

export function clearAppliedSavedRequest<T extends EndpointScopedSavedRequest>(
  applied: AppliedSavedRequests<T>,
  endpointRef: string | undefined,
): AppliedSavedRequests<T> {
  if (!endpointRef || !(endpointRef in applied)) {
    return applied
  }

  const next = { ...applied }
  delete next[endpointRef]
  return next
}

export function replaceAppliedSavedRequest<T extends EndpointScopedSavedRequest>(
  applied: AppliedSavedRequests<T>,
  request: T,
): AppliedSavedRequests<T> {
  return applied[request.endpointRef]?.id === request.id
    ? applySavedRequestToEndpoint(applied, request)
    : applied
}

export function removeAppliedSavedRequest<T extends EndpointScopedSavedRequest>(
  applied: AppliedSavedRequests<T>,
  requestId: string,
): AppliedSavedRequests<T> {
  const endpointRef = Object.entries(applied).find(([, request]) => request.id === requestId)?.[0]
  return clearAppliedSavedRequest(applied, endpointRef)
}

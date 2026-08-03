import { describe, expect, it } from 'vitest'
import {
  appliedSavedRequestForEndpoint,
  applySavedRequestToEndpoint,
  clearAppliedSavedRequest,
} from './saved-request-context'

const request = {
  id: 'create-user-default',
  endpointRef: 'users-primary',
}

describe('saved request endpoint context', () => {
  it('restores a request indicator only when returning to its endpoint', () => {
    const applied = applySavedRequestToEndpoint({}, request)

    expect(appliedSavedRequestForEndpoint(applied, 'users-primary')).toEqual(request)
    expect(appliedSavedRequestForEndpoint(applied, 'users-secondary')).toBeNull()
    expect(appliedSavedRequestForEndpoint(applied, 'users-primary')).toEqual(request)
  })

  it('clears only the endpoint where a different method was selected', () => {
    const applied = applySavedRequestToEndpoint(
      applySavedRequestToEndpoint({}, request),
      { id: 'list-orders-default', endpointRef: 'orders' },
    )

    const afterSelection = clearAppliedSavedRequest(applied, 'users-primary')

    expect(appliedSavedRequestForEndpoint(afterSelection, 'users-primary')).toBeNull()
    expect(appliedSavedRequestForEndpoint(afterSelection, 'orders')).toEqual({
      id: 'list-orders-default',
      endpointRef: 'orders',
    })
  })
})

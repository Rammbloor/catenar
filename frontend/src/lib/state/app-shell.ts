import { writable } from 'svelte/store'
import type {
  BootstrapData,
  DiagnosticsUpdateEvent,
  EventProbeState,
  ProbeAcknowledgement,
  SessionCondition,
  StreamCompletedEvent,
  StreamErrorEvent,
  StreamEventRecord,
  StreamStateEvent,
  StreamState,
} from '../contracts'
import { STREAM_TRANSITIONS, TERMINAL_STREAM_STATES, verifyContractManifest } from '../contracts'
import { appendOrderedStreamEvent } from '../stream-timeline'

export interface AppShellState {
  bootstrap?: BootstrapData
  bootstrapError?: string
  contractMismatch: string[]
  activeStreamState: StreamState
  activeStreamSessionId?: string
  activeStreamCallId?: string
  streamConditions: SessionCondition[]
  streamEvents: StreamEventRecord[]
  streamErrors: StreamErrorEvent[]
  lastStreamCompleted?: StreamCompletedEvent
  diagnostics: DiagnosticsUpdateEvent[]
  eventProbe: EventProbeState
}

const initialState: AppShellState = {
  contractMismatch: [],
  activeStreamState: 'idle',
  streamConditions: [],
  streamEvents: [],
  streamErrors: [],
  diagnostics: [],
  eventProbe: {
    pending: false,
  },
}

function createInvalidTransitionEvent(from: StreamState, to: StreamState): DiagnosticsUpdateEvent {
  return {
    id: `diag_invalid_${from}_${to}`,
    source: 'ui-shell',
    level: 'warning',
    code: 'application.invalid_state_transition',
    category: 'application',
    message: `Blocked invalid stream state transition from ${from} to ${to}.`,
    nextStep: 'Review the canonical stream state machine before dispatching the next session update.',
    details: {
      from,
      to,
    },
    ts: new Date().toISOString(),
  }
}

export function canTransition(from: StreamState, to: StreamState): boolean {
  if (from === to) {
    return true
  }

  return STREAM_TRANSITIONS.some((transition) => transition.from === from && transition.to.includes(to))
}

function isTerminalStreamState(state: StreamState): boolean {
  return TERMINAL_STREAM_STATES.some((terminalState) => terminalState === state)
}

export function createAppShellStore() {
  const { subscribe, update } = writable<AppShellState>(initialState)

  return {
    subscribe,
    hydrateBootstrap(bootstrap: BootstrapData) {
      update((state) => ({
        ...state,
        bootstrap,
        bootstrapError: undefined,
        contractMismatch: verifyContractManifest(bootstrap.contract),
      }))
    },
    setBootstrapError(message: string) {
      update((state) => ({
        ...state,
        bootstrapError: message,
      }))
    },
    setStreamState(nextState: StreamState) {
      update((state) => {
        const startsNewSession = nextState === 'connecting' && isTerminalStreamState(state.activeStreamState)
        if (!startsNewSession && !canTransition(state.activeStreamState, nextState)) {
          return {
            ...state,
            diagnostics: [createInvalidTransitionEvent(state.activeStreamState, nextState), ...state.diagnostics],
          }
        }

        return {
          ...state,
          activeStreamState: nextState,
        }
      })
    },
    setStreamConditions(conditions: SessionCondition[]) {
      update((state) => ({
        ...state,
        streamConditions: conditions,
      }))
    },
    applyStreamStateEvent(event: StreamStateEvent) {
      update((state) => {
        const startsNewSession = event.state === 'connecting' && isTerminalStreamState(state.activeStreamState)
        if (!startsNewSession && !canTransition(state.activeStreamState, event.state)) {
          return {
            ...state,
            diagnostics: [createInvalidTransitionEvent(state.activeStreamState, event.state), ...state.diagnostics],
          }
        }

        return {
          ...state,
          activeStreamState: event.state,
          activeStreamSessionId: event.sessionId,
          activeStreamCallId: event.callId,
          streamConditions: event.conditions,
        }
      })
    },
    applyStreamEvent(event: StreamEventRecord) {
      update((state) => {
        const streamEvents = [...state.streamEvents]
        appendOrderedStreamEvent(streamEvents, event)

        return {
          ...state,
          activeStreamSessionId: event.sessionId,
          activeStreamCallId: event.callId,
          streamEvents: streamEvents.slice(-1000),
        }
      })
    },
    applyStreamError(event: StreamErrorEvent) {
      update((state) => ({
        ...state,
        activeStreamState: 'error',
        activeStreamSessionId: event.sessionId,
        activeStreamCallId: event.callId,
        streamErrors: [event, ...state.streamErrors].slice(0, 25),
      }))
    },
    applyStreamCompleted(event: StreamCompletedEvent) {
      update((state) => ({
        ...state,
        activeStreamState: event.finalState,
        activeStreamSessionId: event.sessionId,
        activeStreamCallId: event.callId,
        streamConditions: event.conditions,
        lastStreamCompleted: event,
      }))
    },
    applyDiagnosticsEvent(event: DiagnosticsUpdateEvent) {
      update((state) => ({
        ...state,
        diagnostics: [event, ...state.diagnostics].slice(0, 25),
      }))
    },
    startProbe() {
      update((state) => ({
        ...state,
        eventProbe: {
          ...state.eventProbe,
          pending: true,
          error: undefined,
        },
      }))
    },
    finishProbe(acknowledgement: ProbeAcknowledgement) {
      update((state) => ({
        ...state,
        eventProbe: {
          pending: false,
          lastAcknowledgement: acknowledgement,
          error: undefined,
        },
      }))
    },
    setProbeError(message: string) {
      update((state) => ({
        ...state,
        eventProbe: {
          ...state.eventProbe,
          pending: false,
          error: message,
        },
      }))
    },
  }
}

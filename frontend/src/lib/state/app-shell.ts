import { writable } from 'svelte/store'
import type {
  AppOverlay,
  AppView,
  BootstrapData,
  DiagnosticsUpdateEvent,
  EventProbeState,
  ProbeAcknowledgement,
  SessionCondition,
  StreamState,
} from '../contracts'
import { STREAM_TRANSITIONS, TERMINAL_STREAM_STATES, verifyContractManifest } from '../contracts'

export interface AppShellState {
  bootstrap?: BootstrapData
  bootstrapError?: string
  contractMismatch: string[]
  currentView: AppView
  activeOverlay: AppOverlay | null
  activeStreamState: StreamState
  streamConditions: SessionCondition[]
  diagnostics: DiagnosticsUpdateEvent[]
  eventProbe: EventProbeState
}

const initialState: AppShellState = {
  contractMismatch: [],
  currentView: 'home',
  activeOverlay: null,
  activeStreamState: 'idle',
  streamConditions: [],
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
    openView(view: AppView) {
      update((state) => ({
        ...state,
        currentView: view,
      }))
    },
    openOverlay(overlay: AppOverlay) {
      update((state) => ({
        ...state,
        activeOverlay: overlay,
      }))
    },
    closeOverlay() {
      update((state) => ({
        ...state,
        activeOverlay: null,
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

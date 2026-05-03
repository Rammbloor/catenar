import { describe, expect, it } from 'vitest'

import type { StreamEventRecord } from './contracts'
import {
  appendOrderedStreamEvent,
  computeTimelineWindow,
  filterStreamEvents,
  findFirstErrorEvent,
  isNearTimelineTail,
  streamEventKindOptions,
  timelineTailScrollTop,
} from './stream-timeline'

function event(seq: number, kind = 'message_received', direction = 'received'): StreamEventRecord {
  return {
    sessionId: 'sess_1',
    callId: 'call_1',
    seq,
    kind,
    direction,
    ts: `2026-05-02T00:00:${String(seq).padStart(2, '0')}Z`,
    payload: {
      preview: {
        json: { seq },
      },
    },
  }
}

describe('stream timeline helpers', () => {
  it('keeps stream append order without sorting the whole event buffer', () => {
    const events = [event(1), event(3)]

    expect(appendOrderedStreamEvent(events, event(4))).toBe(true)
    expect(appendOrderedStreamEvent(events, event(2))).toBe(true)
    expect(appendOrderedStreamEvent(events, event(2))).toBe(false)

    expect(events.map((item) => item.seq)).toEqual([1, 2, 3, 4])
  })

  it('computes a bounded render window with spacer heights', () => {
    const events = Array.from({ length: 1000 }, (_, index) => event(index + 1))
    const window = computeTimelineWindow(events, {
      scrollTop: 40 * 10,
      viewportHeight: 40 * 20,
      rowHeight: 40,
      targetRows: 50,
      overscanRows: 5,
    })

    expect(window.renderedCount).toBe(50)
    expect(window.startIndex).toBe(5)
    expect(window.endIndex).toBe(55)
    expect(window.beforeHeightPx).toBe(200)
    expect(window.afterHeightPx).toBe(37800)
  })

  it('filters by direction and kind while preserving first-seen kind options', () => {
    const events = [
      event(1, 'call_started', 'sent'),
      event(2, 'message_sent', 'sent'),
      event(3, 'message_received', 'received'),
      event(4, 'call_finished', 'received'),
    ]

    expect(filterStreamEvents(events, { direction: 'sent', kind: 'all' }).map((item) => item.seq)).toEqual([1, 2])
    expect(filterStreamEvents(events, { direction: 'all', kind: 'message_received' }).map((item) => item.seq)).toEqual([
      3,
    ])
    expect(streamEventKindOptions(events)).toEqual([
      'call_started',
      'message_sent',
      'message_received',
      'call_finished',
    ])
  })

  it('supports live tail detection and jump-to-error targeting', () => {
    const events = [event(1), event(2, 'call_failed', 'received')]

    expect(isNearTimelineTail(10, timelineTailScrollTop(10, 120, 40), 120, 40)).toBe(true)
    expect(isNearTimelineTail(10, 0, 120, 40)).toBe(false)
    expect(findFirstErrorEvent(events)?.seq).toBe(2)
  })
})

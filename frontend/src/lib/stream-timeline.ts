import type { StreamEventRecord } from './contracts'

export const TIMELINE_ROW_HEIGHT_PX = 184
export const TIMELINE_TARGET_ROWS = 200
export const TIMELINE_OVERSCAN_ROWS = 24
export const TIMELINE_TAIL_THRESHOLD_ROWS = 2

export type TimelineDirectionFilter = 'all' | 'sent' | 'received'

export interface TimelineFilters {
  direction: TimelineDirectionFilter
  kind: string
}

export interface TimelineWindow<T> {
  items: T[]
  startIndex: number
  endIndex: number
  beforeHeightPx: number
  afterHeightPx: number
  totalCount: number
  renderedCount: number
}

export interface TimelineWindowOptions {
  scrollTop?: number
  viewportHeight?: number
  rowHeight?: number
  targetRows?: number
  overscanRows?: number
}

function finiteNonNegative(value: number | undefined, fallback: number): number {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value : fallback
}

export function appendOrderedStreamEvent(events: StreamEventRecord[], event: StreamEventRecord): boolean {
  const duplicate = events.some(
    (existingEvent) => existingEvent.sessionId === event.sessionId && existingEvent.seq === event.seq,
  )
  if (duplicate) {
    return false
  }

  const lastEvent = events[events.length - 1]
  if (!lastEvent || event.seq > lastEvent.seq) {
    events.push(event)
    return true
  }

  const insertAt = events.findIndex((existingEvent) => existingEvent.seq > event.seq)
  events.splice(insertAt === -1 ? events.length : insertAt, 0, event)
  return true
}

export function filterStreamEvents(
  events: readonly StreamEventRecord[],
  filters: TimelineFilters,
): StreamEventRecord[] {
  return events.filter((event) => {
    const directionMatches = filters.direction === 'all' || event.direction === filters.direction
    const kindMatches = filters.kind === 'all' || event.kind === filters.kind
    return directionMatches && kindMatches
  })
}

export function streamEventKindOptions(events: readonly StreamEventRecord[]): string[] {
  const kinds = new Set<string>()
  for (const event of events) {
    if (event.kind) {
      kinds.add(event.kind)
    }
  }

  return [...kinds]
}

export function computeTimelineWindow<T>(
  items: readonly T[],
  options: TimelineWindowOptions = {},
): TimelineWindow<T> {
  const rowHeight = Math.max(1, finiteNonNegative(options.rowHeight, TIMELINE_ROW_HEIGHT_PX))
  const targetRows = Math.max(1, Math.floor(finiteNonNegative(options.targetRows, TIMELINE_TARGET_ROWS)))
  const overscanRows = Math.max(0, Math.floor(finiteNonNegative(options.overscanRows, TIMELINE_OVERSCAN_ROWS)))
  const viewportHeight = finiteNonNegative(options.viewportHeight, rowHeight * targetRows)
  const scrollTop = finiteNonNegative(options.scrollTop, 0)
  const totalCount = items.length

  if (totalCount === 0) {
    return {
      items: [],
      startIndex: 0,
      endIndex: 0,
      beforeHeightPx: 0,
      afterHeightPx: 0,
      totalCount: 0,
      renderedCount: 0,
    }
  }

  const visibleRows = Math.max(1, Math.ceil(viewportHeight / rowHeight))
  const maxRows = Math.min(totalCount, Math.max(targetRows, visibleRows + overscanRows * 2))
  const firstVisibleIndex = Math.min(totalCount - 1, Math.floor(scrollTop / rowHeight))
  const startIndex = Math.max(0, Math.min(totalCount - maxRows, firstVisibleIndex - overscanRows))
  const endIndex = Math.min(totalCount, startIndex + maxRows)
  const visibleItems = items.slice(startIndex, endIndex)

  return {
    items: visibleItems,
    startIndex,
    endIndex,
    beforeHeightPx: startIndex * rowHeight,
    afterHeightPx: Math.max(0, (totalCount - endIndex) * rowHeight),
    totalCount,
    renderedCount: visibleItems.length,
  }
}

export function isNearTimelineTail(
  totalCount: number,
  scrollTop: number,
  viewportHeight: number,
  rowHeight = TIMELINE_ROW_HEIGHT_PX,
  thresholdRows = TIMELINE_TAIL_THRESHOLD_ROWS,
): boolean {
  const totalHeight = Math.max(0, totalCount * rowHeight)
  const distanceFromTail = totalHeight - (scrollTop + viewportHeight)
  return distanceFromTail <= rowHeight * thresholdRows
}

export function timelineScrollTopForIndex(index: number, rowHeight = TIMELINE_ROW_HEIGHT_PX): number {
  return Math.max(0, Math.floor(index) * rowHeight)
}

export function timelineTailScrollTop(
  totalCount: number,
  viewportHeight: number,
  rowHeight = TIMELINE_ROW_HEIGHT_PX,
): number {
  return Math.max(0, totalCount * rowHeight - viewportHeight)
}

export function isErrorTimelineEvent(event: StreamEventRecord): boolean {
  const kind = event.kind.toLowerCase()
  return kind.includes('error') || kind.includes('failed')
}

export function findFirstErrorEvent(events: readonly StreamEventRecord[]): StreamEventRecord | null {
  return events.find(isErrorTimelineEvent) ?? null
}

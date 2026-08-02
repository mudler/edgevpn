import { describe, it, expect } from 'vitest'
import { bytesToSize, truncateID, formatRate } from './format'

describe('bytesToSize', () => {
  it('returns 0 B for zero', () => {
    expect(bytesToSize(0)).toBe('0 B')
  })
  it('formats bytes without a decimal', () => {
    expect(bytesToSize(512)).toBe('512 B')
  })
  it('formats kilobytes to one decimal', () => {
    expect(bytesToSize(1536)).toBe('1.5 kB')
  })
  it('formats megabytes to one decimal', () => {
    expect(bytesToSize(1_572_864)).toBe('1.5 MB')
  })
  it('handles gigabytes', () => {
    expect(bytesToSize(3_221_225_472)).toBe('3.0 GB')
  })
  it('treats negative input as zero', () => {
    expect(bytesToSize(-1)).toBe('0 B')
  })
  it('keeps a unit for fractional values', () => {
    // A negative exponent used to index past the start of UNITS, printing
    // "424.4 undefined". Live per-peer rates are routinely below 1 B/s.
    expect(bytesToSize(0.4145)).toBe('0 B')
    expect(bytesToSize(0.9)).toBe('1 B')
    expect(bytesToSize(0.001)).toBe('0 B')
  })
  it('caps at the largest known unit', () => {
    expect(bytesToSize(Math.pow(1024, 7))).toContain('PB')
  })
})

describe('truncateID', () => {
  it('shortens a long peer ID from both ends', () => {
    expect(truncateID('12D3KooWKzabcdefghijklmnop', 6)).toBe('12D3Ko…klmnop')
  })
  it('leaves short IDs alone', () => {
    expect(truncateID('short', 6)).toBe('short')
  })
  it('handles an empty string', () => {
    expect(truncateID('', 6)).toBe('')
  })
})

describe('formatRate', () => {
  it('appends a per-second suffix', () => {
    expect(formatRate(1536)).toBe('1.5 kB/s')
  })
  it('never emits an undefined unit for a sub-byte rate', () => {
    expect(formatRate(0.4145)).toBe('0 B/s')
    expect(formatRate(36.6 / 1024)).not.toContain('undefined')
  })
})

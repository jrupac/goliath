import type { Mock } from 'vitest';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { GetVersion } from '../goliath';

describe('GetVersion', () => {
  let mockFetch: Mock;

  beforeEach(() => {
    mockFetch = vi.fn();
    global.fetch = mockFetch;
  });

  const respondWith = (body: object) =>
    mockFetch.mockResolvedValue({
      ok: true,
      text: vi.fn().mockResolvedValue(JSON.stringify(body)),
    });

  it('should return version data on successful fetch', async () => {
    respondWith({
      build_timestamp: '2023-10-27T10:00:00Z',
      build_hash: 'abcdef1234567890',
      schema_version: 28,
      db_schema_version: 28,
    });

    const versionData = await GetVersion();

    expect(mockFetch).toHaveBeenCalledWith('/version', {
      credentials: 'include',
    });
    expect(versionData).toEqual({
      build_timestamp: '2023-10-27T10:00:00Z',
      build_hash: 'abcdef1234567890',
      schema: 'v28',
    });
  });

  it('should name the database schema when it differs', async () => {
    respondWith({
      build_timestamp: '2023-10-27T10:00:00Z',
      build_hash: 'abcdef1234567890',
      schema_version: 28,
      db_schema_version: 29,
    });

    const versionData = await GetVersion();
    expect(versionData.schema).toEqual('v28 (database v29)');
  });

  it('should report an unknown schema from a server that sends none', async () => {
    respondWith({
      build_timestamp: '2023-10-27T10:00:00Z',
      build_hash: 'abcdef1234567890',
    });

    const versionData = await GetVersion();
    expect(versionData.schema).toEqual('<unknown>');
  });

  it('should return default version data on failed fetch', async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      text: vi.fn().mockRejectedValue('Server error'), // Simulate error
    });

    const versionData = await GetVersion();

    expect(mockFetch).toHaveBeenCalledWith('/version', {
      credentials: 'include',
    });
    expect(versionData).toEqual({
      build_timestamp: '<unknown>',
      build_hash: '<unknown>',
      schema: '<unknown>',
    });
  });

  it('should return default version data when parsing fails', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      text: vi.fn().mockResolvedValue('{'), // invalid returned value
    });

    const versionData = await GetVersion();
    expect(mockFetch).toHaveBeenCalledWith('/version', {
      credentials: 'include',
    });
    expect(versionData).toEqual({
      build_timestamp: '<unknown>',
      build_hash: '<unknown>',
      schema: '<unknown>',
    });
  });
});

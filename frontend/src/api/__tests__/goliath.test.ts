import {beforeEach, describe, expect, it, vi} from 'vitest';
import {GetVersion, VersionData} from '../goliath';

describe('GetVersion', () => {
  let mockFetch: vi.Mock;

  beforeEach(() => {
    mockFetch = vi.fn();
    global.fetch = mockFetch;
  });

  it('should return version data on successful fetch', async () => {
    const mockVersionData: VersionData = {
      build_timestamp: '2023-10-27T10:00:00Z',
      build_hash: 'abcdef1234567890',
    };
    mockFetch.mockResolvedValue({
      ok: true,
      text: vi.fn().mockResolvedValue(JSON.stringify(mockVersionData)),
    });

    const versionData = await GetVersion();

    expect(mockFetch).toHaveBeenCalledWith('/version', {
      credentials: 'include',
    });
    expect(versionData).toEqual(mockVersionData);
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
    });
  });

  it('should return default version data when parsing fails', async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      text: vi.fn().mockResolvedValue("{") // invalid returned value
    });

    const versionData = await GetVersion();
    expect(mockFetch).toHaveBeenCalledWith('/version', {
      credentials: 'include',
    });
    expect(versionData).toEqual({
      build_timestamp: '<unknown>',
      build_hash: '<unknown>',
    });
  });
});
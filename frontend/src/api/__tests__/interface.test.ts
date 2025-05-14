import {describe, expect, it} from 'vitest';
import {FetchAPIFactory} from '../interface';
import GReader from '../greader';

describe('FetchAPIFactory', () => {
  it('should return a GReader instance when FetchType.GReader is provided', () => {
    const api = FetchAPIFactory.Create();
    expect(api).toBeInstanceOf(GReader);
  });
});
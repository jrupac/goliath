import {describe, expect, it} from 'vitest';
import {FetchAPIFactory, FetchType} from '../interface';
import Fever from '../fever';
import GReader from '../greader';

describe('FetchAPIFactory', () => {
  it('should return a Fever instance when FetchType.Fever is provided', () => {
    const api = FetchAPIFactory.Create(FetchType.Fever);
    expect(api).toBeInstanceOf(Fever);
  });

  it('should return a GReader instance when FetchType.GReader is provided', () => {
    const api = FetchAPIFactory.Create(FetchType.GReader);
    expect(api).toBeInstanceOf(GReader);
  });
});
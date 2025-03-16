import React from 'react';
import {beforeEach, describe, it, vi} from 'vitest';
import {render} from '@testing-library/react';
import App from './App';

describe('App', () => {
  beforeEach(() => {
    vi.mock('./api/goliath', () => {
      return {
        GetVersion: vi.fn().mockResolvedValue({
          build_timestamp: 'test',
          build_hash: 'test',
        }),
      };
    });
    vi.mock('./api/greader', () => {
      const GReader = vi.fn();
      GReader.prototype.VerifyAuth = vi.fn().mockResolvedValue(true);
      GReader.prototype.InitializeContent = vi.fn().mockResolvedValue({});
      return {
        default: GReader,
      };
    });
  });

  it('renders without crashing', async () => {
    render(<App/>);
  });
});
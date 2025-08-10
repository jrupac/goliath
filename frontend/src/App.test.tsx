import React from 'react';
import { beforeEach, describe, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import App from './App';
import { ContentTreeCls } from './models/contentTree';

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
      class MockGReader {
        VerifyAuth = vi.fn().mockResolvedValue(true);
        InitializeContent = vi.fn().mockResolvedValue(
          (() => {
            const mockContentTree = ContentTreeCls.new();
            mockContentTree.UnreadCount = vi.fn().mockReturnValue(0);
            mockContentTree.GetFolderFeedView = vi
              .fn()
              .mockReturnValue(new Map());
            mockContentTree.GetArticleView = vi.fn().mockReturnValue([]);
            mockContentTree.GetFaviconMap = vi.fn().mockReturnValue(new Map());
            return mockContentTree;
          })()
        );
      }
      return {
        default: MockGReader,
      };
    });
  });

  it('renders without crashing', async () => {
    render(<App />);
    await screen.findByText('Goliath');
  });
});

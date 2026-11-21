import { describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen } from '@testing-library/react';
import App from './App';
import { markUpdatePending } from './utils/swUpdate';
import { ContentTreeCls } from './models/contentTree';

// Whether a build is waiting is page-wide state that never goes back, so
// this lives apart from the rest of the App tests rather than leaving every
// test after it running against a marked chrome.

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
    ResumeSession = vi.fn().mockResolvedValue(true);
    Logout = vi.fn().mockResolvedValue(undefined);
    InitializeContent = vi.fn().mockResolvedValue(
      (() => {
        const tree = ContentTreeCls.new();
        tree.UnreadCount = vi.fn().mockReturnValue(0);
        tree.GetFolderFeedView = vi.fn().mockReturnValue(new Map());
        tree.GetArticleView = vi.fn().mockReturnValue([]);
        tree.GetFaviconMap = vi.fn().mockReturnValue(new Map());
        return tree;
      })()
    );
  }
  return { default: MockGReader };
});

describe('a waiting build', () => {
  it('marks the control that opens the offer, and offers it there', async () => {
    render(<App />);
    await screen.findByText('Goliath');

    // Nothing is claimed until a build is actually waiting.
    expect(screen.getByLabelText('Account menu')).toBeInTheDocument();
    expect(screen.queryByText('Reload for new version')).toBeNull();

    await act(async () => {
      markUpdatePending();
    });

    const account = screen.getByLabelText('Account menu, new version ready');
    expect(account.className).toContain('GoliathButtonMarked');

    fireEvent.click(account);
    expect(
      await screen.findByText('Reload for new version')
    ).toBeInTheDocument();
  });
});

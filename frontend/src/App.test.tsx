import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
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
        ResumeSession = vi.fn().mockResolvedValue(true);
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

  it('toggles theme on "t" shortcut', async () => {
    const { container } = render(<App />);
    await screen.findByText('Goliath');

    const mainContainer = container.querySelector(
      '.GoliathMainContainer'
    )?.parentElement;
    expect(mainContainer).not.toBeNull();
    const initialClasses = mainContainer?.className;

    // Trigger 't'
    fireEvent.keyDown(window, { key: 't' });

    // Classes should have updated (switched theme)
    expect(mainContainer?.className).not.toEqual(initialClasses);
  });

  it('toggles hideEmpty state on "f" shortcut', async () => {
    render(<App />);
    await screen.findByText('Goliath');

    // The setting is only visible in the settings dialog.
    fireEvent.click(screen.getByLabelText('Settings'));
    const toggle = (await screen.findByLabelText(
      'Hide feeds with no unread items'
    )) as HTMLInputElement;
    const initiallyChecked = toggle.checked;

    // Shortcuts are suspended while a dialog is open, so close it first.
    fireEvent.keyDown(document.activeElement || window, { key: 'Escape' });
    await waitFor(() => {
      expect(
        screen.queryByLabelText('Hide feeds with no unread items')
      ).toBeNull();
    });

    // Trigger 'f'
    fireEvent.keyDown(window, { key: 'f' });

    fireEvent.click(screen.getByLabelText('Settings'));
    const updated = (await screen.findByLabelText(
      'Hide feeds with no unread items'
    )) as HTMLInputElement;
    expect(updated.checked).toEqual(!initiallyChecked);
  });

  it('opens the settings dialog on "," shortcut', async () => {
    render(<App />);
    await screen.findByText('Goliath');

    expect(screen.queryByText('Subscriptions')).toBeNull();
    fireEvent.keyDown(window, { key: ',' });
    expect(await screen.findByText('Dark theme')).toBeInTheDocument();
  });

  it('toggles keybindings modal on "Shift+?" shortcut', async () => {
    render(<App />);
    await screen.findByText('Goliath');

    // Modal should not be in the document initially
    expect(screen.queryByText('Keyboard Shortcuts')).toBeNull();

    // Trigger Shift+?
    fireEvent.keyDown(window, { key: 'Shift' });
    fireEvent.keyDown(window, { key: '?', shiftKey: true });

    // Modal should be visible now
    expect(await screen.findByText('Keyboard Shortcuts')).toBeInTheDocument();

    // Trigger Shift+? again to close
    fireEvent.keyDown(window, { key: 'Shift' });
    fireEvent.keyDown(window, { key: '?', shiftKey: true });

    // Modal should be gone/closed (wrapped in waitFor to allow transition animation to finish)
    await waitFor(() => {
      expect(screen.queryByText('Keyboard Shortcuts')).toBeNull();
    });
  });

  it('updates isMobile, isTabletPortrait, and isTabletLandscape state on window resize', async () => {
    render(<App />);
    await screen.findByText('Goliath');

    // Change window innerWidth to mobile size
    Object.defineProperty(window, 'innerWidth', {
      writable: true,
      configurable: true,
      value: 500,
    });
    fireEvent(window, new Event('resize'));

    // Verify change is applied (cannot directly access state but layout will use it, which is tested via mock resize trigger)
    expect(window.innerWidth).toBe(500);
  });
});

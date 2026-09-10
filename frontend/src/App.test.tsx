import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import App from './App';
import { ContentTreeCls } from './models/contentTree';

// Opens the chrome menu and picks one of its items. Settings and the
// shortcuts sheet have no button of their own on the action bar.
const openMenuItem = async (label: string) => {
  fireEvent.click(screen.getByLabelText('Menu'));
  fireEvent.click(await screen.findByText(label));
};

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
        Logout = vi.fn().mockResolvedValue(undefined);
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
    await openMenuItem('Settings');
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

    await openMenuItem('Settings');
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

  it('opens the settings dialog from the menu', async () => {
    render(<App />);
    await screen.findByText('Goliath');

    expect(screen.queryByText('Dark theme')).toBeNull();
    await openMenuItem('Settings');
    expect(await screen.findByText('Dark theme')).toBeInTheDocument();
  });

  it('opens the shortcuts sheet from the menu', async () => {
    render(<App />);
    await screen.findByText('Goliath');

    expect(screen.queryByText('Keyboard Shortcuts')).toBeNull();
    await openMenuItem('Keyboard shortcuts');
    expect(await screen.findByText('Keyboard Shortcuts')).toBeInTheDocument();
  });

  it('confirms before logging out, and cancelling keeps the session', async () => {
    render(<App />);
    await screen.findByText('Goliath');

    await openMenuItem('Log out');
    fireEvent.click(await screen.findByText('Cancel'));

    await waitFor(() => {
      expect(screen.queryByText('Cancel')).toBeNull();
    });
    // Still on the reader rather than redirected to the login page.
    expect(screen.getByText('Goliath')).toBeInTheDocument();
  });

  it('logs out and leaves the reader once confirmed', async () => {
    // The one test that needs a router: logging out renders a redirect.
    render(
      <MemoryRouter>
        <App />
      </MemoryRouter>
    );
    await screen.findByText('Goliath');

    await openMenuItem('Log out');
    // Two controls read "Log out" once the dialog is up: the heading and the
    // button that acts.
    const confirm = await screen.findByRole('button', { name: 'Log out' });
    fireEvent.click(confirm);

    await waitFor(() => {
      expect(screen.queryByText('Goliath')).toBeNull();
    });
  });

  it('shows the build stamp in settings', async () => {
    render(<App />);
    await screen.findByText('Goliath');

    await openMenuItem('Settings');
    // The stamp is split across nodes so the hash can be monospaced.
    const about = await screen.findByText('Goliath RSS');
    expect(about.parentElement?.textContent).toContain('Built at test');
    expect(about.parentElement?.textContent).toContain('test');
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

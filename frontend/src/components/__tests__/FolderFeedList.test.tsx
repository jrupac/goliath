import React from 'react';
import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import FolderFeedList from '../FolderFeedList';
import { FolderView } from '../../models/folder';
import { FeedView } from '../../models/feed';
import { SelectionType } from '../../utils/types';
import { expectPartialText } from './helpers';

describe('FolderFeedList', () => {
  it('renders', () => {
    const mockFolderView1: FolderView = {
      id: '1',
      title: 'Test Folder 1',
      unread_count: 0,
    };
    const mockFolderView2: FolderView = {
      id: '2',
      title: 'Test Folder 2',
      unread_count: 0,
    };

    const mockFeed1: FeedView = {
      id: '3',
      title: 'Test Feed 1',
      favicon: null,
      folder_id: '1',
      unread_count: 0,
    };
    const mockFeed2: FeedView = {
      id: '4',
      title: 'Test Feed 2',
      favicon: null,
      folder_id: '2',
      unread_count: 0,
    };

    const mockFolderFeedView = new Map<FolderView, FeedView[]>([
      [mockFolderView1, [mockFeed1]],
      [mockFolderView2, [mockFeed2]],
    ]);

    // Mock functions
    const mockHandleSelect = vi.fn();

    // Minimal rendering test
    const props = {
      folderFeedView: mockFolderFeedView,
      unreadCount: 0,
      selectedKey: '1',
      selectionType: SelectionType.Folder,
      handleSelect: mockHandleSelect,
    };
    const { container } = render(<FolderFeedList {...props} />);

    // Basic assertion to check if it renders without crashing
    expect(container).toBeDefined();
    expect(screen.getByText('Test Folder 1')).toBeInTheDocument();
    expect(screen.getByText('Test Folder 2')).toBeInTheDocument();
  });

  describe('hideEmpty functionality', () => {
    const mockEmptyFolder: FolderView = {
      id: 'empty-folder',
      title: 'Empty Folder',
      unread_count: 0,
    };
    const mockNonEmptyFolder: FolderView = {
      id: 'non-empty-folder',
      title: 'Full Folder',
      unread_count: 5,
    };
    const mockEmptyFeed: FeedView = {
      id: 'empty-feed',
      title: 'Empty Feed',
      favicon: null,
      folder_id: 'empty-folder',
      unread_count: 0,
    };
    const mockNonEmptyFeed: FeedView = {
      id: 'non-empty-feed',
      title: 'Full Feed',
      favicon: null,
      folder_id: 'non-empty-folder',
      unread_count: 5,
    };

    const mockFolderFeedView = new Map<FolderView, FeedView[]>([
      [mockEmptyFolder, [mockEmptyFeed]],
      [mockNonEmptyFolder, [mockNonEmptyFeed]],
    ]);

    const baseProps = {
      folderFeedView: mockFolderFeedView,
      unreadCount: 0,
      handleSelect: vi.fn(),
    };

    it('does not render empty items when hideEmpty is true and not selected', () => {
      render(
        <FolderFeedList
          {...baseProps}
          hideEmpty={true}
          selectedKey="some-other-key"
          selectionType={SelectionType.All}
        />
      );

      expect(screen.queryByText('Empty Folder')).not.toBeInTheDocument();
      expect(screen.queryByText('Empty Feed')).not.toBeInTheDocument();
      expectPartialText(screen, 'Full Folder');
      expectPartialText(screen, 'Full Feed');
    });

    it('renders empty folder when hideEmpty is true but folder is selected', () => {
      render(
        <FolderFeedList
          {...baseProps}
          hideEmpty={true}
          selectedKey={mockEmptyFolder.id}
          selectionType={SelectionType.Folder}
        />
      );

      // Folder is selected and should be visible
      expectPartialText(screen, 'Empty Folder');
      // Feed inside should still be hidden
      expect(screen.queryByText('Empty Feed')).not.toBeInTheDocument();

      expectPartialText(screen, 'Full Folder');
      expectPartialText(screen, 'Full Feed');
    });

    it('renders empty feed when hideEmpty is true but feed is selected', () => {
      render(
        <FolderFeedList
          {...baseProps}
          hideEmpty={true}
          selectedKey={[mockEmptyFeed.id, mockEmptyFolder.id]}
          selectionType={SelectionType.Feed}
        />
      );

      // Folder and corresponding should both be visible
      expectPartialText(screen, 'Empty Folder');
      expectPartialText(screen, 'Empty Feed');

      // Other folders should also be visible
      expectPartialText(screen, 'Full Folder');
      expectPartialText(screen, 'Full Feed');
    });
  });
});

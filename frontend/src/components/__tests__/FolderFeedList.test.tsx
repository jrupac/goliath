import React from 'react';
import {render, screen} from '@testing-library/react';
import {describe, expect, it, vi} from 'vitest';
import FolderFeedList from '../FolderFeedList';
import {FolderView} from "../../models/folder";
import {FeedView} from "../../models/feed";
import {SelectionType} from "../../utils/types";

describe('FolderFeedList', () => {
  it('renders', () => {
    const mockFolderView1: FolderView = {
      id: "1",
      title: "Test Folder 1",
      unread_count: 0
    };
    const mockFolderView2: FolderView = {
      id: "2",
      title: "Test Folder 2",
      unread_count: 0
    }

    const mockFeed1: FeedView = {
      id: "1",
      title: "Test Feed 1",
      favicon: null,
      folder_id: "1",
      unread_count: 0
    };
    const mockFeed2: FeedView = {
      id: "2",
      title: "Test Feed 2",
      favicon: null,
      folder_id: "2",
      unread_count: 0
    }

    const mockFolderFeedView = new Map<FolderView, FeedView[]>([
      [mockFolderView1, [mockFeed1]],
      [mockFolderView2, [mockFeed2]]
    ]);

    // Mock functions
    const mockHandleSelect = vi.fn();

    // Minimal rendering test
    const props = {
      folderFeedView: mockFolderFeedView,
      unreadCount: 0,
      selectedKey: "1",
      selectionType: SelectionType.Folder,
      handleSelect: mockHandleSelect,
    };
    const {container} = render(<FolderFeedList {...props} />);

    // Basic assertion to check if it renders without crashing
    expect(container).toBeDefined();
    expect(screen.getByText('Test Folder 1')).toBeInTheDocument();
    expect(screen.getByText('Test Folder 2')).toBeInTheDocument();
  });
});
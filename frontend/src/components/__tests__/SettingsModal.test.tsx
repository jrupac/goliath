import React from 'react';
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import SettingsModal from '../SettingsModal';
import { FolderView } from '../../models/folder';
import { FeedView } from '../../models/feed';
import { GoliathTheme } from '../../utils/types';

describe('SettingsModal folders', () => {
  const unfiled: FolderView = {
    id: '1',
    title: 'Uncategorized',
    unread_count: 0,
  };
  const news: FolderView = { id: '10', title: 'News', unread_count: 0 };
  const feed = (id: string, title: string, folder_id: string): FeedView => ({
    id,
    title,
    favicon: null,
    folder_id,
    unread_count: 0,
  });
  const folderFeedView = new Map<FolderView, FeedView[]>([
    [unfiled, [feed('1', 'Loose', '1')]],
    [news, [feed('2', 'Daily', '10'), feed('3', 'Weekly', '10')]],
  ]);

  const makeProps = (overrides: Record<string, any> = {}) => ({
    open: true,
    onClose: vi.fn(),
    isMobile: false,
    reloading: false,
    theme: GoliathTheme.Dark,
    onToggleTheme: vi.fn(),
    hideEmpty: false,
    onToggleHideEmpty: vi.fn(),
    onShowKeybindings: vi.fn(),
    buildTimestamp: 'now',
    buildHash: 'abc123',
    folderFeedView,
    onAddFeed: vi.fn(),
    renameFeed: vi.fn().mockResolvedValue(undefined),
    moveFeed: vi.fn().mockResolvedValue(undefined),
    unsubscribeFeed: vi.fn().mockResolvedValue(undefined),
    listFolders: vi.fn().mockResolvedValue([
      { id: '1', title: 'Uncategorized' },
      { id: '10', title: 'News' },
      { id: '11', title: 'Empty' },
    ]),
    moveFeedToNewFolder: vi.fn().mockResolvedValue(undefined),
    renameFolder: vi.fn().mockResolvedValue(undefined),
    deleteFolder: vi.fn().mockResolvedValue(undefined),
    onFeedsChanged: vi.fn(),
    ...overrides,
  });

  const openFeeds = async (props: ReturnType<typeof makeProps>) => {
    render(<SettingsModal {...props} />);
    fireEvent.click(screen.getByRole('button', { name: 'Feeds' }));
    await waitFor(() => expect(props.listFolders).toHaveBeenCalled());
  };

  // The view is built from subscriptions, so a folder with no feeds is known
  // only from the server's list.
  it('shows a folder that holds no feeds', async () => {
    await openFeeds(makeProps());

    expect(await screen.findByText('Empty')).toBeInTheDocument();
    expect(screen.getByText('No feeds in this folder.')).toBeInTheDocument();
  });

  it('offers no rename or removal for the unfiled folder', async () => {
    await openFeeds(makeProps());

    expect(
      screen.getByRole('button', { name: 'Rename folder News' })
    ).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: 'Rename folder Uncategorized' })
    ).toBeNull();
    expect(
      screen.queryByRole('button', { name: 'Remove folder Uncategorized' })
    ).toBeNull();
  });

  it('renames a folder', async () => {
    const props = makeProps();
    await openFeeds(props);

    fireEvent.click(screen.getByRole('button', { name: 'Rename folder News' }));
    fireEvent.change(screen.getByLabelText('Folder name'), {
      target: { value: 'Headlines' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(props.onFeedsChanged).toHaveBeenCalled());
    expect(props.renameFolder).toHaveBeenCalledWith('10', 'Headlines');
  });

  it('refuses a folder name the server would read as an ID', async () => {
    const props = makeProps();
    await openFeeds(props);

    fireEvent.click(screen.getByRole('button', { name: 'Rename folder News' }));
    fireEvent.change(screen.getByLabelText('Folder name'), {
      target: { value: '42' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    expect(
      await screen.findByText(/cannot be just a number/)
    ).toBeInTheDocument();
    expect(props.renameFolder).not.toHaveBeenCalled();
  });

  // Removing a folder does not unsubscribe from anything in it, and the
  // question says where its feeds go before anything is done.
  it('says where the feeds go before removing a folder', async () => {
    const props = makeProps();
    await openFeeds(props);

    fireEvent.click(screen.getByRole('button', { name: 'Remove folder News' }));
    expect(
      screen.getByText('Remove this folder? Its 2 feeds move to Uncategorized.')
    ).toBeInTheDocument();
    expect(props.deleteFolder).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'Remove' }));

    await waitFor(() => expect(props.onFeedsChanged).toHaveBeenCalled());
    expect(props.deleteFolder).toHaveBeenCalledWith('10');
  });

  it('reports a failed removal and keeps the question open', async () => {
    const props = makeProps({
      deleteFolder: vi
        .fn()
        .mockRejectedValue(new Error('Removing the folder failed: Nope')),
    });
    await openFeeds(props);

    fireEvent.click(screen.getByRole('button', { name: 'Remove folder News' }));
    fireEvent.click(screen.getByRole('button', { name: 'Remove' }));

    expect(
      await screen.findByText('Removing the folder failed: Nope')
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Remove' })).toBeInTheDocument();
    expect(props.onFeedsChanged).not.toHaveBeenCalled();
  });

  it('moves a feed into a folder it names', async () => {
    const props = makeProps();
    await openFeeds(props);

    fireEvent.click(screen.getByRole('button', { name: 'Move Daily' }));
    fireEvent.mouseDown(screen.getByRole('combobox'));
    fireEvent.click(await screen.findByRole('option', { name: 'New folder…' }));
    fireEvent.change(screen.getByLabelText('New folder name'), {
      target: { value: 'Reading' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Move' }));

    await waitFor(() => expect(props.onFeedsChanged).toHaveBeenCalled());
    expect(props.moveFeedToNewFolder).toHaveBeenCalledWith(
      '2',
      'Reading',
      '10'
    );
    expect(props.moveFeed).not.toHaveBeenCalled();
  });

  // An empty folder is a place a feed can go even though no feed is there.
  it('offers an empty folder as a move target', async () => {
    const props = makeProps();
    await openFeeds(props);
    await screen.findByText('Empty');

    fireEvent.click(screen.getByRole('button', { name: 'Move Daily' }));
    fireEvent.mouseDown(screen.getByRole('combobox'));
    fireEvent.click(await screen.findByRole('option', { name: 'Empty' }));
    fireEvent.click(screen.getByRole('button', { name: 'Move' }));

    await waitFor(() => expect(props.onFeedsChanged).toHaveBeenCalled());
    expect(props.moveFeed).toHaveBeenCalledWith('2', '11', '10');
  });
});

import React from 'react';
import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import QuickAddDialog from '../QuickAddDialog';
import { FolderView } from '../../models/folder';
import { FeedView } from '../../models/feed';

describe('QuickAddDialog', () => {
  const folder: FolderView = { id: '10', title: 'News', unread_count: 0 };
  const existingFeed: FeedView = {
    id: '1',
    title: 'Existing',
    favicon: null,
    folder_id: '10',
    unread_count: 0,
  };
  const folderFeedView = new Map<FolderView, FeedView[]>([
    [folder, [existingFeed]],
  ]);

  const makeProps = (overrides: Record<string, any> = {}) => ({
    open: true,
    onClose: vi.fn(),
    folderFeedView,
    addFeed: vi.fn().mockResolvedValue({ id: '2', title: 'Fresh' }),
    moveFeed: vi.fn().mockResolvedValue(undefined),
    unsubscribeFeed: vi.fn().mockResolvedValue(undefined),
    onChanged: vi.fn(),
    ...overrides,
  });

  const submitUrl = async (url: string) => {
    fireEvent.change(screen.getByLabelText(/Feed address/), {
      target: { value: url },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Add' }));
  };

  it('subscribes, then moves to the chosen folder', async () => {
    const props = makeProps();
    render(<QuickAddDialog {...props} />);

    await submitUrl('https://example.com/feed');
    await screen.findByText('Fresh');
    expect(props.addFeed).toHaveBeenCalledWith('https://example.com/feed');

    fireEvent.mouseDown(screen.getByRole('combobox'));
    fireEvent.click(await screen.findByRole('option', { name: 'News' }));
    fireEvent.click(screen.getByRole('button', { name: 'Done' }));

    await waitFor(() => expect(props.onClose).toHaveBeenCalled());
    expect(props.moveFeed).toHaveBeenCalledWith('2', '10');
    expect(props.onChanged).toHaveBeenCalled();
    expect(props.unsubscribeFeed).not.toHaveBeenCalled();
  });

  it('leaves a feed unfiled when no folder is chosen', async () => {
    const props = makeProps();
    render(<QuickAddDialog {...props} />);

    await submitUrl('https://example.com/feed');
    await screen.findByText('Fresh');
    fireEvent.click(screen.getByRole('button', { name: 'Done' }));

    await waitFor(() => expect(props.onClose).toHaveBeenCalled());
    expect(props.moveFeed).not.toHaveBeenCalled();
    expect(props.onChanged).toHaveBeenCalled();
  });

  // The address step creates the subscription, so backing out of the folder
  // step has to remove it again or the user is left with a feed they did not
  // finish adding.
  it('cancelling after the add undoes it', async () => {
    const props = makeProps();
    render(<QuickAddDialog {...props} />);

    await submitUrl('https://example.com/feed');
    await screen.findByText('Fresh');
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));

    await waitFor(() => expect(props.onClose).toHaveBeenCalled());
    expect(props.unsubscribeFeed).toHaveBeenCalledWith('2');
    expect(props.onChanged).not.toHaveBeenCalled();
  });

  // Adding an address already subscribed reports the existing feed under the
  // same shape as a new one. Cancelling there must not destroy it.
  it('cancelling after a duplicate add leaves the existing feed alone', async () => {
    const props = makeProps({
      addFeed: vi.fn().mockResolvedValue({ id: '1', title: 'Existing' }),
    });
    render(<QuickAddDialog {...props} />);

    await submitUrl('https://example.com/feed');
    await screen.findByText(/Already subscribed/);
    fireEvent.click(screen.getByRole('button', { name: 'Close' }));

    await waitFor(() => expect(props.onClose).toHaveBeenCalled());
    expect(props.unsubscribeFeed).not.toHaveBeenCalled();
  });

  it("shows the server's reason when the address is refused", async () => {
    const props = makeProps({
      addFeed: vi
        .fn()
        .mockRejectedValue(new Error('That address is not a feed.')),
    });
    render(<QuickAddDialog {...props} />);

    await submitUrl('https://example.com/');

    expect(
      await screen.findByText('That address is not a feed.')
    ).toBeInTheDocument();
    expect(props.onClose).not.toHaveBeenCalled();
  });
});

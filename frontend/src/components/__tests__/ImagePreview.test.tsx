import React, { act } from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import {
  afterAll,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from 'vitest';
import ImagePreview, { previewCache } from '../ImagePreview';

// Lets the on-screen settle delay elapse. Real timers rather than fake ones:
// the component's own effects and promises have to interleave with it, and
// the delay is short enough not to be worth faking.
const settle = () => new Promise((resolve) => setTimeout(resolve, 250));
import * as helpers from '../../utils/helpers';
import { ArticleView } from '../../models/article';

vi.mock('../../utils/helpers', async (importOriginal) => {
  const actual = await importOriginal<typeof helpers>();
  return {
    ...actual,
    getPreviewImage: vi.fn(),
  };
});

describe('ImagePreview', () => {
  const mockArticle: ArticleView = {
    folderId: '1',
    feedId: '1',
    feedTitle: 'Test Feed',
    id: '1',
    title: 'Test Article',
    author: '',
    html: '<img src="http://example.com/image.png" />',
    url: 'http://example.com/article',
    creationTime: 0,
    isRead: false,
    isSaved: false,
  };

  // Mock the IntersectionObserver
  const mockIntersectionObserver = vi.fn(function (
    callback: IntersectionObserverCallback
  ) {
    return {
      observe: vi.fn(),
      unobserve: vi.fn(),
      disconnect: vi.fn(),
      // Expose the callback to trigger it manually in tests
      trigger: (entries: IntersectionObserverEntry[]) => {
        callback(entries, mockIntersectionObserver as any);
      },
    };
  });

  beforeAll(() => {
    vi.stubGlobal('IntersectionObserver', mockIntersectionObserver);
  });

  beforeEach(() => {
    // Clear mocks and the component's internal cache before each test
    vi.clearAllMocks();
    previewCache.clear();
    mockIntersectionObserver.mockClear();
  });

  afterAll(() => {
    vi.unstubAllGlobals();
  });

  it('should render no skeleton initially and not fetch image', () => {
    render(<ImagePreview article={mockArticle} />);

    // Nothing should render before the element is on-screen
    expect(
      screen.queryByTestId('image-preview-skeleton')
    ).not.toBeInTheDocument();

    // Verify getPreviewImage has not been called yet because it's not on screen
    expect(helpers.getPreviewImage).not.toHaveBeenCalled();
  });

  it('should fetch image when it becomes visible', async () => {
    vi.mocked(helpers.getPreviewImage).mockResolvedValueOnce({
      src: 'http://example.com/image.png',
      x: 0,
      y: 0,
      width: 0,
      height: 0,
      origWidth: 0,
    });

    render(<ImagePreview article={mockArticle} />);

    // Simulate the component becoming visible
    await act(async () => {
      mockIntersectionObserver.mock.results[0].value.trigger([
        { isIntersecting: true },
      ] as IntersectionObserverEntry[]);
      // The fetch waits for the row to stay put, so nothing happens until it
      // has.
      await settle();
    });

    // Wait for the image to appear and verify the src
    const img = await screen.findByRole('img');
    expect(img).toHaveAttribute('src', 'http://example.com/image.png');

    // Verify getPreviewImage was called once
    expect(helpers.getPreviewImage).toHaveBeenCalledTimes(1);
    expect(helpers.getPreviewImage).toHaveBeenCalledWith(mockArticle);
  });

  it('should render nothing if no image is found', async () => {
    // Mock getPreviewImage to return no preview
    vi.mocked(helpers.getPreviewImage).mockResolvedValueOnce(undefined);

    const { container } = render(<ImagePreview article={mockArticle} />);

    // Nothing should render before the element is on-screen
    expect(
      screen.queryByTestId('image-preview-skeleton')
    ).not.toBeInTheDocument();

    // Simulate the component becoming visible — skeleton should appear while loading
    await act(async () => {
      mockIntersectionObserver.mock.results[0].value.trigger([
        { isIntersecting: true },
      ] as IntersectionObserverEntry[]);
      // The fetch waits for the row to stay put, so nothing happens until it
      // has.
      await settle();
    });

    // Wait for the component to fully unmount after the load resolves with no image
    await waitFor(() => {
      expect(container.firstChild).toBeNull();
    });
  });
});

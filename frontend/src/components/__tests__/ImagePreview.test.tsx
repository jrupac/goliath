import React from 'react';
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
import * as helpers from '../../utils/helpers';
import { ArticleView } from '../../models/article';

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
  const mockIntersectionObserver = vi.fn((callback) => ({
    observe: vi.fn(),
    unobserve: vi.fn(),
    disconnect: vi.fn(),
    // Expose the callback to trigger it manually in tests
    trigger: (entries: IntersectionObserverEntry[]) => {
      callback(entries, mockIntersectionObserver as any);
    },
  }));

  beforeAll(() => {
    vi.stubGlobal('IntersectionObserver', mockIntersectionObserver);
    // Mock getPreviewImage from helpers
    vi.mock('../../utils/helpers', async (importOriginal) => {
      const actual = await importOriginal<typeof helpers>();
      return {
        ...actual,
        getPreviewImage: vi.fn(),
      };
    });
  });

  beforeEach(() => {
    // Clear mocks and the component's internal cache before each test
    vi.clearAllMocks();
    previewCache.clear();
    mockIntersectionObserver.mockClear();
  });

  afterAll(() => {
    vi.unstubAllGlobals();
    vi.doUnmock('../../utils/helpers');
  });

  it('should render skeleton initially and not fetch image', () => {
    render(<ImagePreview article={mockArticle} />);

    // Check for the skeleton placeholder
    expect(screen.getByTestId('image-preview-skeleton')).toBeInTheDocument();

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
    mockIntersectionObserver.mock.results[0].value.trigger([
      { isIntersecting: true },
    ] as IntersectionObserverEntry[]);

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

    // Skeleton is there initially
    expect(screen.getByTestId('image-preview-skeleton')).toBeInTheDocument();

    // Simulate the component becoming visible
    mockIntersectionObserver.mock.results[0].value.trigger([
      { isIntersecting: true },
    ] as IntersectionObserverEntry[]);

    // Wait for the skeleton to disappear
    await waitFor(() => {
      expect(
        screen.queryByTestId('image-preview-skeleton')
      ).not.toBeInTheDocument();
    });

    // The component should now render nothing
    expect(container.firstChild).toBeNull();
  });
});

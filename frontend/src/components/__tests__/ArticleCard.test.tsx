import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import ArticleCard from '../ArticleCard';
import { ArticleView } from '../../models/article';
import { FaviconCls } from '../../models/feed';
import { formatFriendly } from '../../utils/helpers';
import { SelectionType } from '../../utils/types';

describe('ArticleCard', () => {
  const mockFetchApi = {
    ParseFullArticle: vi
      .fn()
      .mockResolvedValue('<p>Parsed readability content</p>'),
  };
  const mockHandleUpdateArticleParsed = vi.fn();

  const makeProps = (overrides: Record<string, any> = {}) => {
    const articleOverrides = overrides.article || {};
    return {
      fetchApi: mockFetchApi as any,
      handleUpdateArticleParsed: mockHandleUpdateArticleParsed,
      article: {
        folderId: '1',
        feedId: '1',
        feedTitle: 'Test Feed',
        id: '1',
        title: 'Test Article',
        author: '',
        html: '<p>Test content</p>',
        url: 'https://example.com',
        creationTime: 1678886400, // March 15, 2023
        isRead: false,
        isSaved: false,
        parsed: null,
        ...articleOverrides,
      } as ArticleView,
      title: 'Test Feed',
      favicon: new FaviconCls(''),
      feedId: '1',
      isSelected: false,
      onMarkArticleRead: () => {},
      onToggleSave: () => {},
      selectionType: SelectionType.Unread,
      ...overrides,
    };
  };

  it('renders', () => {
    render(<ArticleCard {...makeProps()} />);
  });

  it('renders with all props correctly', () => {
    const articleUrl = 'https://example.com/test-article';
    const articleCreationTime = 1678886400; // March 15, 2023
    const friendlyFormattedDate = formatFriendly(
      new Date(articleCreationTime * 1000)
    );

    const props = makeProps({
      article: {
        folderId: '1',
        feedId: '1',
        feedTitle: 'Test Feed',
        id: '1',
        title: 'Test Article',
        author: 'Test Author',
        html: '<p>Test content</p>',
        url: articleUrl,
        creationTime: articleCreationTime,
        isRead: false,
        isSaved: false,
        parsed: null,
      },
      title: 'Test Feed Title',
    });

    render(<ArticleCard {...props} />);

    // Assert feed title
    expect(screen.getByText('Test Feed Title')).toBeInTheDocument();

    // Assert article title
    expect(screen.getByText('Test Article')).toBeInTheDocument();

    // Assert article content
    expect(screen.getByText('Test content')).toBeInTheDocument();

    // Assert article URL link
    expect(screen.getByRole('link', { name: 'Test Article' })).toHaveAttribute(
      'href',
      articleUrl
    );

    // Assert friendly formatted date
    expect(screen.getByText(friendlyFormattedDate)).toBeInTheDocument();
  });

  it('renders the correct favicon image when provided', () => {
    const testFaviconData =
      'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7';

    render(
      <ArticleCard
        {...makeProps({ favicon: new FaviconCls(testFaviconData) })}
      />
    );

    expect(screen.getByRole('img', { name: '' })).toHaveAttribute(
      'src',
      testFaviconData
    );
  });

  it('calls onMarkArticleRead when mark as read button is clicked', () => {
    const mockOnMarkArticleRead = vi.fn();
    render(
      <ArticleCard
        {...makeProps({
          isSelected: true,
          onMarkArticleRead: mockOnMarkArticleRead,
        })}
      />
    );

    const markAsReadButton = screen.getByLabelText('mark as read');
    fireEvent.click(markAsReadButton);

    expect(mockOnMarkArticleRead).toHaveBeenCalledTimes(1);
  });

  it('shows CheckCircle icon for unread article', () => {
    render(<ArticleCard {...makeProps({ isSelected: true })} />);
    expect(screen.getByTestId('CheckCircleTwoToneIcon')).toBeInTheDocument();
  });

  it('shows Check icon for read article', () => {
    render(
      <ArticleCard
        {...makeProps({ isSelected: true, article: { isRead: true } })}
      />
    );
    expect(screen.getByTestId('CheckCircleOutlineIcon')).toBeInTheDocument();
  });

  it('toggles reader mode on "m" shortcut', async () => {
    const props = makeProps({ isSelected: true });
    render(<ArticleCard {...props} />);

    const readerButton = screen.getByLabelText('reader mode');
    expect(readerButton).not.toHaveClass('GoliathReaderModeActive');

    fireEvent.keyDown(window, { key: 'm' });

    await waitFor(() => {
      expect(readerButton).toHaveClass('GoliathReaderModeActive');
    });

    expect(mockFetchApi.ParseFullArticle).toHaveBeenCalledWith('1');
    expect(mockHandleUpdateArticleParsed).toHaveBeenCalledWith(
      '1',
      '1',
      '1',
      '<p>Parsed readability content</p>'
    );
  });

  it('renders the back button and calls onBack when clicked', () => {
    const mockOnBack = vi.fn();
    render(<ArticleCard {...makeProps({ onBack: mockOnBack })} />);

    const backButton = screen.getByLabelText('back to list');
    expect(backButton).toBeInTheDocument();
    fireEvent.click(backButton);
    expect(mockOnBack).toHaveBeenCalledTimes(1);
  });

  it('renders previous and next buttons and calls callbacks when clicked', () => {
    const mockOnPrev = vi.fn();
    const mockOnNext = vi.fn();
    render(
      <ArticleCard
        {...makeProps({
          onPrev: mockOnPrev,
          onNext: mockOnNext,
        })}
      />
    );

    const prevButton = screen.getByLabelText('previous article');
    const nextButton = screen.getByLabelText('next article');

    expect(prevButton).toBeInTheDocument();
    expect(nextButton).toBeInTheDocument();

    fireEvent.click(prevButton);
    expect(mockOnPrev).toHaveBeenCalledTimes(1);

    fireEvent.click(nextButton);
    expect(mockOnNext).toHaveBeenCalledTimes(1);
  });

  it('renders the title inside the title bar and the byline bar at the root level', () => {
    const props = makeProps();
    const { container } = render(<ArticleCard {...props} />);

    // Check for byline bar and title bar outside the container
    // (direct children of GoliathArticleCardColumn stack)
    const cardColumn = container.firstChild;
    expect(cardColumn).toHaveClass('GoliathArticleCardColumn');

    const byline = cardColumn?.childNodes[1];
    expect(byline).toHaveClass('GoliathArticleByline');

    const titleBar = cardColumn?.childNodes[2];
    expect(titleBar).toHaveClass('GoliathArticleTitleBar');

    const containerEl = cardColumn?.childNodes[3];
    expect(containerEl).toHaveClass('GoliathSplitViewArticleContainer');

    const titleBarOccluder = titleBar?.childNodes[0];
    expect(titleBarOccluder).toHaveClass('GoliathArticleTitleBarOccluder');

    const title = titleBar?.childNodes[1];
    expect(title).toHaveClass('GoliathArticleTitle');
  });
});

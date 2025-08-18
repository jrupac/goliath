import React from 'react';
import { fireEvent, render, screen, within } from '@testing-library/react';
import ArticleList from '../ArticleList';
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { ArticleView } from '../../models/article';
import { expectClass, expectTextInElement } from './helpers';
import { MarkState, SelectionKey, SelectionType } from '../../utils/types';
import { FeedId, FaviconCls } from '../../models/feed';

describe('ArticleList', () => {
  // Mock ArticleView data
  const mockArticles: ArticleView[] = [
    {
      folderId: '1',
      feedId: '1',
      feedTitle: 'Test Feed 1',
      id: '1',
      title: 'Test Article 1',
      author: '',
      html: '<p>Test content 1</p>',
      url: 'https://example.com/1',
      creationTime: 1678972810, // March 16, 2023 12:00:10
      isRead: false,
      isSaved: false,
    },
    {
      folderId: '1',
      feedId: '1',
      feedTitle: 'Test Feed 1',
      id: '2',
      title: 'Test Article 2',
      author: '',
      html: '<p>Test content 2</p>',
      url: 'https://example.com/2',
      creationTime: 1678972809, // March 16, 2023 12:00:09
      isRead: false,
      isSaved: false,
    },
  ];

  // Mock functions
  const mockSelectAllCallback = vi.fn();
  const mockHandleMark = vi.fn();

  let originalOffsetHeight: PropertyDescriptor | undefined;
  let originalOffsetWidth: PropertyDescriptor | undefined;

  beforeEach(() => {
    vi.clearAllMocks();

    // Mock offsetHeight and offsetWidth for all HTMLElements
    originalOffsetHeight = Object.getOwnPropertyDescriptor(
      HTMLElement.prototype,
      'offsetHeight'
    );
    originalOffsetWidth = Object.getOwnPropertyDescriptor(
      HTMLElement.prototype,
      'offsetWidth'
    );

    Object.defineProperty(HTMLElement.prototype, 'offsetHeight', {
      configurable: true,
      value: 1000, // A large enough height to render all articles
    });
    Object.defineProperty(HTMLElement.prototype, 'offsetWidth', {
      configurable: true,
      value: 500, // A reasonable width
    });
  });

  afterEach(() => {
    // Restore original properties
    if (originalOffsetHeight) {
      Object.defineProperty(
        HTMLElement.prototype,
        'offsetHeight',
        originalOffsetHeight
      );
    } else {
      delete (HTMLElement.prototype as any).offsetHeight;
    }
    if (originalOffsetWidth) {
      Object.defineProperty(
        HTMLElement.prototype,
        'offsetWidth',
        originalOffsetWidth
      );
    } else {
      delete (HTMLElement.prototype as any).offsetWidth;
    }
  });

  it('renders', () => {
    // Minimal rendering test
    const props = {
      articleEntriesCls: mockArticles,
      faviconMap: new Map<FeedId, FaviconCls>(),
      selectionKey: '1' as SelectionKey,
      selectionType: SelectionType.Folder,
      selectAllCallback: mockSelectAllCallback,
      handleMark: mockHandleMark,
      buildTimestamp: '',
      buildHash: '',
    };
    const { container } = render(<ArticleList {...props} />);

    // Basic assertion to check if it renders without crashing
    expect(container).toBeDefined();
    const articleTitleElement = screen.getAllByText('Test Article 1');
    expect(articleTitleElement).is.not.empty;
  });

  it('marks article as read and selects next on scroll down', () => {
    const props = {
      articleEntriesCls: mockArticles,
      faviconMap: new Map<FeedId, FaviconCls>(),
      selectionKey: '1' as SelectionKey,
      selectionType: SelectionType.Folder,
      selectAllCallback: mockSelectAllCallback,
      handleMark: mockHandleMark,
      buildTimestamp: '',
      buildHash: '',
    };

    const { container } = render(<ArticleList {...props} />);

    // Before scroll, Article 1 is selected
    const articleListBox = container.querySelector(
      '.GoliathSplitViewArticleListBox'
    ) as HTMLElement;
    const article1Entry = expectTextInElement(articleListBox, 'Test Article 1');
    expectClass(
      article1Entry,
      '.GoliathArticleListBase',
      'GoliathArticleListSelected'
    );

    // Simulate scroll down
    fireEvent.keyDown(window, { key: 'j' });

    // Check that handleMark was called for the first article
    expect(mockHandleMark).toHaveBeenCalledWith(
      MarkState.Read,
      [mockArticles[0].id, mockArticles[0].feedId, mockArticles[0].folderId],
      SelectionType.Article
    );

    // After scroll, Article 2 should be selected
    const article2Entry = expectTextInElement(articleListBox, 'Test Article 2');
    expectClass(
      article1Entry,
      '.GoliathArticleListBase',
      'GoliathArticleListSelected',
      false
    );
    expectClass(
      article2Entry,
      '.GoliathArticleListBase',
      'GoliathArticleListSelected'
    );
  });

  it('selects previous article on scroll up', () => {
    const props = {
      articleEntriesCls: mockArticles,
      faviconMap: new Map<FeedId, FaviconCls>(),
      selectionKey: '1' as SelectionKey,
      selectionType: SelectionType.Folder,
      selectAllCallback: mockSelectAllCallback,
      handleMark: mockHandleMark,
      buildTimestamp: '',
      buildHash: '',
    };

    const { container } = render(<ArticleList {...props} />);

    const articleListBox = container.querySelector(
      '.GoliathSplitViewArticleListBox'
    ) as HTMLElement;

    // Get button references
    const scrollDownButton = screen.getByLabelText('scroll down');
    const scrollUpButton = screen.getByLabelText('scroll up');

    // First, scroll down to select the second article (keyboard)
    fireEvent.keyDown(window, { key: 'j' });

    // Verify Article 2 is selected
    const article1EntryAfterScrollDown = expectTextInElement(
      articleListBox,
      'Test Article 1'
    );
    const article2EntryAfterScrollDown = expectTextInElement(
      articleListBox,
      'Test Article 2'
    );
    expectClass(
      article1EntryAfterScrollDown,
      '.GoliathArticleListBase',
      'GoliathArticleListSelected',
      false
    );
    expectClass(
      article2EntryAfterScrollDown,
      '.GoliathArticleListBase',
      'GoliathArticleListSelected'
    );

    // Simulate scroll up (keyboard)
    fireEvent.keyDown(window, { key: 'k' });

    // Check that handleMark was NOT called again.
    // It should have been called once for marking the first article as read
    // when scrolling down.
    expect(mockHandleMark).toHaveBeenCalledTimes(1);

    // After scroll up, Article 1 should be selected again
    const article1EntryAfterScrollUp = expectTextInElement(
      articleListBox,
      'Test Article 1'
    );
    const article2EntryAfterScrollUp = expectTextInElement(
      articleListBox,
      'Test Article 2'
    );
    expectClass(
      article1EntryAfterScrollUp,
      '.GoliathArticleListBase',
      'GoliathArticleListSelected'
    );
    expectClass(
      article2EntryAfterScrollUp,
      '.GoliathArticleListBase',
      'GoliathArticleListSelected',
      false
    );

    // Simulate scroll down (button click)
    fireEvent.click(scrollDownButton);

    // Verify Article 2 is selected again
    expectClass(
      expectTextInElement(articleListBox, 'Test Article 1'),
      '.GoliathArticleListBase',
      'GoliathArticleListSelected',
      false
    );
    expectClass(
      expectTextInElement(articleListBox, 'Test Article 2'),
      '.GoliathArticleListBase',
      'GoliathArticleListSelected'
    );
    // handleMark should NOT be called again for the first article as it's already read
    expect(mockHandleMark).toHaveBeenCalledTimes(1);

    // Simulate scroll up (button click)
    fireEvent.click(scrollUpButton);

    // Verify Article 1 is selected again
    expectClass(
      expectTextInElement(articleListBox, 'Test Article 1'),
      '.GoliathArticleListBase',
      'GoliathArticleListSelected'
    );
    expectClass(
      expectTextInElement(articleListBox, 'Test Article 2'),
      '.GoliathArticleListBase',
      'GoliathArticleListSelected',
      false
    );
    // handleMark should still be 1, as scrolling up doesn't mark unread
    expect(mockHandleMark).toHaveBeenCalledTimes(1);
  });

  it('merges new articles and scrolls to top', () => {
    const initialArticles: ArticleView[] = [
      {
        id: '1',
        title: 'Initial Article 1',
        html: '<p>Initial Content 1</p>',
        creationTime: 1678972810, // March 16, 2023 12:00:10
        folderId: '1',
        feedId: '1',
        feedTitle: 'Test Feed',
        author: '',
        url: 'http://example.com/1',
        isRead: false,
        isSaved: false,
      },
      {
        id: '2',
        title: 'Initial Article 2',
        html: '<p>Initial Content 2</p>',
        creationTime: 1678972809, // March 16, 2023 12:00:09
        folderId: '1',
        feedId: '1',
        feedTitle: 'Test Feed',
        author: '',
        url: 'http://example.com/2',
        isRead: false,
        isSaved: false,
      },
    ];

    const newArticles: ArticleView[] = [
      {
        id: '3',
        title: 'New Article 3',
        html: '<p>New Content 3</p>',
        creationTime: 1678972811, // March 16, 2023 12:00:11
        folderId: '1',
        feedId: '1',
        feedTitle: 'Test Feed',
        author: '',
        url: 'http://example.com/3',
        isRead: false,
        isSaved: false,
      },
      {
        id: '1',
        title: 'Initial Article 1',
        html: '<p>Initial Content 1</p>',
        creationTime: 1678972810, // March 16, 2023 12:00:10
        folderId: '1',
        feedId: '1',
        feedTitle: 'Test Feed',
        author: '',
        url: 'http://example.com/1',
        isRead: false,
        isSaved: false,
      },
    ];

    const props = {
      articleEntriesCls: initialArticles,
      faviconMap: new Map<FeedId, FaviconCls>(),
      selectionKey: '1' as SelectionKey,
      selectionType: SelectionType.Folder,
      selectAllCallback: mockSelectAllCallback,
      handleMark: mockHandleMark,
      buildTimestamp: '',
      buildHash: '',
    };

    const { container, rerender } = render(<ArticleList {...props} />);

    // Initial render, Article 1 should be selected (latest creationTime)
    const articleListBox = container.querySelector(
      '.GoliathSplitViewArticleListBox'
    ) as HTMLElement;
    const initialArticle1 = expectTextInElement(
      articleListBox,
      'Initial Article 1'
    );
    expectClass(
      initialArticle1,
      '.GoliathArticleListBase',
      'GoliathArticleListSelected'
    );

    // Rerender with new articles
    rerender(<ArticleList {...props} articleEntriesCls={newArticles} />);

    // After rerender, the new article (Article 3) should be at the top and
    // selected
    const newArticle3 = expectTextInElement(articleListBox, 'New Article 3');
    expectClass(
      newArticle3,
      '.GoliathArticleListBase',
      'GoliathArticleListSelected'
    );
  });

  it('maintains scroll position when props changes non-meaningfully', () => {
    const initialArticles: ArticleView[] = [
      {
        folderId: '1',
        feedId: '1',
        feedTitle: 'Test Feed 1',
        id: '1',
        title: 'Test Article 1',
        author: '',
        html: '<p>Test content 1</p>',
        url: 'https://example.com/1',
        creationTime: 1678972810,
        isRead: false,
        isSaved: false,
      },
      {
        folderId: '1',
        feedId: '1',
        feedTitle: 'Test Feed 1',
        id: '2',
        title: 'Test Article 2',
        author: '',
        html: '<p>Test content 2</p>',
        url: 'https://example.com/2',
        creationTime: 1678972809,
        isRead: false,
        isSaved: false,
      },
      {
        folderId: '1',
        feedId: '1',
        feedTitle: 'Test Feed 1',
        id: '3',
        title: 'Test Article 3',
        author: '',
        html: '<p>Test content 3</p>',
        url: 'https://example.com/3',
        creationTime: 1678972808,
        isRead: false,
        isSaved: false,
      },
    ];

    const props = {
      articleEntriesCls: initialArticles,
      faviconMap: new Map<FeedId, FaviconCls>(),
      selectionKey: '1' as SelectionKey,
      selectionType: SelectionType.Folder,
      selectAllCallback: mockSelectAllCallback,
      handleMark: mockHandleMark,
      buildTimestamp: '',
      buildHash: '',
    };

    const { container, rerender } = render(<ArticleList {...props} />);

    const articleListBox = container.querySelector(
      '.GoliathSplitViewArticleListBox'
    ) as HTMLElement;

    // Initially, Article 1 is selected
    expectClass(
      expectTextInElement(articleListBox, 'Test Article 1'),
      '.GoliathArticleListBase',
      'GoliathArticleListSelected'
    );

    // Scroll down to select Article 2
    fireEvent.keyDown(window, { key: 'j' });

    // Verify Article 2 is selected
    expectClass(
      expectTextInElement(articleListBox, 'Test Article 1'),
      '.GoliathArticleListBase',
      'GoliathArticleListSelected',
      false
    );
    expectClass(
      expectTextInElement(articleListBox, 'Test Article 2'),
      '.GoliathArticleListBase',
      'GoliathArticleListSelected'
    );

    // Simulate an update where Article 1 becomes read and is no longer part of
    // the props.
    const updatedArticles: ArticleView[] = [
      initialArticles[1],
      initialArticles[2],
    ];

    rerender(<ArticleList {...props} articleEntriesCls={updatedArticles} />);

    // Assert that Article 2 is still selected (no jump to top)
    expectClass(
      expectTextInElement(articleListBox, 'Test Article 1'),
      '.GoliathArticleListBase',
      'GoliathArticleListSelected',
      false
    );
    expectClass(
      expectTextInElement(articleListBox, 'Test Article 2'),
      '.GoliathArticleListBase',
      'GoliathArticleListSelected'
    );

    // Assert no duplicate articles
    expect(within(articleListBox).getAllByText('Test Article 1')).toHaveLength(
      1
    );
    expect(within(articleListBox).getAllByText('Test Article 2')).toHaveLength(
      1
    );
    expect(within(articleListBox).getAllByText('Test Article 3')).toHaveLength(
      1
    );
  });

  it('resets scroll position to top when new articles are added', () => {
    const initialArticles: ArticleView[] = [
      {
        folderId: '1',
        feedId: '1',
        feedTitle: 'Test Feed 1',
        id: '1',
        title: 'Test Article 1',
        author: '',
        html: '<p>Test content 1</p>',
        url: 'https://example.com/1',
        creationTime: 1678972810,
        isRead: false,
        isSaved: false,
      },
      {
        folderId: '1',
        feedId: '1',
        feedTitle: 'Test Feed 1',
        id: '2',
        title: 'Test Article 2',
        author: '',
        html: '<p>Test content 2</p>',
        url: 'https://example.com/2',
        creationTime: 1678972809,
        isRead: false,
        isSaved: false,
      },
    ];

    const props = {
      articleEntriesCls: initialArticles,
      faviconMap: new Map<FeedId, FaviconCls>(),
      selectionKey: '1' as SelectionKey,
      selectionType: SelectionType.Folder,
      selectAllCallback: mockSelectAllCallback,
      handleMark: mockHandleMark,
      buildTimestamp: '',
      buildHash: '',
    };

    const { container, rerender } = render(<ArticleList {...props} />);

    const articleListBox = container.querySelector(
      '.GoliathSplitViewArticleListBox'
    ) as HTMLElement;

    // Scroll down to select Article 2
    fireEvent.keyDown(window, { key: 'j' });

    // Verify Article 2 is selected
    expectClass(
      expectTextInElement(articleListBox, 'Test Article 1'),
      '.GoliathArticleListBase',
      'GoliathArticleListSelected',
      false
    );
    expectClass(
      expectTextInElement(articleListBox, 'Test Article 2'),
      '.GoliathArticleListBase',
      'GoliathArticleListSelected'
    );

    // Simulate adding a new article at the beginning
    const newArticle: ArticleView = {
      folderId: '1',
      feedId: '1',
      feedTitle: 'Test Feed 1',
      id: '0', // New ID
      title: 'New Test Article 0',
      author: '',
      html: '<p>New content 0</p>',
      url: 'https://example.com/0',
      creationTime: 1678972811, // Newer creation time
      isRead: false,
      isSaved: false,
    };
    const updatedArticles: ArticleView[] = [newArticle, ...initialArticles];

    rerender(<ArticleList {...props} articleEntriesCls={updatedArticles} />);

    // Assert that the new article (Article 0) is selected (implying that the
    // scroll position went to the top)
    expectClass(
      expectTextInElement(articleListBox, 'New Test Article 0'),
      '.GoliathArticleListBase',
      'GoliathArticleListSelected'
    );
    expectClass(
      expectTextInElement(articleListBox, 'Test Article 1'),
      '.GoliathArticleListBase',
      'GoliathArticleListSelected',
      false
    );
    expectClass(
      expectTextInElement(articleListBox, 'Test Article 2'),
      '.GoliathArticleListBase',
      'GoliathArticleListSelected',
      false
    );

    // Assert no duplicate articles
    expect(
      within(articleListBox).getAllByText('New Test Article 0')
    ).toHaveLength(1);
    expect(within(articleListBox).getAllByText('Test Article 1')).toHaveLength(
      1
    );
    expect(within(articleListBox).getAllByText('Test Article 2')).toHaveLength(
      1
    );
  });
});

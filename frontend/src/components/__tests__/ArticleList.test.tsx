import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import ArticleList, { ArticleListProps } from '../ArticleList';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ArticleView } from '../../models/article';
import { expectClass, expectTextInElement } from './helpers';
import {
  MarkState,
  NavigationDirection,
  SelectionKey,
  SelectionType,
} from '../../utils/types';
import { FaviconCls, FeedId } from '../../models/feed';

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
  const mockSelectUnreadCallback = vi.fn();
  const mockHandleMark = vi.fn();
  const mockNavigateToAdjacentEntry = vi.fn();

  const getMockProps = (
    props?: Partial<ArticleListProps>
  ): ArticleListProps => ({
    articleEntriesCls: mockArticles,
    faviconMap: new Map<FeedId, FaviconCls>(),
    selectionKey: '1' as SelectionKey,
    selectionType: SelectionType.Folder,
    selectAllCallback: mockSelectAllCallback,
    selectUnreadCallback: mockSelectUnreadCallback,
    handleMark: mockHandleMark,
    buildTimestamp: '',
    buildHash: '',
    ...props,
  });

  const expectArticleSelected = (
    container: HTMLElement,
    title: string,
    selected: boolean
  ) => {
    const articleListBox = container.querySelector(
      '.GoliathSplitViewArticleListBox'
    ) as HTMLElement;
    const articleEntry = expectTextInElement(articleListBox, title);
    expectClass(
      articleEntry,
      '.GoliathArticleListBase',
      'GoliathArticleListSelected',
      selected
    );
  };

  let originalOffsetHeight: PropertyDescriptor | undefined;
  let originalOffsetWidth: PropertyDescriptor | undefined;

  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigateToAdjacentEntry.mockReset();

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
      value: 182, // Set equal to the height of each entry
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
    const { container } = render(<ArticleList {...getMockProps()} />);

    // Basic assertion to check if it renders without crashing
    expect(container).toBeDefined();
    const articleTitleElement = screen.getAllByText('Test Article 1');
    expect(articleTitleElement).is.not.empty;
  });

  it('marks article as read and selects next on scroll down', () => {
    const { container } = render(<ArticleList {...getMockProps()} />);

    // Before scroll, Article 1 is selected
    expectArticleSelected(container, 'Test Article 1', true);

    // Simulate scroll down
    fireEvent.keyDown(window, { key: 'j' });

    // Check that handleMark was called for the first article
    expect(mockHandleMark).toHaveBeenCalledWith(
      MarkState.Read,
      [mockArticles[0].id, mockArticles[0].feedId, mockArticles[0].folderId],
      SelectionType.Article
    );

    // After scroll, Article 2 should be selected
    expectArticleSelected(container, 'Test Article 1', false);
    expectArticleSelected(container, 'Test Article 2', true);
  });

  it('selects previous article on scroll up', () => {
    const { container, rerender } = render(<ArticleList {...getMockProps()} />);

    // Get button references
    const scrollDownButton = screen.getByLabelText('scroll down');
    const scrollUpButton = screen.getByLabelText('scroll up');

    // First, scroll down to select the second article (keyboard)
    fireEvent.keyDown(window, { key: 'j' });

    // Simulate the parent component updating the props after the article is marked read
    const updatedArticles = mockArticles.map((a, i) =>
      i === 0 ? { ...a, isRead: true } : a
    );
    rerender(
      <ArticleList {...getMockProps({ articleEntriesCls: updatedArticles })} />
    );

    // Verify Article 2 is selected
    expectArticleSelected(container, 'Test Article 1', false);
    expectArticleSelected(container, 'Test Article 2', true);

    // Simulate scroll up (keyboard)
    fireEvent.keyDown(window, { key: 'k' });

    // Check that handleMark was NOT called again.
    // It should have been called once for marking the first article as read
    // when scrolling down.
    expect(mockHandleMark).toHaveBeenCalledTimes(1);

    // After scroll up, Article 1 should be selected again
    expectArticleSelected(container, 'Test Article 1', true);
    expectArticleSelected(container, 'Test Article 2', false);

    // Simulate scroll down (button click)
    fireEvent.click(scrollDownButton);

    // Verify Article 2 is selected again
    expectArticleSelected(container, 'Test Article 1', false);
    expectArticleSelected(container, 'Test Article 2', true);
    // handleMark should NOT be called again for the first article as it's
    // already read
    expect(mockHandleMark).toHaveBeenCalledTimes(1);

    // Simulate scroll up (button click)
    fireEvent.click(scrollUpButton);

    // Verify Article 1 is selected again
    expectArticleSelected(container, 'Test Article 1', true);
    expectArticleSelected(container, 'Test Article 2', false);
    // handleMark should still be 1, as scrolling up doesn't mark unread
    expect(mockHandleMark).toHaveBeenCalledTimes(1);
  });

  it('selects article on click without marking as read', () => {
    const { container } = render(<ArticleList {...getMockProps()} />);

    // Article 1 is selected initially
    expectArticleSelected(container, 'Test Article 1', true);
    expectArticleSelected(container, 'Test Article 2', false);

    // Click on Article 2's entry
    const articleListBox = container.querySelector(
      '.GoliathSplitViewArticleListBox'
    ) as HTMLElement;
    const entries = articleListBox.querySelectorAll('.GoliathArticleListBase');
    fireEvent.click(entries[1]);

    // Article 2 should now be selected
    expectArticleSelected(container, 'Test Article 1', false);
    expectArticleSelected(container, 'Test Article 2', true);

    // No article should have been marked as read
    expect(mockHandleMark).not.toHaveBeenCalled();
  });

  it('resets scroll index when selection key changes', () => {
    const { container, rerender } = render(
      <ArticleList {...getMockProps({ selectionKey: 'key1' })} />
    );

    // Before scroll, Article 1 is selected
    expectArticleSelected(container, 'Test Article 1', true);
    expectArticleSelected(container, 'Test Article 2', false);

    // Simulate scroll down
    fireEvent.keyDown(window, { key: 'j' });

    // After scroll, Article 2 should be selected
    expectArticleSelected(container, 'Test Article 1', false);
    expectArticleSelected(container, 'Test Article 2', true);

    // Rerender with a new selection key
    rerender(<ArticleList {...getMockProps({ selectionKey: 'key2' })} />);

    // After rerender with new key, scroll should reset to top (Article 1)
    expectArticleSelected(container, 'Test Article 1', true);
    expectArticleSelected(container, 'Test Article 2', false);
  });

  it('calls navigateToAdjacentEntry("next") when pressing j at the last article', () => {
    render(
      <ArticleList
        {...getMockProps({
          navigateToAdjacentEntry: mockNavigateToAdjacentEntry,
        })}
      />
    );

    // Scroll to the last article first
    fireEvent.keyDown(window, { key: 'j' });

    // Now at the last article (index 1); pressing j again should invoke the callback
    fireEvent.keyDown(window, { key: 'j' });

    expect(mockNavigateToAdjacentEntry).toHaveBeenCalledWith(
      NavigationDirection.Next
    );
    expect(mockNavigateToAdjacentEntry).toHaveBeenCalledTimes(1);
  });

  it('does not navigate when pressing j at end of list without navigateToAdjacentEntry prop (All stream)', () => {
    // In production, navigateToAdjacentEntry is undefined for All/Unread/Saved streams.
    render(
      <ArticleList
        {...getMockProps({
          selectionType: SelectionType.All,
        })}
      />
    );

    // Scroll to the last article first
    fireEvent.keyDown(window, { key: 'j' });

    // At the last article; pressing j should not throw or navigate
    expect(() => fireEvent.keyDown(window, { key: 'j' })).not.toThrow();
  });

  it('does not navigate when pressing k at start of list without navigateToAdjacentEntry prop (All stream)', () => {
    render(
      <ArticleList
        {...getMockProps({
          selectionType: SelectionType.All,
        })}
      />
    );

    // Already at the first article (index 0); pressing k should NOT invoke navigation
    fireEvent.keyDown(window, { key: 'k' });

    expect(mockNavigateToAdjacentEntry).not.toHaveBeenCalled();
  });

  it('selects first article (not first unread) when switching to All stream', () => {
    // Unread stream articles (only unread shown)
    const unreadArticles: ArticleView[] = [
      {
        folderId: '1',
        feedId: '1',
        feedTitle: 'Test Feed',
        id: '2',
        title: 'Unread Article',
        author: '',
        html: '<p>Unread</p>',
        url: 'https://example.com/2',
        creationTime: 1678972809,
        isRead: false,
        isSaved: false,
      },
    ];

    // All stream articles (both read and unread)
    const allArticles: ArticleView[] = [
      {
        folderId: '1',
        feedId: '1',
        feedTitle: 'Test Feed',
        id: '1',
        title: 'Read Article',
        author: '',
        html: '<p>Read</p>',
        url: 'https://example.com/1',
        creationTime: 1678972810,
        isRead: true,
        isSaved: false,
      },
      {
        folderId: '1',
        feedId: '1',
        feedTitle: 'Test Feed',
        id: '2',
        title: 'Unread Article',
        author: '',
        html: '<p>Unread</p>',
        url: 'https://example.com/2',
        creationTime: 1678972809,
        isRead: false,
        isSaved: false,
      },
    ];

    const { container, rerender } = render(
      <ArticleList
        {...getMockProps({
          articleEntriesCls: unreadArticles,
          selectionKey: 'old-key' as SelectionKey,
          selectionType: SelectionType.Unread,
        })}
      />
    );

    // Unread stream: first unread article is selected
    expectArticleSelected(container, 'Unread Article', true);

    // Switch to All stream — articleEntriesCls changes, triggering selection reset
    rerender(
      <ArticleList
        {...getMockProps({
          articleEntriesCls: allArticles,
          selectionKey: 'new-key' as SelectionKey,
          selectionType: SelectionType.All,
        })}
      />
    );

    // All stream: first article is selected regardless of read status
    expectArticleSelected(container, 'Read Article', true);
  });

  it('does not call navigateToAdjacentEntry when pressing j before the last article', () => {
    const manyArticles: ArticleView[] = [
      ...mockArticles,
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

    render(
      <ArticleList
        {...getMockProps({
          articleEntriesCls: manyArticles,
          navigateToAdjacentEntry: mockNavigateToAdjacentEntry,
        })}
      />
    );

    // Press j once — moves from article 0 to 1, not at the end (3 articles)
    fireEvent.keyDown(window, { key: 'j' });

    expect(mockNavigateToAdjacentEntry).not.toHaveBeenCalled();
  });

  it('calls navigateToAdjacentEntry("prev") when pressing k at the first article', () => {
    render(
      <ArticleList
        {...getMockProps({
          navigateToAdjacentEntry: mockNavigateToAdjacentEntry,
        })}
      />
    );

    // Already at the first article (index 0); pressing k should invoke the callback
    fireEvent.keyDown(window, { key: 'k' });

    expect(mockNavigateToAdjacentEntry).toHaveBeenCalledWith(
      NavigationDirection.Prev
    );
    expect(mockNavigateToAdjacentEntry).toHaveBeenCalledTimes(1);
  });

  it('does not call navigateToAdjacentEntry when pressing k after the first article', () => {
    render(
      <ArticleList
        {...getMockProps({
          navigateToAdjacentEntry: mockNavigateToAdjacentEntry,
        })}
      />
    );

    // Move to the second article first
    fireEvent.keyDown(window, { key: 'j' });
    mockNavigateToAdjacentEntry.mockClear();

    // Press k — should go back to first article, not invoke the callback
    fireEvent.keyDown(window, { key: 'k' });

    expect(mockNavigateToAdjacentEntry).not.toHaveBeenCalled();
  });

  it('does not call navigateToAdjacentEntry when prop is not provided', () => {
    // No navigateToAdjacentEntry prop — pressing j at the last article should not throw
    render(<ArticleList {...getMockProps()} />);

    fireEvent.keyDown(window, { key: 'j' }); // go to last
    expect(() => fireEvent.keyDown(window, { key: 'j' })).not.toThrow();
    expect(() => fireEvent.keyDown(window, { key: 'k' })).not.toThrow();
  });

  it('maintains scroll position with stable sort', () => {
    // Create 20 articles where groups of 10 have same creation time
    const numArticles = 20;
    const articles: ArticleView[] = Array.from(
      { length: numArticles },
      (_, i) => ({
        folderId: '1',
        feedId: '1',
        feedTitle: 'Test Feed 1',
        id: `${i}`,
        title: `Test Article ${i}`,
        author: '',
        html: `<p>Test content ${i}</p>`,
        url: `https://example.com/${i}`,
        creationTime: 1678972810 - Math.floor(i / 10),
        isRead: false,
        isSaved: false,
      })
    );

    // Set threshold very high to force rendering all articles.
    const { container } = render(
      <ArticleList
        {...getMockProps({ articleEntriesCls: articles, threshold: 100000 })}
      />
    );

    // Scroll down and verify selection
    for (let i = 0; i < numArticles - 1; i++) {
      expectArticleSelected(container, `Test Article ${i}`, true);
      fireEvent.keyDown(window, { key: 'j' });
      expectArticleSelected(container, `Test Article ${i}`, false);
      expectArticleSelected(container, `Test Article ${i + 1}`, true);
    }

    // Scroll back up and verify selection
    for (let i = numArticles - 1; i > 0; i--) {
      expectArticleSelected(container, `Test Article ${i}`, true);
      fireEvent.keyDown(window, { key: 'k' });
      expectArticleSelected(container, `Test Article ${i}`, false);
      expectArticleSelected(container, `Test Article ${i - 1}`, true);
    }

    // Verify the order of articles in the list
    const articleListBox = container.querySelector(
      '.GoliathSplitViewArticleListBox'
    ) as HTMLElement;
    const articleElements = Array.from(
      articleListBox.querySelectorAll(
        '.GoliathArticleListBase .GoliathArticleListTitleType'
      )
    ).map((el) => el.textContent);

    const expectedOrder = articles.map((a) => a.title);

    expect(articleElements).toEqual(expectedOrder);
  });
});

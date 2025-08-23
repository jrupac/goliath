import React from 'react';
import { fireEvent, render, screen, within } from '@testing-library/react';
import ArticleList, { ArticleListProps } from '../ArticleList';
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

  const getMockProps = (
    props?: Partial<ArticleListProps>
  ): ArticleListProps => ({
    articleEntriesCls: mockArticles,
    faviconMap: new Map<FeedId, FaviconCls>(),
    selectionKey: '1' as SelectionKey,
    selectionType: SelectionType.Folder,
    selectAllCallback: mockSelectAllCallback,
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
    const { container } = render(<ArticleList {...getMockProps()} />);

    // Get button references
    const scrollDownButton = screen.getByLabelText('scroll down');
    const scrollUpButton = screen.getByLabelText('scroll up');

    // First, scroll down to select the second article (keyboard)
    fireEvent.keyDown(window, { key: 'j' });

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

    const { container, rerender } = render(
      <ArticleList {...getMockProps({ articleEntriesCls: initialArticles })} />
    );

    // Initial render, Article 1 should be selected (latest creationTime)
    expectArticleSelected(container, 'Initial Article 1', true);

    // Rerender with new articles
    rerender(
      <ArticleList {...getMockProps({ articleEntriesCls: newArticles })} />
    );

    // After rerender, the new article (Article 3) should be at the top and
    // selected
    expectArticleSelected(container, 'New Article 3', true);
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

    const { container, rerender } = render(
      <ArticleList {...getMockProps({ articleEntriesCls: initialArticles })} />
    );

    // Initially, Article 1 is selected
    expectArticleSelected(container, 'Test Article 1', true);

    // Scroll down to select Article 2
    fireEvent.keyDown(window, { key: 'j' });

    // Verify Article 2 is selected
    expectArticleSelected(container, 'Test Article 1', false);
    expectArticleSelected(container, 'Test Article 2', true);

    // Simulate an update where Article 1 becomes read and is no longer part of
    // the props.
    const updatedArticles: ArticleView[] = [
      initialArticles[1],
      initialArticles[2],
    ];

    rerender(
      <ArticleList {...getMockProps({ articleEntriesCls: updatedArticles })} />
    );

    // Assert that Article 2 is still selected (no jump to top)
    expectArticleSelected(container, 'Test Article 1', false);
    expectArticleSelected(container, 'Test Article 2', true);

    // Assert no duplicate articles
    const articleListBox = container.querySelector(
      '.GoliathSplitViewArticleListBox'
    ) as HTMLElement;
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

    const { container, rerender } = render(
      <ArticleList {...getMockProps({ articleEntriesCls: initialArticles })} />
    );

    const articleListBox = container.querySelector(
      '.GoliathSplitViewArticleListBox'
    ) as HTMLElement;

    // Scroll down to select Article 2
    fireEvent.keyDown(window, { key: 'j' });

    // Verify Article 2 is selected
    expectArticleSelected(container, 'Test Article 1', false);
    expectArticleSelected(container, 'Test Article 2', true);

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

    rerender(
      <ArticleList {...getMockProps({ articleEntriesCls: updatedArticles })} />
    );

    // Assert that the new article (Article 0) is selected (implying that the
    // scroll position went to the top)
    expectArticleSelected(container, 'New Test Article 0', true);
    expectArticleSelected(container, 'Test Article 1', false);
    expectArticleSelected(container, 'Test Article 2', false);

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

import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import ArticleList, { ArticleListProps } from '../ArticleList';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ArticleView } from '../../models/article';
import { expectTextInElement } from './helpers';
import {
  MarkState,
  NavigationDirection,
  SelectionKey,
  SelectionType,
} from '../../utils/types';
import { FaviconCls, FeedId } from '../../models/feed';
import { FetchAPI } from '../../api/interface';

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
    fetchApi: {} as FetchAPI,
    handleUpdateArticleParsed: vi.fn(),
    articleEntriesCls: mockArticles,
    faviconMap: new Map<FeedId, FaviconCls>(),
    selectionKey: '1' as SelectionKey,
    selectionType: SelectionType.Folder,
    selectAllCallback: mockSelectAllCallback,
    selectUnreadCallback: mockSelectUnreadCallback,
    handleMark: mockHandleMark,
    buildTimestamp: '',
    buildHash: '',
    isMobile: false,
    isTabletPortrait: false,
    isTabletLandscape: false,
    mobilePane: 'list',
    tabletShowFeedList: true,
    onMobileNavigate: vi.fn(),
    onArticleSelect: vi.fn(),
    openDrawer: vi.fn(),
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
    const card = articleEntry.closest('.GoliathArticleCard');
    expect(card).not.toBeNull();
    if (selected && card) {
      const hasSelectedClass = card.classList.contains(
        'GoliathArticleCardSelected'
      );
      expect(hasSelectedClass).toBe(true);
    }
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
    expect(articleTitleElement.length).toBeGreaterThan(0);
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

  it('selects next article on scroll down with arrow key without marking as read', () => {
    const { container } = render(<ArticleList {...getMockProps()} />);

    // Before scroll, Article 1 is selected
    expectArticleSelected(container, 'Test Article 1', true);

    // Simulate scroll down with ArrowDown key
    fireEvent.keyDown(window, { key: 'ArrowDown' });

    // Check that handleMark was NOT called
    expect(mockHandleMark).not.toHaveBeenCalled();

    // After scroll, Article 2 should be selected
    expectArticleSelected(container, 'Test Article 1', false);
    expectArticleSelected(container, 'Test Article 2', true);
  });

  it('selects previous article on scroll up with arrow key without marking as read', () => {
    const { container } = render(<ArticleList {...getMockProps()} />);

    // First scroll down with ArrowDown to select Article 2
    fireEvent.keyDown(window, { key: 'ArrowDown' });
    expectArticleSelected(container, 'Test Article 2', true);

    // Simulate scroll up with ArrowUp key
    fireEvent.keyDown(window, { key: 'ArrowUp' });

    // Check that handleMark was NOT called
    expect(mockHandleMark).not.toHaveBeenCalled();

    // After scroll up, Article 1 should be selected
    expectArticleSelected(container, 'Test Article 1', true);
    expectArticleSelected(container, 'Test Article 2', false);
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
    const entries = articleListBox.querySelectorAll('.GoliathArticleCard');
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
        '.GoliathArticleCard .GoliathArticleCardTitle'
      )
    ).map((el) => el.textContent);

    const expectedOrder = articles.map((a) => a.title);

    expect(articleElements).toEqual(expectedOrder);
  });

  it('toggles article read status via toggle icon click', async () => {
    const { container } = render(<ArticleList {...getMockProps()} />);

    const articleListBox = container.querySelector(
      '.GoliathSplitViewArticleListBox'
    ) as HTMLElement;
    const firstCard = articleListBox.querySelector(
      '.GoliathArticleCard'
    ) as HTMLElement;

    fireEvent.mouseEnter(firstCard);
    const toggleIcon = firstCard.querySelector(
      '[data-testid="FiberManualRecordIcon"]'
    ) as HTMLElement;
    fireEvent.click(toggleIcon);

    // Should call handleMark with MarkState.Read to mark it as read
    expect(mockHandleMark).toHaveBeenCalledWith(
      MarkState.Read,
      [mockArticles[0].id, mockArticles[0].feedId, mockArticles[0].folderId],
      SelectionType.Article
    );
  });

  it('toggles article from read to unread via toggle icon click', async () => {
    const readArticle: ArticleView = {
      folderId: '1',
      feedId: '1',
      feedTitle: 'Test Feed 1',
      id: '99',
      title: 'Read Article',
      author: '',
      html: '<p>Already read</p>',
      url: 'https://example.com/99',
      creationTime: 1678972810,
      isRead: true,
      isSaved: false,
    };

    const { container } = render(
      <ArticleList
        {...getMockProps({
          articleEntriesCls: [readArticle, ...mockArticles],
        })}
      />
    );

    const articleListBox = container.querySelector(
      '.GoliathSplitViewArticleListBox'
    ) as HTMLElement;
    // First card is the read article
    const firstCard = articleListBox.querySelector(
      '.GoliathArticleCard'
    ) as HTMLElement;

    fireEvent.mouseEnter(firstCard);
    const toggleIcon = firstCard.querySelector(
      '[data-testid="RadioButtonUncheckedIcon"]'
    ) as HTMLElement;
    expect(toggleIcon).toBeInTheDocument();
    fireEvent.click(toggleIcon);

    // Should call handleMark with MarkState.Unread to mark it as unread
    expect(mockHandleMark).toHaveBeenCalledWith(
      MarkState.Unread,
      [readArticle.id, readArticle.feedId, readArticle.folderId],
      SelectionType.Article
    );
  });

  it('triggers selectUnreadCallback on "g u" sequential shortcut', () => {
    render(<ArticleList {...getMockProps()} />);

    // Press 'g' then 'u'
    fireEvent.keyDown(window, { key: 'g' });
    fireEvent.keyDown(window, { key: 'u' });

    expect(mockSelectUnreadCallback).toHaveBeenCalledTimes(1);
  });

  it('triggers selectAllCallback on "g a" sequential shortcut', () => {
    render(<ArticleList {...getMockProps()} />);

    // Press 'g' then 'a'
    fireEvent.keyDown(window, { key: 'g' });
    fireEvent.keyDown(window, { key: 'a' });

    expect(mockSelectAllCallback).toHaveBeenCalledTimes(1);
  });

  it('triggers handleMarkAllRead on "Shift+I" chord shortcut', () => {
    render(<ArticleList {...getMockProps()} />);

    // Press Shift then I
    fireEvent.keyDown(window, { key: 'Shift' });
    fireEvent.keyDown(window, { key: 'I', shiftKey: true });

    expect(mockHandleMark).toHaveBeenCalledWith(
      MarkState.Read,
      '1',
      SelectionType.Folder
    );
  });

  it('toggles previews on "p" shortcut', () => {
    const { container } = render(<ArticleList {...getMockProps()} />);

    const body = container.querySelector('.GoliathArticleCardBody');
    expect(body).not.toBeNull();
    // Initially showPreviews is true, so both text and ImagePreview wrapper are rendered
    expect(body?.childNodes.length).toBe(2);

    // Trigger 'p'
    fireEvent.keyDown(window, { key: 'p' });

    // After toggling, ImagePreview is unmounted, leaving only the text child
    expect(body?.childNodes.length).toBe(1);
  });

  it('opens article url in a new tab on "v" shortcut', () => {
    const originalOpen = window.open;
    window.open = vi.fn();

    render(<ArticleList {...getMockProps()} />);

    // Trigger 'v'
    fireEvent.keyDown(window, { key: 'v' });

    expect(window.open).toHaveBeenCalledWith('https://example.com/1', '_blank');
    expect(mockHandleMark).toHaveBeenCalledTimes(1);

    window.open = originalOpen;
  });

  it('triggers clearReadCallback on "c" shortcut when in folder selection', () => {
    const mockClearReadCallback = vi.fn();
    render(
      <ArticleList
        {...getMockProps({
          selectionType: SelectionType.Folder,
          clearReadCallback: mockClearReadCallback,
        })}
      />
    );

    // Trigger 'c'
    fireEvent.keyDown(window, { key: 'c' });

    expect(mockClearReadCallback).toHaveBeenCalledTimes(1);
    expect(mockClearReadCallback).toHaveBeenCalledWith('1'); // ID of the first article
  });

  it('does not trigger clearReadCallback on "c" shortcut when in All stream', () => {
    const mockClearReadCallback = vi.fn();
    render(
      <ArticleList
        {...getMockProps({
          selectionType: SelectionType.All,
          clearReadCallback: mockClearReadCallback,
        })}
      />
    );

    // Trigger 'c'
    fireEvent.keyDown(window, { key: 'c' });

    expect(mockClearReadCallback).not.toHaveBeenCalled();
  });

  it('renders HotelClassTwoTone icon in Saved selectionType and toggles saved state', () => {
    render(
      <ArticleList
        {...getMockProps({
          selectionType: SelectionType.Saved,
          selectionKey: 'SAVED',
        })}
      />
    );

    const toggleIcon = screen.getByLabelText('mark all as unsaved');
    expect(toggleIcon).toBeInTheDocument();
    expect(screen.getByTestId('HotelClassTwoToneIcon')).toBeInTheDocument();

    fireEvent.click(toggleIcon);

    expect(mockHandleMark).toHaveBeenCalledWith(
      MarkState.Saved,
      'SAVED',
      SelectionType.Saved
    );
  });

  it('renders HotelClassRounded when selectionType is Saved and list is empty', () => {
    render(
      <ArticleList
        {...getMockProps({
          articleEntriesCls: [],
          selectionType: SelectionType.Saved,
        })}
      />
    );

    expect(screen.getByTestId('HotelClassRoundedIcon')).toBeInTheDocument();
    expect(screen.queryByTestId('DoneAllRoundedIcon')).not.toBeInTheDocument();
  });

  it('toggles save status of currently selected article on "s" shortcut when unsaved', () => {
    render(<ArticleList {...getMockProps()} />);

    // Trigger 's'
    fireEvent.keyDown(window, { key: 's' });

    expect(mockHandleMark).toHaveBeenCalledTimes(1);
    expect(mockHandleMark).toHaveBeenCalledWith(
      MarkState.Saved,
      ['1', '1', '1'],
      SelectionType.Article
    );
  });

  it('toggles save status of currently selected article on "s" shortcut when saved', () => {
    const savedArticles = [
      {
        ...mockArticles[0],
        isSaved: true,
      },
      mockArticles[1],
    ];

    render(
      <ArticleList
        {...getMockProps({
          articleEntriesCls: savedArticles,
        })}
      />
    );

    // Trigger 's'
    fireEvent.keyDown(window, { key: 's' });

    expect(mockHandleMark).toHaveBeenCalledTimes(1);
    expect(mockHandleMark).toHaveBeenCalledWith(
      MarkState.Unsaved,
      ['1', '1', '1'],
      SelectionType.Article
    );
  });

  it('triggers selectSavedCallback on "g s" shortcut and does not trigger toggleSave', () => {
    vi.useFakeTimers();
    const mockSelectSavedCallback = vi.fn();
    render(
      <ArticleList
        {...getMockProps({
          selectSavedCallback: mockSelectSavedCallback,
        })}
      />
    );

    // Trigger 'g' then 's' quickly
    fireEvent.keyDown(window, { key: 'g' });
    vi.advanceTimersByTime(100);
    fireEvent.keyDown(window, { key: 's' });

    expect(mockSelectSavedCallback).toHaveBeenCalledTimes(1);
    // Standalone toggleSave handler should be skipped because of suffix collision
    expect(mockHandleMark).not.toHaveBeenCalled();

    vi.useRealTimers();
  });

  it('triggers toggleSave on "s" if "g" was pressed more than 1000ms ago', () => {
    vi.useFakeTimers();
    render(<ArticleList {...getMockProps()} />);

    // Trigger 'g' then wait 1500ms and trigger 's'
    fireEvent.keyDown(window, { key: 'g' });
    vi.advanceTimersByTime(1500);
    fireEvent.keyDown(window, { key: 's' });

    // Standalone toggleSave handler should execute because timing gap > 1000ms
    expect(mockHandleMark).toHaveBeenCalledTimes(1);
    expect(mockHandleMark).toHaveBeenCalledWith(
      MarkState.Saved,
      ['1', '1', '1'],
      SelectionType.Article
    );

    vi.useRealTimers();
  });

  it('renders menu button on mobile or tablet portrait (when feed list is hidden) and triggers openDrawer when clicked', () => {
    const mockOpenDrawer = vi.fn();
    const mockOnMobileNavigate = vi.fn();

    const { rerender } = render(
      <ArticleList
        {...getMockProps({
          isMobile: true,
          isTabletPortrait: false,
          mobilePane: 'list',
          onMobileNavigate: mockOnMobileNavigate,
          openDrawer: mockOpenDrawer,
        })}
      />
    );

    const menuButton = screen.getByLabelText('open navigation menu');
    expect(menuButton).toBeInTheDocument();
    fireEvent.click(menuButton);
    expect(mockOpenDrawer).toHaveBeenCalledTimes(1);

    // Rerender as tablet portrait with feed list hidden, should render menu button
    rerender(
      <ArticleList
        {...getMockProps({
          isMobile: false,
          isTabletPortrait: true,
          tabletShowFeedList: false,
          mobilePane: 'list',
          onMobileNavigate: mockOnMobileNavigate,
          openDrawer: mockOpenDrawer,
        })}
      />
    );
    expect(screen.getByLabelText('open navigation menu')).toBeInTheDocument();
  });

  it('applies display: none to columns on mobile based on mobilePane', () => {
    const { container, rerender } = render(
      <ArticleList
        {...getMockProps({
          isMobile: true,
          isTabletPortrait: false,
          mobilePane: 'list',
          onMobileNavigate: vi.fn(),
          openDrawer: vi.fn(),
        })}
      />
    );

    const listCol = container.querySelector('.GoliathArticleListColumn');
    const cardCol = container.querySelector('.GoliathSplitViewArticleOuter');

    expect(listCol).not.toHaveStyle('display: none');
    expect(cardCol).toHaveStyle('display: none');

    // Rerender with mobilePane = 'card'
    rerender(
      <ArticleList
        {...getMockProps({
          isMobile: true,
          isTabletPortrait: false,
          mobilePane: 'card',
          onMobileNavigate: vi.fn(),
          openDrawer: vi.fn(),
        })}
      />
    );

    expect(listCol).toHaveStyle('display: none');
    expect(cardCol).not.toHaveStyle('display: none');
  });

  it('navigates to card view and calls onArticleSelect on article selection when in mobile mode', () => {
    const mockOnMobileNavigate = vi.fn();
    const mockOnArticleSelect = vi.fn();
    const { container } = render(
      <ArticleList
        {...getMockProps({
          isMobile: true,
          isTabletPortrait: false,
          mobilePane: 'list',
          onMobileNavigate: mockOnMobileNavigate,
          onArticleSelect: mockOnArticleSelect,
          openDrawer: vi.fn(),
        })}
      />
    );

    const articleListBox = container.querySelector(
      '.GoliathSplitViewArticleListBox'
    ) as HTMLElement;
    const entries = articleListBox.querySelectorAll('.GoliathArticleCard');
    fireEvent.click(entries[1]);

    expect(mockOnMobileNavigate).toHaveBeenCalledWith('card');
    expect(mockOnMobileNavigate).toHaveBeenCalledTimes(1);
    expect(mockOnArticleSelect).toHaveBeenCalledTimes(1);
  });

  it('applies display: none to Card column on tablet portrait based on tabletShowFeedList', () => {
    const { container, rerender } = render(
      <ArticleList
        {...getMockProps({
          isMobile: false,
          isTabletPortrait: true,
          tabletShowFeedList: true,
        })}
      />
    );

    const cardCol = container.querySelector('.GoliathSplitViewArticleOuter');
    expect(cardCol).toHaveStyle('display: none');

    // Rerender with tabletShowFeedList = false
    rerender(
      <ArticleList
        {...getMockProps({
          isMobile: false,
          isTabletPortrait: true,
          tabletShowFeedList: false,
        })}
      />
    );

    expect(cardCol).not.toHaveStyle('display: none');
  });
});

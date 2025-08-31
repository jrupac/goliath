import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import ArticleList, { ArticleListProps } from '../ArticleList';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ArticleView } from '../../models/article';
import { expectClass, expectTextInElement } from './helpers';
import { MarkState, SelectionKey, SelectionType } from '../../utils/types';
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

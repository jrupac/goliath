import React from 'react';
import {render, screen} from '@testing-library/react';
import ArticleList from '../ArticleList';
import {describe, expect, it, vi} from 'vitest';
import {ArticleView} from "../../models/article";
import {SelectionKey, SelectionType} from "../../utils/types";

describe('ArticleList', () => {
  it('renders', () => {
    // Mock ArticleView data
    const mockArticles: ArticleView[] = [
      {
        folderId: "1",
        feedId: "1",
        feedTitle: 'Test Feed 1',
        favicon: '',
        id: '1',
        title: 'Test Article 1',
        author: '',
        html: '<p>Test content 1</p>',
        url: 'https://example.com/1',
        creationTime: 1678886400, // March 15, 2023
        isRead: false,
        isSaved: false,
      },
      {
        folderId: "2",
        feedId: "2",
        feedTitle: 'Test Feed 2',
        favicon: '',
        id: '2',
        title: 'Test Article 2',
        author: '',
        html: '<p>Test content 2</p>',
        url: 'https://example.com/2',
        creationTime: 1678972800, // March 16, 2023
        isRead: false,
        isSaved: false,
      },
    ];

    // Mock functions
    const mockSelectAllCallback = vi.fn();
    const mockHandleMark = vi.fn();

    // Minimal rendering test
    const props = {
      articleEntriesCls: mockArticles,
      selectionKey: "1" as SelectionKey,
      selectionType: SelectionType.Folder,
      selectAllCallback: mockSelectAllCallback,
      handleMark: mockHandleMark,
    };
    const {container} = render(<ArticleList {...props} />);

    // Basic assertion to check if it renders without crashing
    expect(container).toBeDefined();
    const articleTitleElement = screen.getAllByText(
      'Test Article 1');
    expect(articleTitleElement).is.not.empty;
  });
});
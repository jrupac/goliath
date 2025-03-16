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
        id: '1',
        title: 'Test Article 1',
        author: '',
        url: 'https://example.com/1',
        created_on_time: 1678886400, // March 15, 2023
        html: '<p>Test content 1</p>',
        is_read: 0,
        feed_id: "1",
        folder_id: "1",
        feed_title: 'Test Feed 1',
        favicon: '',
      },
      {
        id: '2',
        title: 'Test Article 2',
        author: '',
        url: 'https://example.com/2',
        created_on_time: 1678972800, // March 16, 2023
        html: '<p>Test content 2</p>',
        is_read: 0,
        feed_id: "2",
        folder_id: "2",
        feed_title: 'Test Feed 2',
        favicon: '',
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
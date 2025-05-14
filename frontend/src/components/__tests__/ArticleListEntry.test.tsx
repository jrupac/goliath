import React from 'react';
import {render} from '@testing-library/react';
import {describe, it} from 'vitest';
import ArticleListEntry from '../ArticleListEntry';
import {ArticleView} from "../../models/article";

describe('ArticleListEntry', () => {
  it('renders', () => {
    const mockArticleView: ArticleView = {
      folderId: "1",
      feedId: "1",
      feedTitle: 'Test Feed',
      favicon: '',
      id: '1',
      title: 'Test Article',
      author: '',
      html: '<p>Test content</p>',
      url: 'https://example.com',
      creationTime: 1678886400, // March 15, 2023
      isRead: false,
      isSaved: false,
    }

    const props = {
      articleView: mockArticleView,
      preview: undefined,
      selected: false,
    };
    render(<ArticleListEntry {...props} />);
  });
});
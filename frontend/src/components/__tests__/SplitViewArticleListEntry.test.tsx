import React from 'react';
import {render} from '@testing-library/react';
import {describe, it} from 'vitest';
import SplitViewArticleListEntry from '../SplitViewArticleListEntry';
import {ArticleView} from "../../models/article";

describe('SplitViewArticleListEntry', () => {
  it('renders', () => {
    const mockArticleView: ArticleView = {
      id: '1',
      title: 'Test Article',
      url: 'https://example.com',
      created_on_time: 1678886400, // March 15, 2023
      html: '<p>Test content</p>',
      is_read: 0,
      feed_id: "1",
      folder_id: "1",
      feed_title: 'Test Feed',
      favicon: '',
      author: '',
    }

    const props = {
      articleView: mockArticleView,
      preview: undefined,
      selected: false,
    };
    render(<SplitViewArticleListEntry {...props} />);
  });
});
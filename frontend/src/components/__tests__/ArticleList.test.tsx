import React from 'react';
import { render } from '@testing-library/react';
import ArticleList from '../ArticleList';

describe('ArticleList', () => {
  it('renders', () => {
    // Minimal rendering test
    const props = {
      articles: [],
      unreadCount: 0,
      selectedArticleId: null,
      onSelectArticle: () => {},
    };
    render(<ArticleList {...props} />);
  });
});
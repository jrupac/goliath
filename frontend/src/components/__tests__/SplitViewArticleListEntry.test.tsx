import React from 'react';
import { render } from '@testing-library/react';
import SplitViewArticleListEntry from '../SplitViewArticleListEntry';

describe('SplitViewArticleListEntry', () => {
  it('renders', () => {
    // Minimal rendering test
    const props = {
      article: { id: '1', title: 'Test Article', url: 'http://example.com', created_on_time: 0, is_read: 0 },
      isSelected: false,
      onSelectArticle: () => {},
    };
    render(<SplitViewArticleListEntry {...props} />);
  });
});
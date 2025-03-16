import React from 'react';
import { render } from '@testing-library/react';
import SplitViewArticleCard from '../SplitViewArticleCard';

describe('SplitViewArticleCard', () => {
  it('renders', () => {
    // Minimal rendering test
    const props = {
      article: { id: '1', title: 'Test Article', url: 'http://example.com', created_on_time: 0, html: '', is_read: 0 },
      feedTitle: 'Test Feed',
      favicon: '',
      isSelected: false,
      shouldRerender: () => {},
    };
    render(<SplitViewArticleCard {...props} />);
  });
});
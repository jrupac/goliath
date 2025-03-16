import React from 'react';
import {describe, it, vi} from 'vitest';
import {render} from '@testing-library/react';
import SplitViewArticleCard from '../SplitViewArticleCard';
import {ArticleView} from "../../models/article";

describe('SplitViewArticleCard', () => {
  it('renders', () => {
    // Minimal rendering test
    const props = {
      article: {
        id: '1',
        title: 'Test Article',
        author: '',
        url: 'https://example.com',
        created_on_time: 0,
        html: '',
        is_read: 0,
        feed_id: "1",
        folder_id: "1",
        feed_title: 'Test Feed',
        favicon: '',
      } as ArticleView,
      title: 'Test Feed',
      favicon: '',
      isSelected: false,
      shouldRerender: vi.fn(),
    };
    render(<SplitViewArticleCard {...props} />);
  });
});
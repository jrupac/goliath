import React from 'react';
import {ArticleCard} from '../ArticleCard';
import {describe, expect, it, vi} from 'vitest';
import {render, screen} from '@testing-library/react';
import {ArticleView} from '../../models/article';

describe('ArticleCard', () => {
  it('renders without crashing with minimal props', () => {
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
    const {container} = render(<ArticleCard {...props} />);
    expect(container).toBeDefined();
  });
});

describe('ArticleCard', () => {
  const article: ArticleView = {
    id: '1',
    title: 'Test Article',
    author: '',
    url: 'https://example.com',
    created_on_time: 1678886400, // March 15, 2023
    html: '<p>Test content</p>',
    is_read: 0,
    feed_id: "1",
    folder_id: "1",
    feed_title: 'Test Feed',
    favicon: '',
  };
  const props = {
    article: article,
    title: 'Test Feed',
    favicon: '',
    isSelected: false,
    shouldRerender: vi.fn(),
  };

  it('renders without crashing', () => {
    render(<ArticleCard {...props} />);
  });

  it('displays the correct title, feed title, and date', () => {
    render(<ArticleCard {...props} />);
    expect(screen.getByText('Test Article')).toBeInTheDocument();
    expect(screen.getByText('Test Feed')).toBeInTheDocument();
    expect(screen.getByText('Wed, Mar 15, 9:20 AM')).toBeInTheDocument();
  });
});
import React from 'react';
import { render, screen } from '@testing-library/react';
import ArticleCard from '../ArticleCard';
import React from 'react';
import { render } from '@testing-library/react';
import ArticleCard from '../ArticleCard';

describe('ArticleCard', () => {
  it('renders', () => {
    // Minimal rendering test
    const props = {
      article: { id: '1', title: 'Test Article', url: 'http://example.com', created_on_time: 0, html: '', is_read: 0 },
      title: 'Test Feed',
      favicon: '',
      isSelected: false,
      shouldRerender: () => {},
    };
    render(<ArticleCard {...props} />);
  });
});
describe('ArticleCard', () => {
  const article = {
    id: '1',
    title: 'Test Article',
    url: 'http://example.com',
    created_on_time: 1678886400, // March 15, 2023
    html: '<p>Test content</p>',
    is_read: 0,
  };
  const props = {
    article: article,
    title: 'Test Feed',
    favicon: '',
    isSelected: false,
    shouldRerender: () => {},
  };

  it('renders without crashing', () => {
    render(<ArticleCard {...props} />);
  });

  it('displays the correct title, feed title, and date', () => {
    render(<ArticleCard {...props} />);
    expect(screen.getByText('Test Article')).toBeInTheDocument();
    expect(screen.getByText('Test Feed')).toBeInTheDocument();
    expect(screen.getByText('Mar 15')).toBeInTheDocument(); // Assuming formatFriendly returns "Mar 15"
  });
});
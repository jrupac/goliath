import React from 'react';
import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import ArticleListEntry from '../ArticleListEntry';
import { ArticleView } from '../../models/article';
import { FaviconCls } from '../../models/feed';

const mockArticleView: ArticleView = {
  folderId: '1',
  feedId: '1',
  feedTitle: 'Test Feed',
  id: '1',
  title: 'Test Article',
  author: '',
  html: '<p>Test content</p>',
  url: 'https://example.com',
  creationTime: 1678886400, // March 15, 2023
  isRead: false,
  isSaved: false,
};

describe('ArticleListEntry', () => {
  it('renders', () => {
    render(
      <ArticleListEntry
        articleView={mockArticleView}
        favicon={undefined}
        selected={false}
        showPreviews={false}
      />
    );
  });

  it('renders RssFeedIcon when favicon is absent', () => {
    render(
      <ArticleListEntry
        articleView={mockArticleView}
        favicon={new FaviconCls(undefined)}
        selected={false}
        showPreviews={false}
      />
    );
    expect(screen.getByTestId('RssFeedIcon')).toBeInTheDocument();
  });

  it('renders favicon avatar with correct src when favicon is provided', () => {
    // 1x1 transparent GIF as a full data URL
    const testFaviconData =
      'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7';

    render(
      <ArticleListEntry
        articleView={mockArticleView}
        favicon={new FaviconCls(testFaviconData)}
        selected={false}
        showPreviews={false}
      />
    );

    const img = screen.getByRole('img', { name: 'Test Article' });
    expect(img).toHaveAttribute('src', testFaviconData);
  });
});

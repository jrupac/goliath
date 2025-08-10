import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import ArticleCard from '../ArticleCard';
import { ArticleView } from '../../models/article';
import { FaviconCls } from '../../models/feed';
import { formatFriendly } from '../../utils/helpers';

describe('ArticleCard', () => {
  it('renders', () => {
    // Minimal rendering test
    const props = {
      article: {
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
      } as ArticleView,
      title: 'Test Feed',
      favicon: new FaviconCls(''),
      isSelected: false,
      onMarkArticleRead: () => {},
    };
    render(<ArticleCard {...props} />);
  });

  it('renders with all props correctly', () => {
    const articleUrl = 'https://example.com/test-article';
    const articleCreationTime = 1678886400; // March 15, 2023
    const friendlyFormattedDate = formatFriendly(
      new Date(articleCreationTime * 1000)
    );

    const props = {
      article: {
        folderId: '1',
        feedId: '1',
        feedTitle: 'Test Feed',
        id: '1',
        title: 'Test Article',
        author: 'Test Author',
        html: '<p>Test content</p>',
        url: articleUrl,
        creationTime: articleCreationTime,
        isRead: false,
        isSaved: false,
      } as ArticleView,
      title: 'Test Feed Title',
      favicon: new FaviconCls(undefined),
      isSelected: false,
      onMarkArticleRead: () => {},
    };
    render(<ArticleCard {...props} />);

    // Assert feed title
    expect(screen.getByText('Test Feed Title')).toBeInTheDocument();

    // Assert article title
    expect(screen.getByText('Test Article')).toBeInTheDocument();

    // Assert article content
    expect(screen.getByText('Test content')).toBeInTheDocument();

    // Assert article URL link
    expect(screen.getByRole('link', { name: 'Test Article' })).toHaveAttribute(
      'href',
      articleUrl
    );

    // Assert that the RssFeedOutlinedIcon is rendered when favicon is undefined
    expect(screen.getByTestId('RssFeedOutlinedIcon')).toBeInTheDocument();

    // Assert friendly formatted date
    expect(screen.getByText(friendlyFormattedDate)).toBeInTheDocument();
  });

  it('renders the correct favicon image when provided', () => {
    // 1x1 transparent GIF
    const testFaviconData =
      'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7';

    const props = {
      article: {
        folderId: '1',
        feedId: '1',
        feedTitle: 'Test Feed',
        id: '1',
        title: 'Test Article',
        author: '',
        html: '<p>Test content</p>',
        url: 'https://example.com',
        creationTime: 1678886400,
        isRead: false,
        isSaved: false,
      } as ArticleView,
      title: 'Test Feed',
      favicon: new FaviconCls(testFaviconData),
      isSelected: false,
      onMarkArticleRead: () => {},
    };
    render(<ArticleCard {...props} />);

    // Assert favicon image src
    expect(screen.getByRole('img', { name: '' })).toHaveAttribute(
      'src',
      `data:${testFaviconData}`
    );
  });

  it('calls onMarkArticleRead when mark as read button is clicked', () => {
    const mockOnMarkArticleRead = vi.fn();
    const props = {
      article: {
        folderId: '1',
        feedId: '1',
        feedTitle: 'Test Feed',
        id: '1',
        title: 'Test Article',
        author: '',
        html: '<p>Test content</p>',
        url: 'https://example.com',
        creationTime: 1678886400,
        isRead: false, // Ensure article is unread for this test
        isSaved: false,
      } as ArticleView,
      title: 'Test Feed',
      favicon: new FaviconCls(''),
      isSelected: true, // Ensure it's selected so keydown handler is active if needed
      onMarkArticleRead: mockOnMarkArticleRead,
    };
    render(<ArticleCard {...props} />);

    // Get the mark as read button
    const markAsReadButton = screen.getByLabelText('mark as read');

    // Simulate click
    fireEvent.click(markAsReadButton);

    // Assert that the onMarkArticleRead callback was called
    expect(mockOnMarkArticleRead).toHaveBeenCalledTimes(1);
  });
});

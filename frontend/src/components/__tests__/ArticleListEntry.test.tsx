import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
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

  it('calls onSelect when clicked', () => {
    const onSelect = vi.fn();
    render(
      <ArticleListEntry
        articleView={mockArticleView}
        favicon={undefined}
        selected={false}
        showPreviews={false}
        onSelect={onSelect}
      />
    );
    fireEvent.click(screen.getByText('Test Article'));
    expect(onSelect).toHaveBeenCalledTimes(1);
  });

  it('does not throw when clicked without onSelect', () => {
    render(
      <ArticleListEntry
        articleView={mockArticleView}
        favicon={undefined}
        selected={false}
        showPreviews={false}
      />
    );
    expect(() =>
      fireEvent.click(screen.getByText('Test Article'))
    ).not.toThrow();
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

    const img = screen.getByRole('img', { name: '' });
    expect(img).toHaveAttribute('src', testFaviconData);
  });

  it('shows CheckCircle icon on hover for unread article', () => {
    render(
      <ArticleListEntry
        articleView={mockArticleView}
        favicon={new FaviconCls('')}
        selected={false}
        showPreviews={false}
      />
    );
    const entry = document.querySelector('.GoliathArticleListBase') as HTMLElement;
    fireEvent.mouseEnter(entry);
    expect(screen.getByTestId('CheckCircleTwoToneIcon')).toBeInTheDocument();
  });

  it('shows Check icon on hover for read article', () => {
    const readArticle = { ...mockArticleView, isRead: true };
    render(
      <ArticleListEntry
        articleView={readArticle}
        favicon={new FaviconCls('')}
        selected={false}
        showPreviews={false}
      />
    );
    const entry = document.querySelector('.GoliathArticleListBase') as HTMLElement;
    fireEvent.mouseEnter(entry);
    expect(screen.getByTestId('CheckTwoToneIcon')).toBeInTheDocument();
  });

  it('calls onToggleRead when toggle icon is clicked', () => {
    const onToggleRead = vi.fn();
    render(
      <ArticleListEntry
        articleView={mockArticleView}
        favicon={new FaviconCls('')}
        selected={false}
        showPreviews={false}
        onToggleRead={onToggleRead}
      />
    );
    const entry = document.querySelector('.GoliathArticleListBase') as HTMLElement;
    fireEvent.mouseEnter(entry);
    const toggleIcon = screen.getByTestId('CheckCircleTwoToneIcon');
    fireEvent.click(toggleIcon);
    expect(onToggleRead).toHaveBeenCalledWith('1');
  });

  it('does not trigger onSelect when toggle icon is clicked', () => {
    const onSelect = vi.fn();
    const onToggleRead = vi.fn();
    render(
      <ArticleListEntry
        articleView={mockArticleView}
        favicon={new FaviconCls('')}
        selected={false}
        showPreviews={false}
        onSelect={onSelect}
        onToggleRead={onToggleRead}
      />
    );
    const entry = document.querySelector('.GoliathArticleListBase') as HTMLElement;
    fireEvent.mouseEnter(entry);
    const toggleIcon = screen.getByTestId('CheckCircleTwoToneIcon');
    fireEvent.click(toggleIcon);
    expect(onToggleRead).toHaveBeenCalledTimes(1);
    expect(onSelect).not.toHaveBeenCalled();
  });
});

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
  creationTime: 1678886400,
  isRead: false,
  isSaved: false,
};

describe('ArticleListEntry', () => {
  it('renders with correct structure', () => {
    render(
      <ArticleListEntry
        articleView={mockArticleView}
        favicon={undefined}
        feedTitle={mockArticleView.feedTitle}
        feedId={mockArticleView.feedId}
        selected={false}
        showPreviews={false}
      />
    );
    expect(document.querySelector('.GoliathArticleCard')).toBeInTheDocument();
  });

  it('shows FeedIcon (initials) when not hovered and no favicon', () => {
    render(
      <ArticleListEntry
        articleView={mockArticleView}
        favicon={undefined}
        feedTitle={mockArticleView.feedTitle}
        feedId={mockArticleView.feedId}
        selected={false}
        showPreviews={false}
      />
    );
    expect(document.querySelector('.GoliathFeedIcon')).toBeInTheDocument();
    expect(
      screen.queryByTestId('FiberManualRecordIcon')
    ).not.toBeInTheDocument();
  });

  it('renders unread dot on hover for unread article', () => {
    render(
      <ArticleListEntry
        articleView={mockArticleView}
        favicon={undefined}
        feedTitle={mockArticleView.feedTitle}
        feedId={mockArticleView.feedId}
        selected={false}
        showPreviews={false}
      />
    );
    fireEvent.mouseEnter(document.querySelector('.GoliathArticleCard')!);
    expect(screen.getByTestId('FiberManualRecordIcon')).toBeInTheDocument();
  });

  it('renders read dot on hover for read article', () => {
    render(
      <ArticleListEntry
        articleView={{ ...mockArticleView, isRead: true }}
        favicon={undefined}
        feedTitle={mockArticleView.feedTitle}
        feedId={mockArticleView.feedId}
        selected={false}
        showPreviews={false}
      />
    );
    fireEvent.mouseEnter(document.querySelector('.GoliathArticleCard')!);
    expect(screen.getByTestId('RadioButtonUncheckedIcon')).toBeInTheDocument();
  });

  it('renders feed name in source row', () => {
    render(
      <ArticleListEntry
        articleView={mockArticleView}
        favicon={undefined}
        feedTitle={mockArticleView.feedTitle}
        feedId={mockArticleView.feedId}
        selected={false}
        showPreviews={false}
      />
    );
    expect(screen.getByText('Test Feed')).toBeInTheDocument();
  });

  it('renders article title as link', () => {
    render(
      <ArticleListEntry
        articleView={mockArticleView}
        favicon={undefined}
        feedTitle={mockArticleView.feedTitle}
        feedId={mockArticleView.feedId}
        selected={false}
        showPreviews={false}
      />
    );
    const link = screen.getByRole('link', { name: 'Test Article' });
    expect(link).toHaveAttribute('href', 'https://example.com');
    expect(link).toHaveAttribute('target', '_blank');
  });

  it('calls onSelect when clicked', () => {
    const onSelect = vi.fn();
    render(
      <ArticleListEntry
        articleView={mockArticleView}
        favicon={undefined}
        feedTitle={mockArticleView.feedTitle}
        feedId={mockArticleView.feedId}
        selected={false}
        showPreviews={false}
        onSelect={onSelect}
      />
    );
    fireEvent.click(screen.getByRole('link', { name: 'Test Article' }));
    expect(onSelect).toHaveBeenCalledWith('1');
  });

  it('does not throw when clicked without onSelect', () => {
    render(
      <ArticleListEntry
        articleView={mockArticleView}
        favicon={undefined}
        feedTitle={mockArticleView.feedTitle}
        feedId={mockArticleView.feedId}
        selected={false}
        showPreviews={false}
      />
    );
    expect(() =>
      fireEvent.click(screen.getByRole('link', { name: 'Test Article' }))
    ).not.toThrow();
  });

  it('calls onToggleRead when dot is clicked', () => {
    const onToggleRead = vi.fn();
    render(
      <ArticleListEntry
        articleView={mockArticleView}
        favicon={new FaviconCls('')}
        feedTitle={mockArticleView.feedTitle}
        feedId={mockArticleView.feedId}
        selected={false}
        showPreviews={false}
        onToggleRead={onToggleRead}
      />
    );
    fireEvent.mouseEnter(document.querySelector('.GoliathArticleCard')!);
    const dot = screen.getByTestId('FiberManualRecordIcon');
    fireEvent.click(dot);
    expect(onToggleRead).toHaveBeenCalledWith('1');
  });

  it('does not trigger onSelect when dot is clicked', () => {
    const onSelect = vi.fn();
    const onToggleRead = vi.fn();
    render(
      <ArticleListEntry
        articleView={mockArticleView}
        favicon={new FaviconCls('')}
        feedTitle={mockArticleView.feedTitle}
        feedId={mockArticleView.feedId}
        selected={false}
        showPreviews={false}
        onSelect={onSelect}
        onToggleRead={onToggleRead}
      />
    );
    fireEvent.mouseEnter(document.querySelector('.GoliathArticleCard')!);
    const dot = screen.getByTestId('FiberManualRecordIcon');
    fireEvent.click(dot);
    expect(onToggleRead).toHaveBeenCalledTimes(1);
    expect(onSelect).not.toHaveBeenCalled();
  });

  it('applies selected class when selected', () => {
    const { rerender } = render(
      <ArticleListEntry
        articleView={mockArticleView}
        favicon={undefined}
        feedTitle={mockArticleView.feedTitle}
        feedId={mockArticleView.feedId}
        selected={true}
        showPreviews={false}
      />
    );
    expect(
      document.querySelector('.GoliathArticleCardUnreadSelected')
    ).toBeInTheDocument();

    rerender(
      <ArticleListEntry
        articleView={mockArticleView}
        favicon={undefined}
        feedTitle={mockArticleView.feedTitle}
        feedId={mockArticleView.feedId}
        selected={false}
        showPreviews={false}
      />
    );
    expect(
      document.querySelector('.GoliathArticleCardUnreadSelected')
    ).not.toBeInTheDocument();
  });

  it('applies read + selected class when read and selected', () => {
    render(
      <ArticleListEntry
        articleView={{ ...mockArticleView, isRead: true }}
        favicon={undefined}
        feedTitle={mockArticleView.feedTitle}
        feedId={mockArticleView.feedId}
        selected={true}
        showPreviews={false}
      />
    );
    expect(
      document.querySelector('.GoliathArticleCardReadSelected')
    ).toBeInTheDocument();
  });
});

import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import ArticleListEntry from '../ArticleListEntry';
import { ArticleView } from '../../models/article';
import { FaviconCls } from '../../models/feed';
import { SelectionType } from '../../utils/types';

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
    expect(document.querySelector('.GoliathArticleCardDot')).toHaveClass(
      'hidden'
    );
    expect(document.querySelector('.GoliathArticleCardFavicon')).toHaveClass(
      'visible'
    );
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
    expect(document.querySelector('.GoliathArticleCardDot')).toHaveClass(
      'visible'
    );
    expect(document.querySelector('.GoliathArticleCardFavicon')).toHaveClass(
      'hidden'
    );
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
    expect(document.querySelector('.GoliathArticleCardDot')).toHaveClass(
      'visible'
    );
    expect(document.querySelector('.GoliathArticleCardFavicon')).toHaveClass(
      'hidden'
    );
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
      document.querySelector('.GoliathArticleCardSelected')
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
      document.querySelector('.GoliathArticleCardSelected')
    ).not.toBeInTheDocument();
  });

  it('applies dimmed class when read', () => {
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
      document.querySelector('.GoliathArticleCardDimmed')
    ).toBeInTheDocument();
    expect(
      document.querySelector('.GoliathArticleCardSelected')
    ).toBeInTheDocument();
  });

  it('forces bright class in Saved items stream view even if article is read', () => {
    render(
      <ArticleListEntry
        articleView={{ ...mockArticleView, isRead: true, isSaved: true }}
        favicon={undefined}
        feedTitle={mockArticleView.feedTitle}
        feedId={mockArticleView.feedId}
        selected={false}
        showPreviews={false}
        selectionType={SelectionType.Saved}
      />
    );
    expect(
      document.querySelector('.GoliathArticleCardDimmed')
    ).not.toBeInTheDocument();
    expect(
      document.querySelector('.GoliathArticleCardBright')
    ).toBeInTheDocument();
  });

  it('applies dimmed class when unsaved in Saved items stream view', () => {
    render(
      <ArticleListEntry
        articleView={{ ...mockArticleView, isSaved: false }}
        favicon={undefined}
        feedTitle={mockArticleView.feedTitle}
        feedId={mockArticleView.feedId}
        selected={false}
        showPreviews={false}
        selectionType={SelectionType.Saved}
      />
    );
    expect(
      document.querySelector('.GoliathArticleCardDimmed')
    ).toBeInTheDocument();
  });

  it('does not apply dimmed class when saved in Saved items stream view', () => {
    render(
      <ArticleListEntry
        articleView={{ ...mockArticleView, isSaved: true }}
        favicon={undefined}
        feedTitle={mockArticleView.feedTitle}
        feedId={mockArticleView.feedId}
        selected={false}
        showPreviews={false}
        selectionType={SelectionType.Saved}
      />
    );
    expect(
      document.querySelector('.GoliathArticleCardDimmed')
    ).not.toBeInTheDocument();
  });

  it('renders star border dot on hover for unsaved article in Saved view', () => {
    render(
      <ArticleListEntry
        articleView={{ ...mockArticleView, isSaved: false }}
        favicon={undefined}
        feedTitle={mockArticleView.feedTitle}
        feedId={mockArticleView.feedId}
        selected={false}
        showPreviews={false}
        selectionType={SelectionType.Saved}
      />
    );
    fireEvent.mouseEnter(document.querySelector('.GoliathArticleCard')!);
    expect(screen.getByTestId('StarBorderIcon')).toBeInTheDocument();
  });

  it('renders star dot on hover for saved article in Saved view', () => {
    render(
      <ArticleListEntry
        articleView={{ ...mockArticleView, isSaved: true }}
        favicon={undefined}
        feedTitle={mockArticleView.feedTitle}
        feedId={mockArticleView.feedId}
        selected={false}
        showPreviews={false}
        selectionType={SelectionType.Saved}
      />
    );
    fireEvent.mouseEnter(document.querySelector('.GoliathArticleCard')!);
    expect(screen.getByTestId('StarIcon')).toBeInTheDocument();
  });

  it('calls onToggleSave when star dot is clicked in Saved view', () => {
    const onToggleSave = vi.fn();
    render(
      <ArticleListEntry
        articleView={{ ...mockArticleView, isSaved: true }}
        favicon={new FaviconCls('')}
        feedTitle={mockArticleView.feedTitle}
        feedId={mockArticleView.feedId}
        selected={false}
        showPreviews={false}
        selectionType={SelectionType.Saved}
        onToggleSave={onToggleSave}
      />
    );
    fireEvent.mouseEnter(document.querySelector('.GoliathArticleCard')!);
    const dot = screen.getByTestId('StarIcon');
    fireEvent.click(dot);
    expect(onToggleSave).toHaveBeenCalledWith('1');
  });
});

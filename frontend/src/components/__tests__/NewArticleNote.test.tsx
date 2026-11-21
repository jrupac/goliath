import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import NewArticleNote from '../NewArticleNote';

describe('NewArticleNote', () => {
  it('reports what a refresh brought in, and announces it', () => {
    render(<NewArticleNote count={10} shown={true} />);

    expect(screen.getByText('10 new')).toBeInTheDocument();
    expect(screen.getByRole('status')).toHaveTextContent('10 new articles');
  });

  it('says so when a refresh found nothing', () => {
    render(<NewArticleNote count={0} shown={true} />);

    expect(screen.getByText('Nothing new')).toBeInTheDocument();
    expect(screen.getByRole('status')).toHaveTextContent('No new articles');
  });

  it('keeps the region, and the last text, once there is nothing to say', () => {
    const { rerender } = render(<NewArticleNote count={3} shown={true} />);

    const note = screen.getByRole('status');
    expect(note.className).toContain('GoliathNewArticleNoteVisible');

    rerender(<NewArticleNote count={3} shown={false} />);

    // The region stays so that the next report is announced, and the text
    // stays so that it does not vanish before the element fades.
    expect(screen.getByRole('status')).toBe(note);
    expect(note.className).not.toContain('GoliathNewArticleNoteVisible');
    expect(screen.getByText('3 new')).toBeInTheDocument();
    expect(note).toHaveTextContent('3 new');
    expect(note).not.toHaveTextContent('3 new articles');
  });

  it('says nothing at all before any refresh', () => {
    render(<NewArticleNote count={null} shown={false} />);

    const note = screen.getByRole('status');
    expect(note).toHaveTextContent('');
    expect(note.className).not.toContain('GoliathNewArticleNoteVisible');
  });
});

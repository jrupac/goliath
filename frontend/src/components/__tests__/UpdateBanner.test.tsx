import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { fireEvent } from '@testing-library/react';
import UpdateBanner from '../UpdateBanner';

describe('UpdateBanner', () => {
  it('offers the new build without taking it', () => {
    const onReload = vi.fn();
    render(<UpdateBanner onReload={onReload} onDismiss={vi.fn()} />);

    expect(screen.getByText('A new version is ready')).toBeInTheDocument();
    expect(onReload).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: 'Reload' }));
    expect(onReload).toHaveBeenCalledTimes(1);
  });

  it('dismisses without reloading', () => {
    const onReload = vi.fn();
    const onDismiss = vi.fn();
    render(<UpdateBanner onReload={onReload} onDismiss={onDismiss} />);

    fireEvent.click(
      screen.getByRole('button', { name: 'Dismiss update notice' })
    );

    expect(onDismiss).toHaveBeenCalledTimes(1);
    expect(onReload).not.toHaveBeenCalled();
  });
});

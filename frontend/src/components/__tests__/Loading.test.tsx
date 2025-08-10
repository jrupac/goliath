import React from 'react';
import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import Loading, { LoadingProps } from '../Loading';
import { Status } from '../../utils/types';

const statusToString = (status: Status): string => {
  const statusRatio =
    status / (Status.Folder | Status.Feed | Status.Article | Status.Favicon);
  return Math.round(100 * statusRatio).toString();
};

describe('Loading', () => {
  it('renders the correct progress for single Status', () => {
    const status = Status.Folder;
    const props: LoadingProps = {
      status: status,
    };

    render(<Loading {...props} />);

    const progressBar = screen.getByTestId('loading-progress');
    expect(progressBar).toBeInTheDocument();
    expect(progressBar).toHaveAttribute(
      'aria-valuenow',
      statusToString(status)
    );
  });
  it('renders the correct progress for mixed Status', () => {
    const status = Status.Folder | Status.Article;
    const props: LoadingProps = {
      status: status,
    };

    render(<Loading {...props} />);

    const progressBar = screen.getByTestId('loading-progress');
    expect(progressBar).toBeInTheDocument();
    expect(progressBar).toHaveAttribute(
      'aria-valuenow',
      statusToString(status)
    );
  });

  it('renders without crashing', () => {
    render(<Loading status={0} />);
  });
});

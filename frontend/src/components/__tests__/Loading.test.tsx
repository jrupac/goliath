import React from 'react';
import { render } from '@testing-library/react';
import Loading from '../Loading';

describe('Loading', () => {
  it('renders', () => {
    // Minimal rendering test
    render(<Loading />);
  });
});
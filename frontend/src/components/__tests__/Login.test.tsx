import React from 'react';
import { render } from '@testing-library/react';
import Login from '../Login';

describe('Login', () => {
  it('renders', () => {
    // Minimal rendering test
    render(<Login />);
  });
});
import React from 'react';
import {render} from '@testing-library/react';
import {describe, it} from 'vitest';
import Login from '../Login';

describe('Login', () => {
  it('renders', () => {
    // Minimal rendering test
    render(<Login/>);
  });
});
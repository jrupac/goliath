import React from 'react';
import { render } from '@testing-library/react';
import FolderFeedList from '../FolderFeedList';

describe('FolderFeedList', () => {
  it('renders', () => {
    // Minimal rendering test
    const props = {
      folders: [],
      selectedFeedId: null,
      onSelectFeed: () => {},
    };
    render(<FolderFeedList {...props} />);
  });
});
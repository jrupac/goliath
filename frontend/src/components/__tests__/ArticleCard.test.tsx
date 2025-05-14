import React from 'react';
import {describe, it} from 'vitest';
import {render} from '@testing-library/react';
import ArticleCard from '../ArticleCard';
import {ArticleView} from "../../models/article";

describe('ArticleCard', () => {
  it('renders', () => {
    // Minimal rendering test
    const props = {
      article: {
        folderId: "1",
        feedId: "1",
        feedTitle: 'Test Feed',
        favicon: '',
        id: '1',
        title: 'Test Article',
        author: '',
        html: '<p>Test content</p>',
        url: 'https://example.com',
        creationTime: 1678886400, // March 15, 2023
        isRead: false,
        isSaved: false,
      } as ArticleView,
      title: 'Test Feed',
      favicon: '',
      isSelected: false,
    };
    render(<ArticleCard {...props} />);
  });
});
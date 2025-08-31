import React, { memo, useEffect, useRef, useState } from 'react';
import { ArticleView, ArticleId } from '../models/article';
import { getPreviewImage } from '../utils/helpers';
import { Skeleton } from '@mui/material';
import { ArticleImagePreview } from '../utils/types';

interface ImagePreviewProps {
  article: ArticleView;
}

// Module-level cache for image preview promises.
// This ensures we only fetch the preview for each article once.
// Exported for testing purposes.
export const previewCache = new Map<
  ArticleId,
  Promise<ArticleImagePreview | undefined>
>();

function getCachedPreview(
  article: ArticleView
): Promise<ArticleImagePreview | undefined> {
  if (previewCache.has(article.id)) {
    return previewCache.get(article.id)!;
  }

  const promise = getPreviewImage(article);
  previewCache.set(article.id, promise);
  return promise;
}

// Custom hook to use IntersectionObserver
const useOnScreen = (ref: React.RefObject<HTMLElement>) => {
  const [isIntersecting, setIntersecting] = useState(false);

  useEffect(() => {
    const observer = new IntersectionObserver(([entry]) => {
      if (entry.isIntersecting) {
        setIntersecting(true);
        if (ref.current) {
          observer.unobserve(ref.current);
        }
      }
    });

    if (ref.current) {
      observer.observe(ref.current);
    }

    return () => {
      if (ref.current) {
        // eslint-disable-next-line react-hooks/exhaustive-deps
        observer.unobserve(ref.current);
      }
    };
  }, [ref]);

  return isIntersecting;
};

const ImagePreview: React.FC<ImagePreviewProps> = ({ article }) => {
  const ref = useRef<HTMLDivElement>(null);
  const onScreen = useOnScreen(ref);
  const [imgSrc, setImgSrc] = useState<string | null | undefined>(undefined);

  useEffect(() => {
    // Only fetch if the component is on screen and we haven't already fetched.
    if (onScreen && imgSrc === undefined) {
      getCachedPreview(article).then((preview) => {
        setImgSrc(preview ? preview.src : null);
      });
    }
  }, [onScreen, imgSrc, article]);

  // If loading has failed (imgSrc is null), render nothing.
  if (imgSrc === null) {
    return null;
  }

  // The outer div is what the IntersectionObserver will watch.
  // It needs a fixed size and margin to prevent layout shifts.
  return (
    <div
      ref={ref}
      style={{ width: '100px', height: '100px', marginRight: '10px' }}
    >
      {imgSrc === undefined && (
        // Still loading and not on screen
        <Skeleton
          data-testid="image-preview-skeleton"
          variant="rectangular"
          width={100}
          height={100}
          animation="wave"
          sx={{ borderRadius: '10px' }}
        />
      )}

      {imgSrc && (
        // Success
        <figure className="GoliathArticleListImagePreviewFigure">
          <img
            className="GoliathArticleListImagePreview"
            src={imgSrc}
            alt={`Preview for ${article.title}`}
          />
        </figure>
      )}
    </div>
  );
};

export default memo(ImagePreview);

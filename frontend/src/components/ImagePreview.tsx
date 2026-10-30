import React, { memo, useEffect, useRef, useState } from 'react';
import { ArticleView, ArticleId } from '../models/article';
import { getPreviewImage } from '../utils/helpers';
import { Skeleton } from '@mui/material';
import { ArticleImagePreview } from '../utils/types';

interface ImagePreviewProps {
  article: ArticleView;
}

// Module-level LRU cache for image preview promises (capped at 1000 entries).
// Exported for testing purposes.
export const previewCache = new Map<
  ArticleId,
  Promise<ArticleImagePreview | undefined>
>();

const MAX_PREVIEW_CACHE_SIZE = 1000;

function getCachedPreview(
  article: ArticleView
): Promise<ArticleImagePreview | undefined> {
  if (previewCache.has(article.id)) {
    // Move to end to maintain LRU order.
    const existing = previewCache.get(article.id)!;
    previewCache.delete(article.id);
    previewCache.set(article.id, existing);
    return existing;
  }

  if (previewCache.size >= MAX_PREVIEW_CACHE_SIZE) {
    const oldestKey = previewCache.keys().next().value as ArticleId;
    previewCache.delete(oldestKey);
  }

  const promise = getPreviewImage(article);
  previewCache.set(article.id, promise);
  return promise;
}

// How long a row has to stay on screen before its image is worth fetching.
// Short enough to be invisible to a reader who has stopped, long enough to
// cover the rows a flick throws past.
const SETTLE_MS = 150;

// Reports whether the element has come to rest on screen, rather than merely
// crossed it.
//
// Finding an image means downloading one to measure it, which is the most
// expensive thing a row asks for. Committing the moment a row touches the
// viewport means a scroll pays that for every row it passes, all of it
// competing with the scrolling itself and almost all of it discarded unseen.
// Waiting for the row to still be there a moment later spends the work only on
// rows the reader actually arrived at.
const useOnScreen = (ref: React.RefObject<HTMLElement | null>) => {
  const [settled, setSettled] = useState(false);

  useEffect(() => {
    const element = ref.current;
    if (!element) {
      return;
    }

    let timer: ReturnType<typeof setTimeout> | undefined;
    const observer = new IntersectionObserver(([entry]) => {
      if (entry.isIntersecting) {
        timer = setTimeout(() => {
          timer = undefined;
          setSettled(true);
          observer.unobserve(element);
        }, SETTLE_MS);
      } else if (timer !== undefined) {
        clearTimeout(timer);
        timer = undefined;
      }
    });
    observer.observe(element);

    return () => {
      if (timer !== undefined) {
        clearTimeout(timer);
      }
      observer.unobserve(element);
    };
  }, [ref]);

  return settled;
};

const ImagePreview = ({ article }: ImagePreviewProps) => {
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
    <div ref={ref} className="GoliathArticleListImagePreviewBox">
      {onScreen && imgSrc === undefined && (
        // On screen and still loading
        <Skeleton
          data-testid="image-preview-skeleton"
          variant="rectangular"
          width={110}
          height={110}
          animation="wave"
          className="GoliathImagePreviewSkeleton"
        />
      )}

      {imgSrc && (
        // Success
        <figure className="GoliathArticleListImagePreviewFigure">
          <img
            className="GoliathArticleListImagePreview"
            src={imgSrc}
            alt={`Preview for ${article.title}`}
            // The box this is drawn in is a fixed size in CSS, so the
            // dimensions are only here to let the decode happen off the
            // critical path: a row that appears mid-scroll must not hold up
            // the frame while a full-resolution photograph is decoded.
            decoding="async"
            width={110}
            height={110}
          />
        </figure>
      )}
    </div>
  );
};

export default memo(ImagePreview);

import moment from "moment";
import {Decimal} from "decimal.js-light";
import {Readability} from "@mozilla/readability";
import * as LosslessJSON from "lossless-json";

import {ArticleId, ArticleView} from "../models/article";
import {ArticleImagePreview, GoliathTheme, ThemeInfo} from "./types";
import {createTheme, darkScrollbar, PaletteMode, Theme} from "@mui/material";
import smartcrop from "smartcrop";

export function extractText(html: string): string | null {
  return new DOMParser()
    .parseFromString(html, "text/html")
    .documentElement.textContent;
}

export function makeAbsolute(url: string): string {
  const a = document.createElement('a');
  a.href = url;
  return a.href;
}

export function formatFull(date: Date) {
  return moment(date).format('dddd, MMMM Do YYYY, h:mm:ss A');
}

export function formatFriendly(date: Date) {
  const now = moment();
  const before = moment(now).subtract(1, 'days');
  const then = moment(date);
  if (then.isBetween(before, now)) {
    return then.fromNow();
  } else {
    return then.format("ddd, MMM D, h:mm A");
  }
}

export function maxDecimal(a: Decimal | ArticleId, b: Decimal | ArticleId): Decimal {
  a = new Decimal(a.toString());
  b = new Decimal(b.toString());
  return a.greaterThan(b) ? a : b;
}

export function fetchReadability(url: string): Promise<string> {
  return new Promise((resolve, reject) => {
    fetch(url)
      .then((response) => response.text())
      .then((result) => {
        const doc = new DOMParser().parseFromString(result, "text/html");
        const parsed = new Readability(doc).parse();

        if (!parsed || !parsed.content) {
          reject(new Error("Could not parse document"));
        } else {
          resolve(parsed.content);
        }
      })
      .catch(reject);
  })
}

export function parseJson(text: string): any {
  // Parse as Lossless numbers since values from the server are 64-bit
  // Integer, but then convert back to String for use going forward.
  return LosslessJSON.parse(text, (_: string, v: any) => {
    if (v && v.isLosslessNumber) {
      return String(v);
    }
    return v;
  });
}

export function cookieExists(name: string): boolean {
  const cookies: string[] = document.cookie.split(";");
  return cookies.some((cookie: string) => cookie.trim().startsWith(`${name}=`));
}

export function populateThemeInfo(themeSetting: GoliathTheme): ThemeInfo {
  let themeClasses: string, paletteMode: PaletteMode;

  if (themeSetting === GoliathTheme.Default) {
    themeClasses = 'default-theme';
    paletteMode = 'light';
  } else {
    themeClasses = 'dark-theme';
    paletteMode = 'dark';
  }

  const theme: Theme = createTheme({
    palette: {
      mode: paletteMode,
    },
    components: {
      MuiCssBaseline: {
        styleOverrides: {
          body: paletteMode === 'dark' ? darkScrollbar() : null,
        },
      },
    },
  });
  return {themeClasses, theme};
}

export async function getPreviewImage(article: ArticleView): Promise<ArticleImagePreview | undefined> {
  const minPixelSize = 100;
  const imgFetchLimit = 5;

  type ProcessedImageCandidate = {
    src: string;
    bitmap: ImageBitmap;
    origWidth: number;
    origHeight: number;
  };

  let imageElements: HTMLCollectionOf<HTMLImageElement>;
  try {
    imageElements = new DOMParser().parseFromString(
      article.html, "text/html").images;
  } catch (e) {
    console.error("Error parsing article HTML for preview:", e);
    return;
  }

  if (!imageElements || imageElements.length === 0) {
    return;
  }

  const imageProcessingPromises: Promise<ProcessedImageCandidate | null>[] = [];
  const limit = Math.min(imgFetchLimit, imageElements.length);

  for (let i = 0; i < limit; i++) {
    const imgSrc = imageElements[i].src;

    if (!imgSrc ||
      (!imgSrc.startsWith('http') && !imgSrc.startsWith('data:'))) {
      continue;
    }

    imageProcessingPromises.push(
      fetch(imgSrc)
        .then(response => {
          if (!response.ok) {
            throw new Error(`Failed to fetch ${imgSrc}: ${response.statusText}`);
          }
          return response.blob();
        })
        .then(createImageBitmap)
        .then(bitmap => {
          if (bitmap.width >= minPixelSize && bitmap.height >= minPixelSize) {
            return {
              src: imgSrc,
              bitmap,
              origWidth: bitmap.width,
              origHeight: bitmap.height
            };
          } else {
            bitmap.close();
            return null;
          }
        })
        .catch(_ => {
          return null;
        })
    );
  }

  const settledResults = await Promise.allSettled(imageProcessingPromises);
  let imageCandidate: ProcessedImageCandidate | null = null;

  settledResults.forEach(result => {
    if (result.status === 'fulfilled' && result.value) {
      const currentImage = result.value;
      if (currentImage) {
        if (
          !imageCandidate ||
          currentImage.origHeight > imageCandidate.origHeight ||
          currentImage.origWidth > imageCandidate.origWidth
        ) {
          if (imageCandidate) {
            imageCandidate.bitmap.close();
          }
          imageCandidate = currentImage;
        } else {
          currentImage.bitmap.close();
        }
      }
    }
  });

  if (!imageCandidate) {
    return;
  }

  const preview: ProcessedImageCandidate = imageCandidate;
  let cropResult = null;

  try {
    cropResult = await smartcrop.crop(preview.bitmap, {
      minScale: 0.001, // Allow very small scales if necessary
      height: minPixelSize,
      width: minPixelSize,
      ruleOfThirds: false
    });
  }
  finally {
    preview.bitmap.close();
  }

  let imgPreview: ArticleImagePreview;

  if (cropResult && cropResult.topCrop) {
    imgPreview = {
      src: preview.src,
      x: cropResult.topCrop.x,
      y: cropResult.topCrop.y,
      origWidth: preview.origWidth,
      width: cropResult.topCrop.width,
      height: cropResult.topCrop.height,
    };
  } else {
    // Fallback: use original image dimensions (will be scaled by CSS if needed)
    imgPreview = {
      src: preview.src,
      x: 0,
      y: 0,
      origWidth: preview.origWidth,
      width: preview.origWidth,
      height: preview.origHeight,
    };
  }

  return imgPreview;
}
import moment from "moment";
import {Decimal} from "decimal.js-light";
import {Readability} from "@mozilla/readability";
import * as LosslessJSON from "lossless-json";

import {ArticleId} from "../models/article";
import {GoliathTheme, ThemeInfo} from "./types";
import {createTheme, darkScrollbar, PaletteMode, Theme} from "@mui/material";

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

        if (!parsed || parsed.content === null) {
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
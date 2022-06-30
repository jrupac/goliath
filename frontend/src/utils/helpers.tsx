import moment from "moment";
import {Decimal} from "decimal.js-light";
import {ArticleId} from "./types";
import {Readability} from "@mozilla/readability";

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
  return a > b ? a : b;
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
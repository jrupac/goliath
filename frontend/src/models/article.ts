import {MarkState} from "../utils/types";

/** ArticleId is the ID associated with an Article object. */
export type ArticleId = string;

export interface ArticleView {
  id: ArticleId;
  title: string;
  author: string;
  html: string;
  url: string;
  is_read: 0 | 1;
  created_on_time: number;
}

export enum ReadStatus {
  Read = 0,
  Unread = 1
}

export class ArticleCls {
  private readonly id: ArticleId;
  private readonly title: string;
  private readonly author: string;
  private readonly html: string;
  private readonly url: string;
  private readonly is_saved: 0 | 1;
  private readonly created_on_time: number;
  private readStatus: ReadStatus;

  constructor(id: ArticleId, title: string, author: string, html: string,
    url: string, is_saved: 0 | 1, created_on_time: number,
    is_read: 0 | 1) {
    this.id = id;
    this.title = title;
    this.author = author;
    this.html = html;
    this.url = url;
    this.is_saved = is_saved;
    this.created_on_time = created_on_time;
    this.readStatus = (is_read === 1) ? ReadStatus.Read : ReadStatus.Unread;
  }

  public MarkArticle(markState: MarkState): ReadStatus {
    if (markState === "read") {
      this.readStatus = ReadStatus.Read;
    }

    return this.readStatus;
  }

  public GetArticleEntry(): ArticleView {
    return {
      id: this.id,
      title: this.title,
      author: this.author,
      html: this.html,
      url: this.url,
      is_read: (this.readStatus === ReadStatus.Read) ? 1 : 0,
      created_on_time: this.created_on_time,
    };
  }

  public ReadStatus(): ReadStatus {
    return this.readStatus;
  }

  public Id(): ArticleId {
    return this.id;
  }

  public CreateTime(): number {
    return this.created_on_time;
  }

  public static Comparator(a: ArticleCls, b: ArticleCls): number {
    // Sort by article creation time in descending order
    return b.CreateTime() - a.CreateTime();
  }
}
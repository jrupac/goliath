declare module 'lossless-json' {
  export function parse(text: string, reviver?: (key: string, value: any) => object): object;
}
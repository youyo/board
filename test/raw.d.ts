// vite の `?raw` import（テストの入力ファイルを文字列として読む）。
declare module "*?raw" {
  const content: string;
  export default content;
}

// vite の `import.meta.glob`(ファイルの存在確認に使う。中身は読まない)。vite の型を直接は入れていないので必要な分だけ宣言する。
interface ImportMeta {
  glob(
    patterns: string | string[],
    options?: { query?: string; import?: string; eager?: boolean },
  ): Record<string, () => Promise<unknown>>;
}

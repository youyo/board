import { describe, expect, it } from "vitest";
import { foldContains, nameFold } from "../../src/cache/fold";

// Go 版 internal/cache/fold/fold_test.go を転記。BOARD の name_cont(Ransack)の実測に近づけた正規化。
describe("nameFold (NFKC + 小文字 + trim)", () => {
  const cases: Array<[string, string]> = [
    ["COI", "coi"],
    ["coi", "coi"],
    ["ＣＯＩ", "coi"], // 全角 ASCII → 半角 + 小文字
    ["  COI  ", "coi"], // trim
    ["株式会社", "株式会社"], // 漢字はそのまま
    ["カイシャ", "カイシャ"], // カナはそのまま
    ["ｶﾌﾞｼｷ", "カブシキ"], // 半角カナ → 全角カナ
    ["", ""],
    [" \t\n ", ""],
  ];
  for (const [input, want] of cases) {
    it(`nameFold(${JSON.stringify(input)}) = ${JSON.stringify(want)}`, () => {
      expect(nameFold(input)).toBe(want);
    });
  }

  it("全角スペース(U+3000)だけの文字列も空になる", () => {
    expect(nameFold("　　")).toBe("");
  });
});

describe("foldContains", () => {
  const cases: Array<[string, string, boolean]> = [
    // ASCII 大文字小文字
    ["弘前大学ＣＯＩ研究推進機構", "coi", true],
    ["弘前大学ＣＯＩ研究推進機構", "COI", true],
    ["弘前大学ＣＯＩ研究推進機構", "ＣＯＩ", true],
    // trim
    ["弘前大学ＣＯＩ研究推進機構", " coi ", true],
    // 漢字はそのまま部分一致
    ["株式会社A", "株", true],
    ["株式会社A", "会社", true],
    // 漢字 ↔ カナ の fold はしない
    ["会社", "カイシャ", false],
    // 空の needle は常に true
    ["any", "", true],
    ["any", "   ", true],
    // 半角 ↔ 全角カナ
    ["カブシキ", "ｶﾌﾞｼｷ", true],
    ["ｶﾌﾞｼｷ", "カブシキ", true],
    // 不一致
    ["abc", "xyz", false],
  ];
  for (const [haystack, needle, want] of cases) {
    it(`foldContains(${JSON.stringify(haystack)}, ${JSON.stringify(needle)}) = ${want}`, () => {
      expect(foldContains(haystack, needle)).toBe(want);
    });
  }

  it("haystack が null / undefined なら needle があるときは false、空 needle なら true", () => {
    expect(foldContains(null, "a")).toBe(false);
    expect(foldContains(undefined, "a")).toBe(false);
    expect(foldContains(null, "")).toBe(true);
  });
});

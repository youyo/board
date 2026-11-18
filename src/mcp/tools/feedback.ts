import { z } from "zod";
import { buildFeedbackGuide, FEEDBACK_DESCRIPTION, FEEDBACK_KINDS } from "../../feedback/guide";
import { defineTool } from "../registry";

// feedback ツール(ADR 0007。共通仕様は logbutler の ADR 0019)。上流の公開リポジトリ youyo/board に issue を起票するための案内を返す。
// issue は作らず、BOARD にも GitHub にも通信しない。応答は kind だけで決まり、ctx は使わない。

export const feedback = defineTool({
  // <製品>_feedback の形で固定(接頭辞を付けない規約の唯一の例外。ADR 0007)
  name: "board_feedback",
  title: "フィードバック（GitHub issue の起票案内）",
  // 開始条件・中止条件・承認の固定文を含む(src/feedback/guide.ts。steps と同じ文言)
  description: FEEDBACK_DESCRIPTION,
  category: "ReadOnly",
  // 外部と通信しない(annotation の openWorldHint=false)
  openWorld: false,
  // 応答は数 KB の固定の案内。max_response_tokens は付けない(入力は kind だけ。ADR 0007)
  responseLimit: false,
  inputSchema: z.object({
    kind: z
      .enum(FEEDBACK_KINDS)
      .describe("Type of feedback: bug, feature, question or other (default: bug)")
      .optional(),
  }),
  handler: async ({ kind }) => buildFeedbackGuide(kind ?? "bug"),
});

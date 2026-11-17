import type { LanguageModelV4Usage } from '@ai-sdk/provider';

export interface OpenAnyUsage {
  prompt_tokens?: number | null;
  completion_tokens?: number | null;
  total_tokens?: number | null;
  prompt_tokens_details?: {
    cached_tokens?: number | null;
  } | null;
  completion_tokens_details?: {
    reasoning_tokens?: number | null;
  } | null;
}

function finite(value: number | null | undefined): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

/**
 * Map a Chat Completions usage object. Counts that the response omits stay
 * undefined. This function does not substitute zeros.
 */
export function convertOpenAnyUsage(usage: OpenAnyUsage | null | undefined): LanguageModelV4Usage {
  if (usage == null) {
    return {
      inputTokens: {
        total: undefined,
        noCache: undefined,
        cacheRead: undefined,
        cacheWrite: undefined,
      },
      outputTokens: {
        total: undefined,
        text: undefined,
        reasoning: undefined,
      },
    };
  }
  const prompt = finite(usage.prompt_tokens);
  const completion = finite(usage.completion_tokens);
  const cached = finite(usage.prompt_tokens_details?.cached_tokens);
  const reasoning = finite(usage.completion_tokens_details?.reasoning_tokens);
  return {
    inputTokens: {
      total: prompt,
      noCache: prompt != null && cached != null ? prompt - cached : undefined,
      cacheRead: cached,
      cacheWrite: undefined,
    },
    outputTokens: {
      total: completion,
      text: completion != null && reasoning != null ? Math.max(0, completion - reasoning) : undefined,
      reasoning,
    },
    raw: usage as LanguageModelV4Usage['raw'],
  };
}

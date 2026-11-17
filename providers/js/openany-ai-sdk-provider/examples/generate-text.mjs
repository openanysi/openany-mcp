/**
 * Text Chat Completions through the named OpenAny provider.
 * This example is not executed by the test suite and does not contain a key.
 *
 *   export OPENANY_API_KEY=...   # raw key, not a Bearer header
 *   node examples/generate-text.mjs
 *
 * Set OPENANY_MODEL to a model id copied from the current listing.
 * No live call is part of package verification.
 */
import { generateText } from 'ai';
import { createOpenAny } from 'openany-ai-sdk-provider';

const modelId = process.env.OPENANY_MODEL;
if (!modelId) {
  console.error('Set OPENANY_MODEL to a model id from the current OpenAny listing.');
  process.exit(1);
}

const openany = createOpenAny();
const { text, usage, finishReason } = await generateText({
  model: openany(modelId),
  prompt: 'Reply with exactly: OpenAny connected',
  maxOutputTokens: 32,
});

console.log({ text, finishReason, usage });

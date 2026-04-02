const fs = require('fs');
const path = require('path');
const crypto = require('crypto');

const baseUrl = process.env.BIFROST_URL || 'http://localhost:8080';
const defaultImagePath = path.join(__dirname, '..', 'asserts', 'cat.jpg');
const imagePath =
  process.env.IMAGE_PATH || (fs.existsSync(defaultImagePath) ? defaultImagePath : '');
const truthy = (v) => ['1', 'true', 'yes', 'on'].includes(String(v || '').toLowerCase());
const hasCliTestName =
  process.argv.includes('-t') || process.argv.includes('--testNamePattern');
const shouldRun = truthy(process.env.E2E) || truthy(process.env.JEST_E2E) || hasCliTestName;
const maxTokens = Number(process.env.MAX_TOKENS || 80);
const stopSeq = process.env.STOP_SEQ || '';

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

/**
 * Builds OpenAI-style message array.
 * Supports text and vision modalities.
 */
const buildMessages = (prompt, modality) => {
  if (modality !== 'vision') {
    return [{ role: 'user', content: prompt }];
  }
  if (!imagePath) {
    throw new Error('MODALITY=vision requires IMAGE_PATH');
  }
  const img = fs.readFileSync(imagePath);
  const b64 = img.toString('base64');
  return [
    {
      role: 'user',
      content: [
        { type: 'text', text: prompt },
        { type: 'image_url', image_url: { url: `data:image/png;base64,${b64}` } }
      ]
    }
  ];
};

/**
 * Sends a chat completion request to Bifrost.
 */
const sendRequest = async ({ tier, reasoning, modality, prompt, metadata = {} }) => {
  const id = crypto.randomUUID();
  // For easy log discovery, we can append ID to prompt if prompt is string
  const taggedPrompt = typeof prompt === 'string' ? `${prompt} [test_id:${id}]` : prompt;
  
  const payload = {
    model: tier,
    messages: buildMessages(taggedPrompt, modality),
    max_tokens: maxTokens,
    max_completion_tokens: maxTokens,
    ...(stopSeq ? { stop: [stopSeq] } : {}),
    metadata: { ...metadata, test_id: id }
  };

  const res = await fetch(`${baseUrl}/v1/chat/completions`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'x-tier': tier,
      'x-reasoning': reasoning,
      'x-modality': modality
    },
    body: JSON.stringify(payload)
  });

  const text = await res.text();
  let json;
  try {
    json = JSON.parse(text);
  } catch {
    throw new Error(`Non-JSON response: ${text.slice(0, 500)}`);
  }

  if (!res.ok) {
    throw new Error(`Request failed: ${res.status} ${JSON.stringify(json)}`);
  }

  return { id, response: json, status: res.status };
};

/**
 * Polling to find the log entry for a specific test ID.
 */
const hasTestId = (content, testId) => {
  const needle = `test_id:${testId}`;
  if (typeof content === 'string') {
    return content.includes(needle);
  }
  if (Array.isArray(content)) {
    return content.some((part) => typeof part?.text === 'string' && part.text.includes(needle));
  }
  if (content && typeof content === 'object' && typeof content.text === 'string') {
    return content.text.includes(needle);
  }
  return false;
};

const findLogByTestId = async (testId) => {
  const maxAttempts = 10;
  for (let i = 0; i < maxAttempts; i += 1) {
    const res = await fetch(`${baseUrl}/api/logs?limit=50&offset=0&sort_by=timestamp&order=desc`);
    const json = await res.json();
    const logs = json.logs || [];
    const match = logs.find((l) => {
      if (l?.metadata?.test_id === testId) {
        return true;
      }
      const history = l.input_history || [];
      return history.some((h) => hasTestId(h.content, testId));
    });
    if (match) {
      return match;
    }
    await sleep(1000);
  }
  throw new Error(`Log not found for test_id:${testId}`);
};

module.exports = {
  baseUrl,
  imagePath,
  shouldRun,
  sleep,
  buildMessages,
  sendRequest,
  findLogByTestId
};

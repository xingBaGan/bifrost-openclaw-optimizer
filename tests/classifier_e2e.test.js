const { shouldRun, imagePath, sendRequest, findLogByTestId } = require('./e2e_helper');

describe('Classifier Plugin (multi-dimensional scoring)', () => {
  const maybe = shouldRun ? test : test.skip;

  // --- Modality Detection ---

  // const visionTest = imagePath ? maybe : test.skip;
  // visionTest('vision: image in message → vision modality', async () => {
  //   const { id } = await sendRequest({
  //     prompt: 'Describe this image',
  //     modality: 'vision'
  const visionTest = imagePath ? maybe : test.skip;
  visionTest('vision: image in message → vision modality', async () => {
    const { id } = await sendRequest({
      prompt: 'Describe this image',
      modality: 'vision'
    });
    const log = await findLogByTestId(id);
    expect(log.routing_rule_name).toBe('vision-quality-fast');
    expect(['kimi', 'openai']).toContain(log.provider);
  }, 40000);

  // --- Code Classification ---

  maybe('code-weak: single keyword (func) → quality/fast', async () => {
    const { id } = await sendRequest({
      prompt: 'Can you finish this code? func main() { }'
    });
    const log = await findLogByTestId(id);
    // 1 code keyword → tierScore=1 → quality, no reason keywords → fast
    expect(log.routing_rule_name).toBe('text-quality-fast');
    expect(['kimi', 'openai']).toContain(log.provider);
  }, 40000);

  maybe('code-strong: 3+ keywords → quality/think (strong coding signal)', async () => {
    const { id } = await sendRequest({
      prompt: 'Debug this:\nimport os\ndef main():\n  return os.path.join("a", "b")'
    });
    const log = await findLogByTestId(id);
    // 3+ code keywords (import, def, return) → tierScore=2(+1 sys) -> 3. reasonScore=1 → quality/think
    expect(log.routing_rule_name).toBe('text-quality-think');
    expect(['kimi', 'openai']).toContain(log.provider);
  }, 40000);

  // --- Reasoning Classification ---

  maybe('reasoning-weak: 1 keyword → no tier/reason boost', async () => {
    const { id } = await sendRequest({
      prompt: 'Can you analyze this data?'
    });
    const log = await findLogByTestId(id);
    // 1 reason keyword (analyze) → reasonHits=1 < threshold(2). No boost.
    // tierScore=0 → economy, reasonScore=0 → fast
    expect(log.routing_rule_name).toBe('text-economy-fast');
    expect(log.provider).toBe('deepseek');
  }, 40000);

  maybe('reasoning-strong: 2+ keywords → quality/think', async () => {
    const { id } = await sendRequest({
      prompt: 'Please analyze this problem and explain why step by step'
    });
    const log = await findLogByTestId(id);
    // 2+ reason keywords (analyze, step by step) → tierScore+1, reasonScore+1
    expect(log.routing_rule_name).toBe('text-quality-think');
    expect(['kimi', 'openai']).toContain(log.provider);
  }, 40000);

  // --- System Prompt Influence ---

  maybe('system-research: research keywords in system prompt → research tier', async () => {
    const { id } = await sendRequest({
      prompt: 'Tell me about quantum mechanics',
      systemPrompt: 'You are an expert academic researcher. Provide comprehensive peer-reviewed analysis.'
    });
    const log = await findLogByTestId(id);
    // system: expert(+2) + academic(+2) + research(+2) + comprehensive(+2) + peer-reviewed(+2) = tierScore=10 → research
    expect(log.routing_rule_name).toBe('text-research-fast');
  }, 40000);

  maybe('system-coding: coding keywords in system prompt → quality tier', async () => {
    const { id } = await sendRequest({
      prompt: 'How does React work?',
      systemPrompt: 'You are a senior software engineer. Help with programming and debug issues.'
    });
    const log = await findLogByTestId(id);
    // system: engineer(+1) + programming(+1) + debug(+1) = tierScore=3 → quality
    expect(log.routing_rule_name).toBe('text-quality-fast');
  }, 40000);

  // --- Default / Simple Chat ---

  maybe('default: plain greeting → economy/fast', async () => {
    const { id } = await sendRequest({
      prompt: 'Hello, how are you?'
    });
    const log = await findLogByTestId(id);
    expect(log.routing_rule_name).toBe('text-economy-fast');
    expect(log.provider).toBe('deepseek');
  }, 40000);

  // --- Header Override ---

  maybe('override: explicit headers bypass classifier scoring', async () => {
    const { id } = await sendRequest({
      prompt: 'Just a simple hello',
      overrideHeaders: {
        'x-route-modality': 'text',
        'x-route-tier': 'research',
        'x-route-reasoning': 'think'
      }
    });
    const log = await findLogByTestId(id);
    // Explicit override should force research/think regardless of content
    expect(log.routing_rule_name).toBe('text-research-think');
  }, 40000);
});

const { shouldRun, imagePath, sendRequest, findLogByTestId } = require('./e2e_helper');

describe('Routing rules (representative requests)', () => {
  const maybe = shouldRun ? test : test.skip;

  // maybe('economy + fast + text -> DeepSeek', async () => {
  //   const { id } = await sendRequest({
  //     tier: 'economy',
  //     reasoning: 'fast',
  //     modality: 'text',
  //     prompt: 'routing check economy fast'
  //   });

  //   const log = await findLogByTestId(id);
  //   expect(log.routing_rule_name).toBe('text-economy-fast');
  //   expect(log.provider).toBe('deepseek');
  //   expect(log.model).toBe('deepseek-chat');
  // }, 30000);

  // maybe('quality + think + text -> Kimi (or OpenAI fallback)', async () => {
  //   const { id } = await sendRequest({
  //     tier: 'quality',
  //     reasoning: 'think',
  //     modality: 'text',
  //     prompt: 'routing check quality think'
  //   });

  //   const log = await findLogByTestId(id);
  //   expect(log.routing_rule_name).toBe('text-quality-think');
  //   expect(['kimi', 'openai']).toContain(log.provider);
  // }, 30000);

  // maybe('research + fast + text -> OpenRouter Grok (or fallback)', async () => {
  //   const { id } = await sendRequest({
  //     tier: 'research',
  //     reasoning: 'fast',
  //     modality: 'text',
  //     prompt: 'routing check research fast'
  //   });

  //   const log = await findLogByTestId(id);
  //   expect(log.routing_rule_name).toBe('text-research-fast');
  //   expect(log.provider).toBe('openrouter');
  // }, 30000);

  const visionTest = imagePath ? maybe : test.skip;
  visionTest('quality + fast + vision -> Kimi (or OpenAI fallback)', async () => {
    const { id } = await sendRequest({
      tier: 'quality',
      reasoning: 'fast',
      modality: 'vision',
      prompt: 'routing check vision quality fast'
    });

    const log = await findLogByTestId(id);
    expect(log.routing_rule_name).toBe('vision-quality-fast');
    expect(['kimi', 'openai']).toContain(log.provider);
  }, 30000);
});

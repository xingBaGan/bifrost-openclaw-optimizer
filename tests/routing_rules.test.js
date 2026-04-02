const fs = require('fs');
const path = require('path');

const configPath = path.join(__dirname, '..', 'config.json');

const loadRules = () => {
  const raw = fs.readFileSync(configPath, 'utf8');
  const json = JSON.parse(raw);
  return json.governance?.routing_rules || [];
};

describe('Bifrost routing rules (basic coverage)', () => {
  const rules = loadRules();

  test('has 10 routing rules', () => {
    expect(rules.length).toBe(10);
  });

  test('all rules have unique ids', () => {
    const ids = rules.map(r => r.id);
    const unique = new Set(ids);
    expect(ids.every(Boolean)).toBe(true);
    expect(unique.size).toBe(ids.length);
  });

  test('priorities are ascending as expected', () => {
    const priorities = rules.map(r => r.priority);
    const sorted = [...priorities].sort((a, b) => a - b);
    expect(priorities).toEqual(sorted);
  });

  test('vision rules come before text rules', () => {
    const visionPriorities = rules
      .filter(r => r.name.startsWith('vision-'))
      .map(r => r.priority);
    const textPriorities = rules
      .filter(r => r.name.startsWith('text-'))
      .map(r => r.priority);

    expect(Math.max(...visionPriorities)).toBeLessThan(Math.min(...textPriorities));
  });

  test('economy text routes to DeepSeek', () => {
    const rFast = rules.find(r => r.name === 'text-economy-fast');
    const rThink = rules.find(r => r.name === 'text-economy-think');

    expect(rFast.targets[0]).toMatchObject({ provider: 'deepseek', model: 'deepseek-chat' });
    expect(rThink.targets[0]).toMatchObject({ provider: 'deepseek', model: 'deepseek-reasoner' });
  });

  test('quality routes to Kimi with OpenAI fallback', () => {
    const rFast = rules.find(r => r.name === 'text-quality-fast');
    const rThink = rules.find(r => r.name === 'text-quality-think');

    expect(rFast.targets[0]).toMatchObject({ provider: 'kimi', model: 'kimi-k2.5' });
    expect(rThink.targets[0]).toMatchObject({ provider: 'kimi', model: 'kimi-k2-thinking' });
    expect(rFast.fallbacks).toEqual(expect.arrayContaining(['openai/o3', 'openai/o1']));
    expect(rThink.fallbacks).toEqual(expect.arrayContaining(['openai/o3', 'openai/o1']));
  });

  test('research routes to OpenRouter Grok with fallbacks', () => {
    const rFast = rules.find(r => r.name === 'text-research-fast');
    const rThink = rules.find(r => r.name === 'text-research-think');

    expect(rFast.targets[0]).toMatchObject({ provider: 'openrouter', model: 'x-ai/grok-4.20' });
    expect(rThink.targets[0]).toMatchObject({ provider: 'openrouter', model: 'x-ai/grok-4.20' });
    expect(rFast.fallbacks).toEqual(
      expect.arrayContaining([
        'openrouter/anthropic/claude-3.5-sonnet',
        'openrouter/nousresearch/hermes-4-405b'
      ])
    );
  });

  test('rules use required request headers', () => {
    for (const r of rules) {
      expect(r.cel_expression).toContain('headers["x-tier"]');
      expect(r.cel_expression).toContain('headers["x-modality"]');
      if (r.name.includes('think') || r.name.includes('fast')) {
        expect(r.cel_expression).toContain('headers["x-reasoning"]');
      }
    }
  });
});

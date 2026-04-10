/**
 * Integration tests for embedding_service POST /classify.
 *
 * Start the service first, e.g.:
 *   docker compose up -d embedding-service
 *
 * Run:
 *   RUN_EMBEDDING_CLASSIFY=1 npm test -- tests/embedding_classify.test.js
 *   # or (same gate as classifier e2e):
 *   JEST_E2E=1 npm test -- tests/embedding_classify.test.js
 */

const truthy = (v) => ['1', 'true', 'yes', 'on'].includes(String(v || '').toLowerCase());
const hasCliTestName =
  process.argv.includes('-t') || process.argv.includes('--testNamePattern');

const shouldRun =
  truthy(process.env.RUN_EMBEDDING_CLASSIFY) ||
  truthy(process.env.JEST_E2E) ||
  hasCliTestName;

const baseUrl = process.env.EMBEDDING_SERVICE_URL || 'http://127.0.0.1:8001';

async function classify(text) {
  const res = await fetch(`${baseUrl}/classify`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ text })
  });
  let body = {};
  try {
    body = await res.json();
  } catch {
    body = {};
  }
  return { status: res.status, body };
}

const describeOrSkip = shouldRun ? describe : describe.skip;

describeOrSkip('Embedding service — POST /classify (intent)', () => {
  let serviceReady = false;

  beforeAll(async () => {
    try {
      const res = await fetch(`${baseUrl}/health`, { signal: AbortSignal.timeout(8000) });
      const j = await res.json();
      serviceReady = res.ok && j.model_ready === true;
    } catch {
      serviceReady = false;
    }
    if (!serviceReady) {
      throw new Error(
        `Embedding service not ready at ${baseUrl}. ` +
          'Start it (e.g. docker compose up -d embedding-service) or set EMBEDDING_SERVICE_URL.'
      );
    }
  }, 15000);

  test('rejects empty text with 400', async () => {
    const { status, body } = await classify('   ');
    expect(status).toBe(400);
    expect(body.detail).toBeDefined();
  });

  test('code_simple: direct implementation phrasing', async () => {
    // Match route seed phrases closely (longer paraphrases can fall through to no_route).
    const { status, body } = await classify('write a quick sort algorithm');
    expect(status).toBe(200);
    expect(body.route_name).toBe('code_simple');
    expect(body.task_type).toBe('code');
    expect(body.reasoning).toBe('fast');
    expect(body.confidence).toBeGreaterThanOrEqual(0.5);
    expect(body.fallback_reason).toBeNull();
  });

  test('code_complex: architecture / performance phrasing', async () => {
    const { status, body } = await classify(
      'We need to refactor the authentication module and improve scalability of the system.'
    );
    expect(status).toBe(200);
    expect(body.route_name).toBe('code_complex');
    expect(body.task_type).toBe('code_complex');
    expect(body.reasoning).toBe('think');
  });

  test('reasoning: step-by-step analysis phrasing', async () => {
    const { status, body } = await classify(
      'Explain step by step why this algorithm works and analyze the time complexity.'
    );
    expect(status).toBe(200);
    expect(body.route_name).toBe('reasoning');
    expect(body.reasoning).toBe('think');
    expect(body.task_type).toBe('reasoning');
  });

  test('research: academic / literature phrasing', async () => {
    const { status, body } = await classify('peer review academic paper summary');
    expect(status).toBe(200);
    expect(body.route_name).toBe('research');
    expect(body.tier).toBe('research');
    expect(body.task_type).toBe('research');
  });

  test('casual: greeting falls back with low tier', async () => {
    const { status, body } = await classify('Hello, how is your day going?');
    expect(status).toBe(200);
    expect(body.route_name).toBe('casual');
    expect(body.tier).toBe('economy');
    expect(body.task_type).toBe('casual');
    expect(body.fallback_reason).toMatch(/^(no_route_matched|low_confidence_matched_[a-z0-9_]+)$/);
  });

  test('code_understand: reading and explaining existing code', async () => {
    const { status, body } = await classify(
      'Can you explain what this function does and walk me through the logic?'
    );
    expect(status).toBe(200);
    expect(body.route_name).toBe('code_understand');
    expect(body.task_type).toBe('code_understand');
    expect(body.reasoning).toBe('fast');
  });

  test('system_design: designing a new system from scratch', async () => {
    const { status, body } = await classify(
      'How should I design a distributed notification service — microservices or monolith?'
    );
    expect(status).toBe(200);
    expect(body.route_name).toBe('system_design');
    expect(body.task_type).toBe('system_design');
    expect(body.reasoning).toBe('think');
  });

  test('system_design: technology selection tradeoff (not reasoning)', async () => {
    // "SQL vs NoSQL" is a tech-selection decision, should route to system_design not reasoning
    const { status, body } = await classify(
      'Which database should I choose, SQL or NoSQL, for a high-traffic application?'
    );
    expect(status).toBe(200);
    expect(body.route_name).toBe('system_design');
    expect(body.task_type).toBe('system_design');
  });

  test('writing: technical documentation phrasing', async () => {
    const { status, body } = await classify(
      'Help me write a README and generate docstrings for this module.'
    );
    expect(status).toBe(200);
    expect(body.route_name).toBe('writing');
    expect(body.task_type).toBe('writing');
    expect(body.reasoning).toBe('fast');
  });

  test('diagram: draw a diagram (not writing)', async () => {
    // Drawing a diagram is output-format intent, should not fall into writing
    const { status, body } = await classify(
      'Draw a sequence diagram showing the authentication flow using Mermaid.'
    );
    expect(status).toBe(200);
    expect(body.route_name).toBe('diagram');
    expect(body.task_type).toBe('diagram');
    expect(body.reasoning).toBe('fast');
  });

  test('planning: task breakdown and estimation', async () => {
    const { status, body } = await classify(
      'Break down this feature into subtasks and estimate the effort for each.'
    );
    expect(status).toBe(200);
    expect(body.route_name).toBe('planning');
    expect(body.task_type).toBe('planning');
    expect(body.reasoning).toBe('think');
  });

  test('brainstorm: open-ended ideation phrasing', async () => {
    const { status, body } = await classify(
      "Let's brainstorm some creative approaches to solving this caching problem."
    );
    expect(status).toBe(200);
    expect(body.route_name).toBe('brainstorm');
    expect(body.task_type).toBe('brainstorm');
    expect(body.reasoning).toBe('think');
  });
});

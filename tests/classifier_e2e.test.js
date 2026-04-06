const { shouldRun, imagePath, sendRequest, findLogByTestId } = require('./e2e_helper');

describe('SmartClassifier Plugin (classification based on body keywords)', () => {
  const maybe = shouldRun ? test : test.skip;

  const visionTest = imagePath ? maybe : test.skip;
  visionTest('Classifies vision task based on image presence', async () => {
    // We send NO headers, only the body containing an image.
    const { id } = await sendRequest({
      prompt: 'Is this an image?',
      modality: 'vision' // e2e_helper's sendRequest will put the image in messages for us
    });

    const log = await findLogByTestId(id);

    console.log('Vision test log:', JSON.stringify(log, null, 2));

    // routing_rule_name should reflect the quality version since vision -> quality tier in plugin
    expect(log.routing_rule_name).toBe('vision-quality-fast');
    
    // We can also verify headers injected by the plugin are present in input_headers if exposed
    // Or just check that it correctly routed.
    expect(['kimi', 'openai']).toContain(log.provider);
  }, 40000);

  maybe('Classifies heavy coding task based on code keywords', async () => {
    // Send a message with code keywords (e.g., 'func main()')
    const { id } = await sendRequest({
      prompt: 'Can you finish this code? func main() { }'
    });

    const log = await findLogByTestId(id);
    
    // heavy coding -> text-quality-fast in our logic
    expect(log.routing_rule_name).toBe('text-quality-fast');
    expect(log.provider).toBe('kimi');
  }, 40000);

  maybe('Classifies deep reasoning task based on reasoning keywords', async () => {
    // Send a message with reasoning keywords (e.g., 'analyze step by step')
    const { id } = await sendRequest({
      prompt: 'Please analyze this complex problem step by step'
    });

    const log = await findLogByTestId(id);
    
    // deep reasoning -> text-research-think in our logic
    expect(log.routing_rule_name).toBe('rr-text-research-think'); // ID check
    // Wait, let's verify if matches rule name or ID. findLogByTestId usually has matching properties.
    if (log.routing_rule_name) {
        expect(log.routing_rule_name).toBe('text-research-think');
    } else {
        expect(log.routing_rule_id).toBe('rr-text-research-think');
    }
  }, 40000);

  maybe('Defaults to simple chat for normal text', async () => {
    const { id } = await sendRequest({
      prompt: 'Hello, how are you?'
    });

    const log = await findLogByTestId(id);
    
    // simple chat -> text-economy-fast
    expect(log.routing_rule_name).toBe('text-economy-fast');
    expect(log.provider).toBe('deepseek');
  }, 40000);
});

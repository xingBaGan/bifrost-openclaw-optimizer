# Token Optimization To-Do (Priority Order)

- [ ] P1 OpenClaw -> Bifrost routing contract: stable aliases (`economy`/`quality`/`research`) + required headers (`x-tier`, `x-reasoning`, `x-modality`)
	- [x] P1.1 Define alias mapping and document it in a single place
	- [ ] P1.2 Ensure OpenClaw client always sends `x-tier`, `x-reasoning`, `x-modality`
	- [x] P1.3 Add a minimal routing smoke test or curl example

- [ ] P2 Trim OpenClaw context files to <8 KB total (SOUL/AGENTS/MEMORY/TOOLS) and move details to `vault/`; enforce `memory_search` rule
	- [ ] P2.1 Rewrite SOUL/AGENTS/MEMORY/TOOLS to target sizes
	- [ ] P2.2 Move long-form content into `vault/` with pointer links
	- [ ] P2.3 Add explicit `memory_search` rule to SOUL/AGENTS
	- [ ] P2.4 Measure total injected context size and record baseline

- [ ] P3 Implement 3-tier memory with local embeddings (Ollama) for on-demand retrieval; standardize `memory/` + `vault/` structure
	- [ ] P3.1 Install Ollama and pull the embedding model
	- [ ] P3.2 Create `memory/` + `vault/` directories with standard subfolders
	- [ ] P3.3 Populate MEMORY index with pointers only
	- [ ] P3.4 Verify `memory_search` returns relevant hits

- [ ] P4 Orchestrator/worker split: main model plans, sub-agents execute on cheaper tier via Bifrost
	- [ ] P4.1 Define decision tree for when to spawn workers
	- [ ] P4.2 Map worker tasks to `economy` tier via headers
	- [ ] P4.3 Add a template spawn instruction for code and research tasks

- [ ] P5 Fallback chains per tier + automatic downgrade when token/cost exceeds limits or provider errors
	- [ ] P5.1 Define fallback models per tier in Bifrost routing rules
	- [ ] P5.2 Implement downgrade trigger (token or cost threshold)
	- [ ] P5.3 Add a test that simulates provider failure and validates fallback

- [ ] P6 Semantic cache header readiness (`x-bf-cache-key`) + response reuse for identical prompts
	- [ ] P6.1 Define cache key composition (prompt + tool inputs + model tier)
	- [ ] P6.2 Add `x-bf-cache-key` to client requests
	- [ ] P6.3 Log cache hit rate for validation
	
- [ ] P7 Conversation truncation with summarization for older turns
	- [ ] P7.1 Set summarization threshold (turn count or tokens)
	- [ ] P7.2 Choose a summarizer model tier and format
	- [ ] P7.3 Store summaries in memory/vault for later retrieval

- [ ] P8 Tool/function call output length limits
	- [ ] P8.1 Set max output tokens per tool call
	- [ ] P8.2 Truncate or summarize oversized tool outputs
	- [ ] P8.3 Add tests for tool output limits

- [ ] P9 Log storage compression + token/cost analytics (summary + per-rule stats)
	- [ ] P9.1 Define log schema for token/cost per request
	- [ ] P9.2 Add summary fields for long inputs/outputs
	- [ ] P9.3 Add a report view grouped by routing rule

# Devlog Tools

## benchmark_summarizer.sh

Test different LLM models and prompts for the summarizer.

**Usage:**
```bash
# Quick test
./tools/benchmark_summarizer.sh --models qwen2.5:7b qwen2.5:14b --counts 25

# Full docs
./tools/benchmark_summarizer.sh --help
# Or just read the script header
```

**Results:**
- Creates `benchmark_results/benchmark_TIMESTAMP.md`
- Contains all summaries for manual review
- Just open in your editor and read the actual outputs

**Workflow:**
1. Run benchmark
2. Open results markdown file
3. Read summaries to judge quality
4. Pick best model/prompt
5. Update config: `./devlog config plugins.summarizer.model "qwen2.5:14b"`

# Devlog Tools

Utility scripts for testing and analyzing the devlog summarizer.

## Available Tools

### benchmark_summarizer.sh
Test different LLM models and prompt variants. Run `--help` for full documentation.

**Quick start:**
```bash
./tools/benchmark_summarizer.sh --models qwen2.5:14b --counts 50
```

### historical_summarizer.sh
Generate summaries for historical time periods. Run `--help` for full documentation.

**Quick start:**
```bash
./tools/historical_summarizer.sh --hours 24 --interval 30
```

Both scripts include comprehensive help text with examples and options.

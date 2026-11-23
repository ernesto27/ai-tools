export const MODEL_CATALOG: Readonly<Record<string, readonly string[]>> = {
  // Verified 2026-11-23 against the CLI catalog and model-selection docs:
  // https://github.com/openai/codex/blob/rust-v0.171.0/codex-rs/models-manager/models.json
  // https://learn.chatgpt.com/docs/models?surface=cli
  codex: [
    "gpt-6.2-astra", // Codex CLI 0.166.0+; not available on Free or K-12 plans.
    "gpt-6.2-sol", // Codex CLI 0.166.0+; not available on Free or K-12 plans.
    "gpt-6-astra",
    "gpt-6.1-sol",
    "gpt-6.1-terra",
    "gpt-6-sol",
    "gpt-6-luna",
    "gpt-5.6-sol",
    "gpt-5.6-terra",
    "gpt-5.6-luna",
    "gpt-5.5", // API-key authentication only; retired from ChatGPT sign-in.
  ],
  // https://support.claude.com/en/articles/11940350-claude-code-model-configuration
  // https://code.claude.com/docs/en/model-config
  claude: [
    "claude-fable-5-2", // Requires Claude Code 2.1.321+ and eligible account access.
    "claude-fable-5-1",
    "claude-opus-5-5",
    "claude-sonnet-5-5",
    "claude-haiku-5-5",
    "claude-opus-5",
    "claude-sonnet-5",
    "claude-fable-5",
    "claude-opus-4-8",
    "claude-opus-4-7",
    "claude-sonnet-4-6",
    "claude-opus-4-6",
    "claude-opus-4-5-20251101",
    "claude-haiku-4-5-20251001",
    "claude-sonnet-4-5-20250929",
  ],
};

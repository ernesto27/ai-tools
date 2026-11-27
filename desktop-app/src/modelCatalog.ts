// Verified 2026-11-27. Selection still depends on CLI version and account access.
export const MODEL_CATALOG: Readonly<Record<string, readonly string[]>> = {
  // CLI catalog: https://github.com/openai/codex/blob/rust-v0.172.0/codex-rs/models-manager/models.json
  // Selection/access: https://learn.chatgpt.com/docs/models?surface=cli
  codex: [
    "gpt-6.2-astra",
    "gpt-6.2-sol",
    "gpt-6-astra",
    "gpt-6.1-sol",
    "gpt-6.1-terra",
    "gpt-6-sol",
    "gpt-6-luna",
    "gpt-5.6-sol",
    "gpt-5.6-terra",
    "gpt-5.6-luna",
    "gpt-5.5", // API-key sessions only; retired for ChatGPT sign-in.
  ],
  // Full model IDs: https://code.claude.com/docs/en/model-config
  // CLI releases: https://code.claude.com/docs/en/changelog
  // Status/retirements: https://platform.claude.com/docs/en/about-claude/model-deprecations
  claude: [
    "claude-fable-5-2", // Requires Claude Code 2.1.321+; usage credits may apply.
    "claude-fable-5-1",
    "claude-fable-5",
    // Mythos: verified organizations only (Anthropic verification programs, e.g. Cyber Verification Program).
    // Source: https://platform.claude.com/docs/en/models/mythos-5-2/overview
    "claude-mythos-5-2",
    "claude-mythos-5-1", // Unverified in Claude Code docs; API status Active.
    "claude-mythos-5",
    "claude-opus-5-5",
    "claude-opus-5",
    "claude-opus-4-8",
    "claude-opus-4-7",
    "claude-opus-4-6",
    "claude-opus-4-5", // Deprecated; selectable until 2027-01-22.
    "claude-sonnet-5-5",
    "claude-sonnet-5",
    "claude-sonnet-4-6",
    "claude-haiku-5-5",
    "claude-haiku-4-5",
  ],
};

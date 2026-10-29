import { CONFIG_STAGES } from "./data";

export type TaskSource = "file" | "folder" | "drive";

export interface StageSettings {
  agent: string;
  model: string;
  effort: string;
  customSkill: string;
}

export interface StageConfig extends Partial<StageSettings> {
  disabled?: boolean;
  saveFindings?: boolean;
  [key: string]: unknown;
}

export interface FactoryConfig {
  agentSandbox?: {
    agent?: string;
    model?: string;
    effort?: string;
    branch?: string;
    baseImage?: string;
    push?: boolean;
    pr?: boolean;
    hn?: boolean;
    run?: { branch?: string; query?: string; filePrompt?: string; [key: string]: unknown };
    resume?: { branch?: string; [key: string]: unknown };
    [key: string]: unknown;
  };
  stages?: Record<string, StageConfig>;
  documents?: string[];
  googleDrive?: { folders?: string[]; files?: string[]; [key: string]: unknown };
  taskFolder?: string;
  jiraUrl?: string;
  [key: string]: unknown;
}

export interface ConfigForm {
  agent: string;
  model: string;
  effort: string;
  branch: string;
  baseImage: string;
  stageSettings: Record<string, StageSettings>;
  flags: { push: boolean; pr: boolean; hn: boolean };
  off: Record<string, boolean>;
  saveFindings: boolean;
  taskSrc: TaskSource;
  taskVals: Record<TaskSource, string>;
  documents: string[];
}

type JsonObject = Record<string, unknown>;

export function applyTaskInputs(original: FactoryConfig | null, values: {
  task: string;
  file: string;
  folder: string;
  jira: string;
  branch?: string;
}): FactoryConfig {
  const config = { ...original };
  const agentSandbox = { ...original?.agentSandbox };
  const run = { ...agentSandbox.run };

  function setOptionalString(target: JsonObject, key: string, value: string) {
    if (value.trim()) target[key] = value;
    else delete target[key];
  }

  setOptionalString(config, "taskFolder", values.folder);
  setOptionalString(config, "jiraUrl", values.jira);
  setOptionalString(run, "query", values.task);
  setOptionalString(run, "filePrompt", values.file);
  if (values.branch !== undefined) {
    setOptionalString(agentSandbox, "branch", values.branch.trim());
    delete run.branch;
  }

  if (Object.keys(run).length || agentSandbox.run !== undefined) agentSandbox.run = run;
  if (Object.keys(agentSandbox).length || original?.agentSandbox !== undefined) config.agentSandbox = agentSandbox;

  return config;
}

function isObject(value: unknown): value is JsonObject {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function requireObject(value: unknown, path: string): asserts value is JsonObject {
  if (!isObject(value)) throw new Error(`${path} must be a JSON object.`);
}

function checkFields(object: JsonObject, fields: string[], type: "string" | "boolean", path: string) {
  for (const field of fields) {
    if (object[field] !== undefined && typeof object[field] !== type) {
      throw new Error(`${path}.${field} must be a ${type}.`);
    }
  }
}

function checkStringList(value: unknown, path: string) {
  if (value !== undefined && (!Array.isArray(value) || value.some((item) => typeof item !== "string"))) {
    throw new Error(`${path} must be a list of strings.`);
  }
}

export function parseFactoryConfig(value: unknown): FactoryConfig {
  requireObject(value, "Configuration");
  if (value.agentSandbox !== undefined) {
    requireObject(value.agentSandbox, "agentSandbox");
    checkFields(value.agentSandbox, ["agent", "model", "effort", "branch", "baseImage"], "string", "agentSandbox");
    checkFields(value.agentSandbox, ["push", "pr", "hn"], "boolean", "agentSandbox");
    if (value.agentSandbox.run !== undefined) {
      requireObject(value.agentSandbox.run, "agentSandbox.run");
      checkFields(value.agentSandbox.run, ["branch", "query", "filePrompt"], "string", "agentSandbox.run");
    }
  }
  if (value.stages !== undefined) {
    requireObject(value.stages, "stages");
    for (const { key } of CONFIG_STAGES) {
      const stage = value.stages[key];
      if (stage === undefined) continue;
      requireObject(stage, `stages.${key}`);
      checkFields(stage, ["agent", "model", "effort", "customSkill"], "string", `stages.${key}`);
      checkFields(stage, ["disabled", "saveFindings"], "boolean", `stages.${key}`);
    }
  }
  checkStringList(value.documents, "documents");
  checkFields(value, ["taskFolder", "jiraUrl"], "string", "Configuration");
  if (value.googleDrive !== undefined) {
    requireObject(value.googleDrive, "googleDrive");
    checkStringList(value.googleDrive.files, "googleDrive.files");
    checkStringList(value.googleDrive.folders, "googleDrive.folders");
  }
  // Known editable properties have been checked; other JSON properties stay intact.
  return value as FactoryConfig;
}

export function configToForm(config: FactoryConfig | null): ConfigForm {
  const shared = config?.agentSandbox;
  const stages = config?.stages;
  return {
    agent: shared?.agent ?? "codex",
    model: shared?.model ?? "",
    effort: shared?.effort || "medium",
    branch: shared?.branch ?? "factory-task",
    baseImage: shared?.baseImage ?? "",
    flags: { push: shared?.push ?? false, pr: shared?.pr ?? false, hn: shared?.hn ?? false },
    stageSettings: Object.fromEntries(CONFIG_STAGES.map((s) => [s.key, {
      agent: stages?.[s.key]?.agent ?? (config ? "" : s.agent),
      model: stages?.[s.key]?.model ?? "",
      effort: stages?.[s.key]?.effort || shared?.effort || "medium",
      customSkill: stages?.[s.key]?.customSkill ?? "",
    }])),
    off: Object.fromEntries(CONFIG_STAGES.map((s) => [s.key, stages?.[s.key]?.disabled ?? false])),
    saveFindings: stages?.securityReview?.saveFindings ?? false,
    taskSrc: config?.taskFolder ? "folder" : shared?.run?.filePrompt !== undefined ? "file" : config?.googleDrive?.files?.length ? "drive" : "file",
    taskVals: {
      file: shared?.run?.filePrompt ?? (config ? "" : "task.txt"),
      folder: config?.taskFolder ?? "",
      drive: config?.googleDrive?.files?.[0] ?? "",
    },
    documents: config?.documents ?? [],
  };
}

export function formToConfig(form: ConfigForm, original: FactoryConfig | null): FactoryConfig {
  const stages: Record<string, StageConfig> = {};
  for (const { key } of CONFIG_STAGES) {
    const settings = form.stageSettings[key];
    stages[key] = {
      ...(settings.agent ? { agent: settings.agent } : {}),
      ...(settings.model.trim() ? { model: settings.model.trim() } : {}),
      effort: settings.effort,
      ...(key !== "implementation" && settings.customSkill.trim() ? { customSkill: settings.customSkill.trim() } : {}),
      ...(key === "securityReview" ? { saveFindings: form.saveFindings } : {}),
      ...(form.off[key] ? { disabled: true } : {}),
    };
  }
  return {
    documents: form.documents,
    ...(form.taskSrc === "drive" ? { googleDrive: { files: [form.taskVals.drive, ...(original?.googleDrive?.files?.slice(1) ?? [])] } } : {}),
    ...(form.taskSrc === "folder" ? { taskFolder: form.taskVals.folder } : {}),
    agentSandbox: {
      agent: form.agent,
      ...(form.model.trim() ? { model: form.model.trim() } : {}),
      effort: form.effort,
      ...(form.baseImage.trim() ? { baseImage: form.baseImage.trim() } : {}),
      ...form.flags,
      branch: form.branch,
      run: form.taskSrc === "file" ? { filePrompt: form.taskVals.file } : {},
    },
    stages,
  };
}

// Apply only changes made in the form, preserving omitted settings and extra fields.
export function mergeConfigEdits(original: FactoryConfig, initial: FactoryConfig, current: FactoryConfig): FactoryConfig {
  function merge(base: JsonObject, before: JsonObject, after: JsonObject): JsonObject {
    const result = { ...base };
    for (const key of new Set([...Object.keys(before), ...Object.keys(after)])) {
      if (JSON.stringify(before[key]) === JSON.stringify(after[key])) continue;
      if (!(key in after)) {
        if (isObject(before[key]) && isObject(base[key])) {
          const remaining = merge(base[key], before[key], {});
          if (Object.keys(remaining).length) result[key] = remaining;
          else delete result[key];
        } else {
          delete result[key];
        }
      } else if (isObject(after[key])) {
        result[key] = merge(isObject(base[key]) ? base[key] : {}, isObject(before[key]) ? before[key] : {}, after[key]);
      } else {
        result[key] = after[key];
      }
    }
    return result;
  }
  return merge(original, initial, current) as FactoryConfig;
}

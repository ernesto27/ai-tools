import { toUiMessage, messageText, type UiMessage } from "../messages";
import { useTranslation } from "react-i18next";
import { useEffect, useMemo, useState } from "react";
import { AGENTS, CONFIG_STAGES } from "../data";
import { Checkbox, Crumb, Field, Input, OutlineButton, PrimaryButton, SectionHeading, Select, Title } from "../components/ui";
import { open } from "@tauri-apps/plugin-dialog";
import { invoke } from "@tauri-apps/api/core";
import { StatusMessage } from "../components/StatusMessage";
import { ModelAutocomplete } from "../components/ModelAutocomplete";
import { SkillAutocomplete } from "../components/SkillAutocomplete";
import { useSkills } from "../hooks/useSkills";
import { configToForm, formToConfig, mergeConfigEdits, parseFactoryConfig } from "../config";
import type { FactoryConfig, StageSettings } from "../config";

const FLAGS = [
  { key: "push", hint: "commitPush" },
  { key: "pr", hint: "openAPr" },
  { key: "hn", hint: "hostNetwork" },
] as const;

type FlagKey = (typeof FLAGS)[number]["key"];

const EFFORT_OPTIONS = ["low", "medium", "high", "xhigh"] as const;

const FORM_GRID = "grid grid-cols-1 gap-4 pt-5 pb-8 @min-[400px]:grid-cols-2 @min-[800px]:grid-cols-4";

interface Props {
  repo: string;
  repoPath: string;
}

export function Config({ repo, repoPath }: Props) {
  const { t } = useTranslation();
  const skills = useSkills(repoPath);
  const [agent, setAgent] = useState("codex");
  const [model, setModel] = useState("");
  const [effort, setEffort] = useState("medium");
  const [baseImage, setBaseImage] = useState("");
  const [stageSettings, setStageSettings] = useState<Record<string, StageSettings>>(() =>
    Object.fromEntries(CONFIG_STAGES.map((s) => [s.key, { agent: s.agent, model: "", effort: "medium", customSkill: s.skill }])),
  );
  const [flags, setFlags] = useState<Record<FlagKey, boolean>>({ push: false, pr: false, hn: false });
  const [off, setOff] = useState<Record<string, boolean>>({});
  const [saveFindings, setSaveFindings] = useState(false);
  const [documents, setDocuments] = useState<string[]>([]);
  const [dirty, setDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [saveError, setSaveError] = useState<UiMessage | null>(null);
  const [saveMessage, setSaveMessage] = useState<UiMessage | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<UiMessage | null>(null);
  const [loadAttempt, setLoadAttempt] = useState(0);
  const [loadedConfig, setLoadedConfig] = useState<FactoryConfig | null>(null);
  const [initialConfig, setInitialConfig] = useState<FactoryConfig | null>(null);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setLoadError(null);
    setSaveError(null);
    setSaveMessage(null);

    invoke<unknown>("load_factory_config", { path: repoPath })
      .then((value) => {
        if (!active) return;
        const config = value === null ? null : parseFactoryConfig(value);
        const form = configToForm(config);
        setAgent(form.agent);
        setModel(form.model);
        setEffort(form.effort);
        setBaseImage(form.baseImage);
        setStageSettings(form.stageSettings);
        setFlags(form.flags);
        setOff(form.off);
        setSaveFindings(form.saveFindings);
        setDocuments(form.documents);
        setLoadedConfig(config);
        setInitialConfig(formToConfig(form));
        setSaved(config !== null);
        setDirty(false);
      })
      .catch((error) => {
        if (active) setLoadError(toUiMessage(error));
      })
      .finally(() => {
        if (active) setLoading(false);
      });

    return () => { active = false; };
  }, [repoPath, loadAttempt]);

  // Every edit marks the form dirty until Save is pressed.
  const edit = <T,>(fn: (v: T) => void) => (v: T) => {
    fn(v);
    setDirty(true);
    setSaveMessage(null);
  };

  const editStage = (key: string, field: keyof StageSettings, value: string) => {
    setStageSettings((current) => ({ ...current, [key]: { ...current[key], [field]: value } }));
    setDirty(true);
    setSaveMessage(null);
  };

  async function addDocument() {
    const path = await open({
      directory: false,
      multiple: false,
    });

    if (!path) return;

    setDocuments((current) =>
      current.includes(path) ? current : [...current, path],
    );
    setDirty(true);
    setSaveMessage(null);
  }

  const formConfig = useMemo(() => formToConfig({ agent, model, effort, baseImage, stageSettings, flags, off, saveFindings, documents }),
    [agent, model, effort, baseImage, stageSettings, flags, off, saveFindings, documents]);
  const config = loadedConfig && initialConfig ? mergeConfigEdits(loadedConfig, initialConfig, formConfig) : formConfig;
  const json = JSON.stringify(config, null, 2);

  async function saveConfig() {
    if (saving || loading || loadError) return;
    setSaving(true);
    setSaveError(null);
    setSaveMessage(null);

    try {
      await invoke(loadedConfig ? "update_factory_config" : "create_factory_config", {
        path: repoPath,
        config,
      });
      setLoadedConfig(config);
      setInitialConfig(formConfig);
      setSaved(true);
      setDirty(false);
      setSaveMessage({ key: "softwareFactoryJsonSavedInPath", values: { path: repoPath } });
    } catch (error) {
      setSaveError(toUiMessage(error));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="config-form flex w-full min-w-0 flex-col gap-9">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1">
          <Crumb>{repo} · {repoPath}</Crumb>
          <Title>{t("config")}</Title>
        </div>
        <div className="flex items-center gap-4 text-md">
          <span role="status" className="text-muted">{loading ? t("loading") : loadError ? t("couldNotLoad") : saving ? t("saving") : dirty ? t("unsavedChanges") : saved ? t("saved") : t("notSaved")}</span>
          <PrimaryButton disabled={saving || loading || !!loadError} onClick={saveConfig}>{saving ? t("saving") : t("save")}</PrimaryButton>
        </div>
      </header>
      {saveMessage && <StatusMessage message={messageText(saveMessage, t)} />}
      {saveError && <p role="alert" className="text-sm text-bad">{messageText(saveError, t)}</p>}
      {loadError && (
        <div role="alert" className="flex flex-wrap items-center gap-3 text-sm text-bad">
          <span>{messageText(loadError, t)}</span>
          <OutlineButton onClick={() => setLoadAttempt((attempt) => attempt + 1)}>{t("retry")}</OutlineButton>
        </div>
      )}

      {skills.loading && <p id="skills-loading" role="status" className="text-sm text-muted">{t("loadingSkills")}</p>}
      {skills.error && (
        <div id="skills-error" role="alert" className="flex flex-wrap items-center gap-3 text-sm text-bad">
          <span>{t("couldNotLoadSkills")} {skills.error}</span>
          <OutlineButton onClick={skills.retry}>{t("retry")}</OutlineButton>
        </div>
      )}

      <div className="flex flex-wrap items-start gap-8">
        <fieldset disabled={saving || loading || !!loadError} className="@container m-0 flex min-w-0 flex-[1.2_1_520px] flex-col border-0 p-0">
          <SectionHeading>{t("agentDefaults")}</SectionHeading>
          <div className={FORM_GRID}>
            <Field label={t("agent")}>
              <Select value={agent} onChange={(e) => edit(setAgent)(e.target.value)}>
                {!AGENTS.includes(agent) && <option value={agent}>{agent || t("agentDefault")}</option>}
                {AGENTS.map((a) => (
                  <option key={a} value={a}>
                    {a}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label={t("model")}>
              <ModelAutocomplete
                key={agent}
                id="default-model"
                agent={agent}
                value={model}
                placeholder={t("agentDefaultPlaceholder")}
                disabled={saving || loading || !!loadError}
                onChange={edit(setModel)}
              />
            </Field>
            <Field label={t("effort")}>
              <Select value={effort} onChange={(e) => edit(setEffort)(e.target.value)}>
                {!EFFORT_OPTIONS.some((value) => value === effort) && <option value={effort}>{effort}</option>}
                {EFFORT_OPTIONS.map((value) => (
                  <option key={value} value={value}>{t(value)}</option>
                ))}
              </Select>
            </Field>
            <Field label={t("baseImage")}>
              <Input mono value={baseImage} placeholder={t("optional")} onChange={(e) => edit(setBaseImage)(e.target.value)} />
            </Field>
          </div>

          <SectionHeading>Git</SectionHeading>
          <div className="flex flex-wrap gap-x-8 gap-y-2 pt-4 pb-8 text-base">
            {FLAGS.map((f) => (
              <Checkbox key={f.key} checked={flags[f.key]} onChange={() => edit(setFlags)({ ...flags, [f.key]: !flags[f.key] })}>
                <span className="font-mono">{f.key}</span>
                <span className="text-sm text-muted">{t(f.hint)}</span>
              </Checkbox>
            ))}
          </div>

          <SectionHeading>{t("stages")}</SectionHeading>
          <p className="pt-3 pb-5 text-sm text-muted">{t("setOverridesForEachStageEmptyAgentAndModelValuesUseTheDefaultsAbove")}</p>
          <div className="flex flex-col gap-4 pb-8">
            {CONFIG_STAGES.map((s) => (
              <div key={s.key} className={`flex flex-col gap-4 rounded-lg border border-line p-5 ${off[s.key] ? "bg-canvas" : "bg-field"}`}>
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <h3 className="text-base font-medium">{t(s.label)}</h3>
                  <div className="flex flex-wrap items-center gap-4">
                    {s.key === "securityReview" && (
                      <Checkbox tight className="text-sm" disabled={!!off[s.key]} checked={saveFindings} onChange={() => edit(setSaveFindings)(!saveFindings)}>
                        {t("saveFindings")}
                      </Checkbox>
                    )}
                    <Checkbox
                      tight
                      className="text-sm"
                      aria-label={t("disableStage", { stage: t(s.label) })}
                      checked={!!off[s.key]}
                      onChange={() => edit(setOff)({ ...off, [s.key]: !off[s.key] })}
                    >
                      {t("disabled")}
                    </Checkbox>
                  </div>
                </div>
                <fieldset disabled={!!off[s.key]} aria-label={t("stageSettings", { stage: t(s.label) })} className="m-0 grid min-w-0 grid-cols-1 gap-4 border-0 p-0 disabled:opacity-50 @min-[400px]:grid-cols-2 @min-[800px]:grid-cols-[minmax(0,1fr)_minmax(0,1.3fr)_minmax(0,0.8fr)_minmax(0,1.2fr)]">
                  <Field label={t("agent")}>
                    <Select className="min-w-0 w-full" value={stageSettings[s.key].agent} onChange={(e) => editStage(s.key, "agent", e.target.value)}>
                      <option value="">{t("useSharedAgent")}</option>
                      {stageSettings[s.key].agent && !AGENTS.includes(stageSettings[s.key].agent) && <option value={stageSettings[s.key].agent}>{stageSettings[s.key].agent}</option>}
                      {AGENTS.map((a) => <option key={a} value={a}>{a}</option>)}
                    </Select>
                  </Field>
                  <Field label={t("model")}>
                    <ModelAutocomplete
                      key={stageSettings[s.key].agent || agent}
                      id={`${s.key}-model`}
                      agent={stageSettings[s.key].agent || agent}
                      value={stageSettings[s.key].model}
                      disabled={saving || loading || !!loadError || !!off[s.key]}
                      onChange={(value) => editStage(s.key, "model", value)}
                    />
                  </Field>
                  <Field label={t("effort")}>
                    <Select className="min-w-0 w-full" value={stageSettings[s.key].effort} onChange={(e) => editStage(s.key, "effort", e.target.value)}>
                      {!EFFORT_OPTIONS.some((value) => value === stageSettings[s.key].effort) && <option value={stageSettings[s.key].effort}>{stageSettings[s.key].effort}</option>}
                      {EFFORT_OPTIONS.map((value) => (
                        <option key={value} value={value}>{t(value)}</option>
                      ))}
                    </Select>
                  </Field>
                  {s.key !== "implementation" && (
                    <Field label={t("customSkill")}>
                      <SkillAutocomplete
                        key={`${repoPath}-${s.key}`}
                        id={`${s.key}-skill`}
                        value={stageSettings[s.key].customSkill}
                        names={skills.names}
                        disabled={saving || loading || !!loadError || !!off[s.key] || skills.loading || !!skills.error}
                        onChange={(value) => editStage(s.key, "customSkill", value)}
                      />
                    </Field>
                  )}
                </fieldset>
              </div>
            ))}
          </div>

          <SectionHeading>{t("documents")}</SectionHeading>
          <div className="flex flex-col font-mono text-md">
            {documents.map((doc) => (
              <div key={doc} className="flex min-h-11 items-center justify-between border-b border-line">
                {doc}
                <button
                  type="button"
                  aria-label={t("removeDocument", { document: doc })}
                  className="size-9 text-xl text-muted hover:text-ink"
                  onClick={() => edit(setDocuments)(documents.filter((d) => d !== doc))}
                >
                  ×
                </button>
              </div>
            ))}
            {documents.length === 0 && <p className="py-3 font-sans text-sm text-muted">{t("noDocumentsAdded")}</p>}
            <button
              type="button"
              onClick={addDocument}
              className="min-h-10 self-start font-sans text-md underline-offset-4 hover:underline"
            >
              {t("addDocument")}
            </button>
          </div>
        </fieldset>

        <aside aria-labelledby="json-h" className="sticky top-0 flex min-w-0 flex-[1_1_440px] flex-col gap-2.5">
          <h2 id="json-h" className="font-mono text-sm font-normal text-muted">
            software-factory.json
          </h2>
          <pre className="overflow-x-auto rounded-md bg-panel p-4 font-mono text-sm leading-[1.7] whitespace-pre">{loading ? t("loadingConfiguration") : loadError ? t("configurationUnavailable") : json}</pre>
        </aside>
      </div>
    </div>
  );
}

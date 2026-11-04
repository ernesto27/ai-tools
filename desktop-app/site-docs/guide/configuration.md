# Configuration

Use a repository's **Config** screen to create or update its `software-factory.json`. The JSON preview shows the configuration you are preparing to save.

## Agent defaults and stages

Choose the default agent, model, effort, base image, and branch. Model fields offer autocomplete suggestions and allow custom values. Available models and effort settings depend on the selected agent and provider.

Adjust individual workflow stages to override their agent settings, select a custom skill where supported, or disable a stage. The workflow includes implementation, review, risk classification, and a final walkthrough. See the [Software Factory workflow guide](https://ernesto27.github.io/ai-tools/soft-factory/workflow) for what each stage does.

![Repository configuration screen](/screenshots/config-dark.png)

## Task sources and documents

The Config screen supports a task file, a task folder, or a Google Drive source, as well as supporting documents. Use the file and folder pickers for local paths. A task folder contains the task and its supporting context; see the [CLI task documents guide](https://ernesto27.github.io/ai-tools/soft-factory/documents) for the expected structure.

The **New run** screen also lets you enter task text or a Jira URL and review the resulting configuration before launching. Google Drive and Jira access require their corresponding CLI setup.

## Save changes

Save the configuration before starting a run. Resolve any displayed read or write errors; the JSON preview alone does not mean the file has been saved.

## Desktop preferences

Open **Settings** in the sidebar to switch between light and dark themes or choose **English** and **Español (Argentina)**. Preferences apply immediately and are remembered on this device. Interface translations work offline; CLI output and agent-generated content keep their original language.

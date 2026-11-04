# Configuración

Crea `software-factory.json` en la carpeta del proyecto y ejecuta Soft Factory
allí. Empieza con esta configuración:

```json
{
  "agentSandbox": {
    "agent": "codex",
    "branch": "fix-login",
    "push": false,
    "pr": false,
    "run": {
      "filePrompt": "task.txt"
    }
  }
}
```

Escribe la tarea en `task.txt`. Los ajustes compartidos se aplican a la
implementación y las revisiones. Elige una rama nueva para cada tarea nueva.

| Sección | Propósito |
| --- | --- |
| `agentSandbox` | Agente, rama, archivo de tarea y ajustes del sandbox. Obligatoria. |
| `stages` | Ajustes opcionales para las etapas del flujo de trabajo. |
| `documents` | Archivos locales opcionales que se incluyen con la tarea. |
| `googleDrive` | Google Docs opcionales que se incluyen con la tarea. |

Usa nombres de propiedades en camelCase. Los ejemplos siguientes son añadidos
a este archivo; conserva los ajustes existentes al incorporarlos.

## Ajustes compartidos y sobrescrituras {#shared-settings-and-overrides}

Coloca los ajustes en `agentSandbox` para compartirlos. Añade una sección `run`
o `resume` solo cuando necesites valores diferentes:

- `run` controla la implementación de una tarea nueva.
- `resume` controla las etapas posteriores, `continue` y la recuperación con `resume <path>`.
- Los ajustes de etapa tienen prioridad sobre la sección seleccionada y luego sobre los compartidos.

El flujo predeterminado usa `run.branch`; la continuación y la recuperación
usan `resume.branch`. Ambos recurren a la `branch` compartida si no hay un
valor en la sección. Todas las etapas de una ejecución usan la misma rama.

La tarea local siempre proviene de `run.filePrompt`, incluso en la continuación
y recuperación. Para usar una incidencia de Jira, pasa [`--jira`](jira.md).

### Opciones del sandbox {#sandbox-options}

Estas opciones pueden compartirse o definirse dentro de `run` y `resume`:

| Opción | Propósito |
| --- | --- |
| `agent` | `codex`, `claude`, `opencode` o `pi`. Las revisiones necesitan soporte para subagentes. |
| `branch` | Rama del sandbox para la tarea. |
| `model` | Modelo compatible con el agente y la cuenta elegidos. |
| `effort` | Nivel de esfuerzo admitido por el agente. |
| `baseImage` | Imagen Docker con las herramientas necesarias para la tarea. |
| `push` | Crea un commit y envía los cambios después de una etapa. |
| `pr` | Crea un commit, envía los cambios y crea o reutiliza un pull request de GitHub. |
| `hn` | Permite acceder a servicios del host. El valor predeterminado es `false`. |
| `file` | Lista de archivos montados en modo de solo lectura. |

Solo `run` y `resume` aceptan además `filePrompt`, `query`, `commitMessage` e
`image`. Usa `run.filePrompt` para la tarea original; Factory proporciona los
prompts de las etapas posteriores. `image` es una lista de adjuntos para los
agentes compatibles. `commitMessage` se aplica al publicar cambios.

Los valores de sección reemplazan los compartidos, incluidos `false` explícito,
cadenas vacías y listas vacías. Las opciones omitidas usan los valores
predeterminados de Agent Sandbox.

## Agentes y modelos por etapa {#stage-agents-and-models}

Usa `stages` para modificar pasos individuales:

```json
{
  "stages": {
    "implementation": {"effort": "high"},
    "codeReview": {"agent": "claude", "model": "claude-sonnet-4-6"}
  }
}
```

Autentica cada agente que uses. Al cambiar de agente, elige también un modelo compatible.

| Clave de etapa | Paso |
| --- | --- |
| `implementation` | Implementa la tarea. |
| `codeReview` | Revisa el código y aplica correcciones. |
| `securityReview` | Revisa la seguridad y aplica correcciones. |
| `riskClassification` | Clasifica el riesgo final. |
| `reviewChanges` | Escribe una explicación sin modificar los archivos. |

Las cinco etapas aceptan `agent`, `model`, `effort`, `customSkill` y `disabled`.
La revisión de seguridad también acepta `saveFindings`.

Los valores de etapa `agent` y `model` omitidos o vacíos conservan los ajustes
heredados. En `effort`, omitirlo o usar `null` hereda el valor; `""` lo sobrescribe
con un valor vacío. Agent Sandbox valida los agentes, modelos y niveles de
esfuerzo admitidos.

## Skills personalizadas {#custom-skills}

Añade el nombre de una skill a cualquier etapa:

```json
{
  "stages": {
    "implementation": {"customSkill": "implementation-guide"},
    "codeReview": {"customSkill": "project-review"}
  }
}
```

Crea `.agents/skills/<name>/SKILL.md` en tu proyecto. Factory también busca
`.claude/skills/<name>/SKILL.md` y usa la primera coincidencia. Indica el nombre
de un directorio, no una ruta.

Las skills añaden instrucciones a la tarea o a las instrucciones de revisión
predeterminadas de la etapa. Una skill ausente o ilegible detiene el flujo antes
de ejecutar agentes si su etapa debe ejecutarse. Las etapas deshabilitadas no
cargan sus skills.

## Deshabilitar etapas {#disabling-stages}

Establece `disabled: true` para omitir una etapa:

```json
{
  "stages": {
    "securityReview": {"disabled": true}
  }
}
```

Las etapas posteriores siguen ejecutándose. Las etapas omitidas no producen
registros ni informes. La clasificación de riesgo indica la evidencia de
revisión que falta.

Si omites la implementación, el worktree del sandbox seleccionado debe existir.
Si omites todas las etapas, se siguen comprobando la configuración y el contexto.

## Guardar hallazgos de seguridad sin corregirlos {#save-security-findings-without-corrections}

Para obtener sugerencias de corrección sin aplicarlas:

```json
{
  "stages": {
    "securityReview": {"saveFindings": true}
  }
}
```

La revisión de seguridad hace una sola pasada sin editar archivos de la tarea,
crear commits, enviar cambios ni abrir un PR. Los hallazgos, las sugerencias y
las verificaciones pendientes se guardan en `summary.log`. La clasificación de
riesgo recibe los hallazgos sin aplicar. Las demás etapas conservan su comportamiento.

El valor predeterminado es `false`; `null` también significa `false`. Los
hallazgos no detienen las etapas posteriores, pero los errores de ejecución sí.
Consulta los [informes](reports.md).

## Documentos contexto {#supporting-documents}

```json
{
  "documents": ["requirements.md"],
  "googleDrive": {
    "folders": ["Project Documentation"],
    "files": ["https://docs.google.com/document/d/DOCUMENT_ID/edit"]
  }
}
```

Las rutas locales son relativas a `software-factory.json`. Omite las secciones
que no necesites. Para Drive, indica carpetas, URL de documentos o ambos. Los
nombres de carpeta deben identificar una única carpeta accesible; solo se
cargan los Google Docs directamente dentro de ella.

Drive requiere `service_account.json` en la carpeta del proyecto y acceso
compartido para esa cuenta. Mantén las credenciales privadas. Consulta los
[documentos contexto](documents.md) y la [configuración de Google Drive](google-drive.md).

## Transporte de ajustes del sandbox {#sandbox-settings-transport}

Factory envía los ajustes a Agent Sandbox como opciones de la CLI. No crea un
archivo de configuración del sandbox. Las rutas relativas conservan la carpeta
del proyecto como base. Los prompts generados y los controles de etapa de Factory
tienen prioridad sobre los valores configurados.

`apiKey` y `reviewers` se ignoran actualmente en `run` y `resume`. Nunca se
reenvían ni se registran. Usa la autenticación del host.

::: tip Migrar una configuración anterior
Renombra `config.json` a `software-factory.json`; `--config` y `-c` ya no están
disponibles. Mueve los valores de `customSkills` del nivel superior a
`stages.<stage>.customSkill` y las entradas de `disabledStages` a
`stages.<stage>.disabled: true`. Conserva las asignaciones de etapa existentes.
:::

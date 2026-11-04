# Comandos y opciones {#commands-and-flags}

## Comandos {#commands}

| Comando | Función |
| --- | --- |
| `run` | Inicia una tarea nueva con salida de texto. |
| `resume` | Continúa una tarea registrada; requiere `-b`. |
| `run-tui` | Inicia una tarea en la interfaz de terminal. |
| `resume-tui` | Continúa una tarea en la interfaz de terminal; requiere `-b`. |
| `doctor [--json]` | Comprueba dependencias del equipo en `PATH`; puede devolver JSON. |
| `models` | Explora el catálogo público de modelos de OpenRouter en una terminal interactiva. |
| `update` | Instala el binario de la última versión. |
| `worktree-list [--repo <directory>] [--json]` | Lista registros de rama y ruta del repositorio seleccionado; puede devolver JSON. |
| `worktree-editor -b <branch>` | Abre un worktree registrado en VS Code. |
| `worktree-delete -b <branch> [--force]` | Elimina un worktree y su rama local. |
| `worktree-delete-all [--yes]` | Elimina todas las tareas registradas del repositorio actual, incluidos los cambios sin confirmar. |

Codex y Claude Code muestran un [registro compacto de actividad](running.md#read-the-compact-audit-feed)
en los cuatro comandos de ejecución. Los comandos de texto admiten redirección;
los de interfaz de terminal requieren entrada y salida interactivas.

## Salida de doctor {#doctor-output}

`doctor --json` escribe en stdout un arreglo JSON con exactamente `name` (cadena)
e `installed` (booleano) por dependencia, en el orden `git`, `docker`, `gh`, `code`.
Incluye las herramientas ausentes con `installed: false` y termina correctamente
aunque falten. Solo comprueba PATH, sin ejecutar herramientas ni comprobar
servicios. Los errores de salida y uso conservan el manejo normal de errores de
la CLI. Sin `--json`, o con `--json=false`, muestra la tabla de texto con colores.

## Opciones de ejecución {#execution-flags}

Estas opciones se aplican a `run`, `resume` y sus equivalentes con interfaz de terminal:

| Opción | Significado |
| --- | --- |
| `-b`, `--branch` | Rama de la tarea. Se genera en tareas nuevas si se omite; es obligatoria para continuar. |
| `-a`, `--agent` | `codex`, `claude`, `opencode` o `pi`; obligatorio salvo que lo indique el JSON. |
| `--apiKey <key>` | Clave del proveedor para Codex o Claude Code; reemplaza la de la sección JSON. `--apiKey=""` usa la autenticación del equipo. Sin abreviatura. |
| `--reviewers <username>` | Repite por cada usuario GitHub. Reemplaza `run.reviewers`; `--reviewers=""` por sí solo lo vacía. Se valida pero se ignora al continuar. Sin abreviatura. |
| `-m`, `--model` | Reemplaza el modelo seleccionado para el agente. |
| `--effort` | Esfuerzo del agente. Por defecto `high`; el agente comprueba los niveles compatibles. |
| `-q`, `--query` | Instrucciones de la tarea. |
| `-f`, `--filePrompt` | Lee las instrucciones desde un archivo, en lugar de `-q`. |
| `-i`, `--baseImage` | Construye la imagen del sandbox desde una base externa compatible. |
| `--image` | Adjunta una imagen a Codex o Claude Code; repite para varios archivos. |
| `--file` | Monta un archivo regular de solo lectura para cualquier agente; repite para varios. Consulta [archivos de referencia](environment.md#mount-reference-files). |
| `--folder` | Monta un directorio existente de solo lectura para cualquier agente; repite para varios. Solo CLI. Consulta [carpetas de referencia](environment.md#mount-reference-folders). |
| `-p`, `--push` | Pide al agente crear el commit y luego publica desde el equipo. |
| `--pr` | Crea el commit y publica desde el equipo. La ejecución inicial crea o reutiliza un PR; las continuaciones requieren uno abierto y agregan una lista de la sesión. |
| `-c`, `--commitMessage` | Mensaje exacto del commit con `--push` o `--pr`. |
| `--hn` | Comparte la red del equipo con el contenedor. |

Debe proporcionarse exactamente una fuente de prompt mediante opciones o JSON.
Las opciones reemplazan la [configuración](configuration.md), incluso con valores
vacíos explícitos. Usa camelCase `--apiKey`; `--api-key` no se admite.
Las claves usan los [modos de autenticación](authentication.md) existentes y no
se agregan a prompts, registros de worktrees, ayuda ni transcripciones.
Un valor `--apiKey` es visible para otros usuarios y procesos locales en la lista
de procesos durante toda la ejecución; un valor literal también puede guardarse
en el historial del shell. En equipos compartidos, prefiere el `apiKey` de la
sección JSON y permite que solo tú puedas leer `agent-sandbox.json` (por ejemplo,
`chmod 600 agent-sandbox.json`), manteniéndolo fuera de Git (por ejemplo, en
`.gitignore`). Los revisores se recortan, validan y deduplican sin distinguir
mayúsculas; las listas con comas y los valores vacíos mezclados son errores de
uso. Consulta las [reglas de revisores](publishing.md#request-reviewers).

## Ayuda y versión {#help-and-version}

```bash
agent-sandbox --help
agent-sandbox run --help
agent-sandbox --version
```

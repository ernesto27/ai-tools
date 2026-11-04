# Tu primera ejecución {#your-first-run}

Este ejemplo crea una rama, solicita un cambio pequeño al agente y deja el resultado
para que lo revises. Completa primero la [instalación](installation.md) y la
[autenticación](authentication.md).

## 1. Partir de tu proyecto {#_1-start-from-your-project}

```bash
cd /path/to/your/repository
git status
```

Elige la rama desde la que trabajará el agente. El nuevo worktree parte del commit
actual; los archivos sin confirmar de tu copia de trabajo no se copian.
Confirma antes de empezar cualquier cambio que el agente necesite.

## 2. Dar una tarea al agente {#_2-give-the-agent-a-task}

```bash
agent-sandbox run -b improve-readme -a codex \
  -q "Add a short getting-started section to README.md using the project's existing setup commands."
```

Reemplaza `codex` por `claude`, `opencode` o `pi` si usas otro agente.
Elige un nombre de rama que no esté en uso u omite `-b` para generar uno.

La primera ejecución puede tardar más porque la herramienta construye una imagen
Docker e instala los agentes. Cada ejecución necesita acceso a npm para comprobar
la versión del agente seleccionado. Sin `--push` ni `--pr`, el prompt indica al
agente que deje los cambios sin preparar para commit y sin confirmar.

## 3. Buscar y revisar el resultado {#_3-find-and-review-the-result}

```bash
agent-sandbox worktree-list
agent-sandbox worktree-editor -b improve-readme
```

El segundo comando abre el worktree en VS Code. Si usas otro editor, abre la ruta
que muestra `worktree-list`.

Si el agente generó [evidencias de verificación](running.md#review-verification-evidence),
lee `.agent-sandbox-verification/summary.md` en ese worktree antes de decidir
si el resultado está listo.

## 4. Pedir un cambio adicional {#_4-ask-for-a-follow-up}

Desde el directorio original de tu repositorio:

```bash
agent-sandbox resume -b improve-readme -a codex \
  -q "Make the getting-started section shorter and verify the commands against this repository."
```

`resume` inicia otra invocación del agente en el mismo worktree y conserva sus
archivos y commits. No garantiza restaurar la conversación anterior.

Cuando estés conforme, confirma los cambios dentro del worktree e incorpóralos
a tu flujo habitual de Git, o [publica la rama o un PR](publishing.md).
Conserva el worktree hasta haber guardado sus cambios de forma segura.

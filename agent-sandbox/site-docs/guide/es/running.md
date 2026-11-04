# Prompts e interfaz de terminal {#prompts-and-terminal-ui}

## Dar una tarea clara {#give-a-clear-task}

Un prompt útil indica el comportamiento deseado, las restricciones relevantes
y cómo verificar el resultado:

```bash
agent-sandbox run -b fix-login -a codex \
  -q "Fix the login redirect loop. Keep the existing API unchanged, add a regression test, and run the relevant tests."
```

La herramienta agrega instrucciones sobre Git y publicación. También pide al
agente que decida sin hacer preguntas, así que incluye de entrada la información
que necesite.

## Usar un archivo de prompt {#use-a-prompt-file}

Para tareas largas, coloca las instrucciones en un archivo:

```bash
agent-sandbox run -a claude -f prompt.md
```

Proporciona exactamente una fuente: `-q`, `-f` o la configuración JSON
correspondiente. Se rechazan los prompts posicionales y combinar `-q` con `-f`.
Una fuente explícita en la CLI reemplaza la del JSON.

## Elegir agente y modelo {#choose-an-agent-and-model}

```bash
agent-sandbox run -a claude -m sonnet -q "add a regression test for the login redirect"
agent-sandbox run -a opencode -q "update the empty-state copy"
agent-sandbox run -a pi -q "inspect the test failures and fix their cause"
```

`--model` se pasa al agente elegido. Los valores predeterminados actuales son
`gpt-6.1-sol` para Codex y `claude-opus-5-5` para Claude Code. opencode y pi
usan su propia configuración si lo omites. La disponibilidad depende de tu cuenta
del proveedor.

Para explorar modelos por creador, descripción, límite de contexto y precios
de entrada y salida, abre el catálogo público de OpenRouter:

```bash
agent-sandbox models
```

Este explorador requiere entrada y salida de terminal interactivas. No cambia
tu modelo configurado ni concede acceso a proveedores.

## Elegir el esfuerzo {#choose-effort}

Todos los agentes usan `high` por defecto. Cámbialo para una tarea:

```bash
agent-sandbox run -a codex --effort low -q "inspect this repository"
```

También puedes compartir `effort` en la [configuración JSON](configuration.md#set-effort-globally).
El valor se pasa a `model_reasoning_effort` en Codex, `--effort` en Claude,
`--variant` en opencode o `--thinking` en pi. Los valores admitidos dependen del
agente y modelo; el agente informa los valores no compatibles con su código
de salida habitual.

## Leer el registro compacto de actividad {#read-the-compact-audit-feed}

Codex y Claude Code muestran automáticamente un registro compacto en `run`,
`resume`, `run-tui` y `resume-tui`. Agrupa comandos, herramientas, cambios de
archivos y mensajes del agente en entradas con hora.

Por ejemplo, una tarea de Codex puede mostrar:

```text
codex / fix-login
---------------------------------------------
10:32:02  > COMMAND
            rg redirect auth/

10:32:02  + RESULT
            rg redirect auth/
            exit 0 / 0.1s

10:32:04  > TOOL
            Context7 > query-docs

10:32:05  + TOOL
            Context7 > query-docs
            completed / 1.2s

10:32:07  * FILE
            auth/login.go
            patch applied

10:32:12  > COMMAND
            go test ./...

10:32:13  ! FAILED
            go test ./...
            exit 1 / 0.8s
            TestLoginRedirect
            Expected /home
            Received /login

---------------------------------------------
```

El encabezado identifica al agente y la rama real del worktree, incluida la
rama generada si omites `-b`.

| Etiqueta | Significado |
| --- | --- |
| `> COMMAND` | Se inició un comando. |
| `+ RESULT` | Finalizó un comando. Codex incluye el código de salida numérico si está disponible. |
| `> TOOL` / `+ TOOL` | Se inició o completó una herramienta. Las de edición de Claude también empiezan con `> TOOL`. |
| `* FILE` | Se completó un cambio de archivo informado. |
| `! FAILED` | Un comando, herramienta o agente informó un fallo. A continuación aparece la salida disponible del fallo. |
| `AGENT` / `THINKING` | Texto o razonamiento expuesto por el flujo de eventos del agente. |
| `+ SEARCH` / `PLAN` | Actividad de búsqueda o lista de tareas expuesta por Codex. |
| `? EVENT` | Un aviso, como un reintento o un tipo de evento no compatible. |
| `! OUTPUT` | No se pudo decodificar la salida; siguen el diagnóstico y la línea original. |

El registro oculta la salida de las operaciones exitosas y muestra los errores
disponibles. Para los archivos modificados, muestra su ruta y estado.
Abre el worktree para ver los cambios completos.

Las horas corresponden a tu entorno local. La duración aparece cuando se reciben
los avisos de inicio y fin de una operación. Claude muestra estados como
`completed` (completado) o `failed` (fallido). Se ocultan las actualizaciones
parciales del texto y los mensajes finales repetidos.

Una línea marca el final del registro, incluso si cancelas la ejecución.
Revisa el código de salida y los mensajes de publicación para saber cómo terminó
la tarea. Un error puede resolverse en un intento posterior.

## Revisar evidencias de verificación {#review-verification-evidence}

Cuando una sesión produce cambios netos en archivos del proyecto, la herramienta
pide al agente intentar comprobaciones adecuadas y guardar evidencias en
`.agent-sandbox-verification/` dentro del worktree:

| Archivo | Función |
| --- | --- |
| `summary.md` | Resultados, comprobaciones y códigos de salida, rutas de evidencias y limitaciones. |
| `verification.log` | Un registro combinado resumido, cuando existe salida útil. |
| `desktop.png` y `mobile.png` | Capturas finales de la interfaz web a 1280×800 y 390×844. |
| `demo.webm` | Una grabación breve que demuestra los cambios en la interfaz web. |

Las tareas sin interfaz no requieren capturas ni video. Las herramientas o
servicios ausentes deben informarse como verificación no disponible. Son
instrucciones al agente: no se garantizan los archivos ni las comprobaciones
correctas. La verificación fallida o no disponible no bloquea por sí misma la
finalización de la tarea ni la publicación.

Un `.gitignore` local con `*` excluye las evidencias del staging normal.
Al continuar, se pide al agente actualizar las evidencias existentes. Una sesión
sin cambios netos omite la verificación automática y deja intactas las evidencias
anteriores; confirmar trabajo preexistente no cuenta como un cambio nuevo.

Abre el worktree con los [comandos de worktree](worktrees.md#open-and-review)
para revisar las evidencias. La publicación del PR puede incluirlas y luego
eliminar los archivos locales; consulta [verificación en pull requests](publishing.md#verification-in-pull-requests).

## Usar la interfaz de terminal {#use-the-terminal-ui}

Usa `run-tui` o `resume-tui` para una vista de terminal con salida y detalles
de ejecución:

```bash
agent-sandbox run-tui -b fix-login -a codex -q "fix the login redirect loop"
agent-sandbox resume-tui -b fix-login -a codex -q "add a regression test"
```

Estos comandos usan las mismas opciones y configuración `run`/`resume` que sus
equivalentes de texto. Requieren una terminal interactiva para entrada y salida;
usa `run` o `resume` en scripts o al redirigir la salida.

El panel muestra el mismo [registro compacto](#read-the-compact-audit-feed) para
Codex y Claude Code, con colores para actividad, avisos y fallos. La transcripción
guarda ese texto formateado. Los registros suelen estar en
`~/.config/agent-sandbox/logs/`, respetando `XDG_CONFIG_HOME`; contienen el registro
formateado, no los eventos JSON originales. En macOS están en
`~/Library/Application Support/agent-sandbox/logs/`.

| Tecla | Acción |
| --- | --- |
| Tab | Cambiar el foco |
| Flechas, Page Up, Page Down | Desplazarse |
| `q` o Ctrl+C | Cancelar la ejecución y esperar la limpieza |

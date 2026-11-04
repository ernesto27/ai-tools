# Solución de problemas {#troubleshooting}

## El shell no encuentra `agent-sandbox` {#the-shell-cannot-find-agent-sandbox}

El instalador usa `~/.local/bin` por defecto. Agrégalo a `PATH`:

```bash
export PATH="$HOME/.local/bin:$PATH"
agent-sandbox --version
```

Agrega el export al perfil de tu shell para conservarlo entre sesiones.

## Docker está instalado, pero la ejecución falla {#docker-is-installed-but-the-run-fails}

```bash
agent-sandbox doctor
docker info
```

`doctor` solo comprueba la disponibilidad de ejecutables. Si `docker info` falla,
inicia Docker o corrige el acceso de tu usuario siguiendo las instrucciones de
tu instalación de Docker.

## Faltan rutas de autenticación {#authentication-paths-are-missing}

Ejecuta e inicia sesión con tu agente en el equipo primero. Comprueba todas las
rutas de la [página de autenticación](authentication.md), incluida `~/.claude.json`
para Claude Code. No crees archivos vacíos para saltarte la comprobación.
Para Codex o Claude, también puedes usar una clave de API.

## La primera ejecución es lenta o falla la preparación de la imagen {#the-first-run-is-slow-or-image-preparation-fails}

La primera ejecución puede construir una imagen. Cada ejecución consulta npm y
una actualización del agente puede provocar una reconstrucción. Comprueba la
red, la salida de construcción de Docker y los repositorios de paquetes de la
imagen base. Si tu proyecto necesita Go u otro entorno de ejecución, elige una
[imagen base externa](environment.md).

## El comando rechaza mi prompt o JSON {#the-command-rejects-my-prompt-or-json}

Usa `-q "tarea"` o `-f prompt.md`, no texto posicional, y no pases ambos.
Comprueba que el JSON use campos y tipos admitidos, sin valores `null`.
Se validan tanto la sección `run` como `resume`.

Ejecuta desde el directorio que contiene el `agent-sandbox.json` deseado; no se
busca en directorios superiores. Si el JSON activa la publicación, usa
`--push=false --pr=false` para dejar los cambios sin confirmar.

## La rama ya existe {#the-branch-already-exists}

Usa otro nombre para una tarea nueva u omite `-b`. Para una tarea registrada,
usa `resume` en lugar de `run`:

```bash
agent-sandbox worktree-list
agent-sandbox resume -b fix-login -a codex -q "continue the fix"
```

## No se encuentra mi tarea al continuar o aparece ocupada {#a-resume-cannot-find-my-task-or-says-it-is-busy}

Ejecuta desde el mismo repositorio y usa la rama que muestra `worktree-list`.
Solo se pueden continuar worktrees registrados del sandbox. Si está ocupado,
espera a que termine su sesión o cancélala y espera la limpieza.

## La interfaz de terminal falla o desaparece {#the-terminal-ui-fails-or-disappears}

Usa una terminal interactiva para `run-tui` y `resume-tui`. Para redirección o
scripts, usa `run` o `resume`. Revisa la ruta del registro en el mensaje de error;
las transcripciones suelen estar en `~/.config/agent-sandbox/logs/`, respetando
`XDG_CONFIG_HOME`.

## El registro muestra `! OUTPUT` o `? EVENT` {#the-feed-shows-output-or-event}

Codex y Claude Code usan el [registro compacto de actividad](running.md#read-the-compact-audit-feed).
`! OUTPUT` incluye un diagnóstico de decodificación y la línea original.
`? EVENT` puede indicar un evento no compatible, un reintento de API o un aviso
del servidor MCP. Ninguna etiqueta determina por sí sola si la tarea terminó
correctamente; comprueba el código de salida final y los errores asociados.

Para una ejecución TUI, revisa su transcripción en
`~/.config/agent-sandbox/logs/`, respetando `XDG_CONFIG_HOME`. El error o resumen
de salida fallida muestra la ruta del registro. La transcripción contiene
entradas formateadas, no el flujo JSON original. Para ejecuciones de texto,
captura stdout y stderr si necesitas un registro:

```bash
agent-sandbox run -a codex -q "inspect the failing tests" > sandbox.log 2>&1
```

## Falla el push o la creación del PR {#push-or-pr-creation-fails}

Comprueba el código de salida del agente e inspecciona primero el worktree.
Publicar requiere cambios confirmados y un worktree limpio; se omite si el agente
falla. Pide que termine con `resume --push` o `resume --pr`.

Para PR, comprueba `gh auth status`, las URL de origin y los permisos de push.
La base registrada debe existir en `origin`; las tareas que parten de HEAD
separado y los registros antiguos sin base no pueden publicar PR. Sin diferencias
con la base no se crea un PR.

Si el error indica que el push tuvo éxito pero falló la creación del PR, la rama
remota ya está publicada. Corrige el problema informado y reintenta con
`resume --pr`. Los revisores se solicitan en PR nuevos de `run`; resuelve por
separado en GitHub las solicitudes ausentes si la creación tuvo éxito parcial.

Si el push tuvo éxito pero falló la actualización de la descripción, los cambios
confirmados ya están en el remoto. La lista fallida no se guarda ni se vuelve a
aplicar en la siguiente continuación; recupera manualmente en GitHub el texto faltante.

## No puedo eliminar un worktree {#i-cannot-delete-a-worktree}

Ejecuta desde el repositorio original, fuera del worktree que vas a eliminar.
Espera a que termine cualquier sesión que lo use. Revisa o guarda los cambios
sin confirmar antes de reintentar. Usa `--force` solo si quieres descartarlos;
la eliminación también borra la rama local.

## Informar un problema {#report-an-issue}

Incluye la versión de la CLI, el agente elegido, el comando sin secretos,
el código de salida y los errores relevantes en un
[issue de GitHub](https://github.com/ernesto27/ai-tools/issues).
Revisa los registros y elimina claves, código privado e información de cuenta
antes de compartirlos.

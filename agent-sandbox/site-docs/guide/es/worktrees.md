# Worktrees y continuación de tareas {#worktrees-and-continuing-work}

Un worktree es un segundo directorio de trabajo de tu repositorio. Cada tarea
tiene su propia rama y archivos, así que puedes trabajar en tu directorio original
mientras el agente edita otro.

## Buscar una tarea {#find-a-task}

Ejecuta desde cualquier directorio dentro del repositorio original:

```bash
agent-sandbox worktree-list
```

Solo enumera worktrees registrados por `agent-sandbox` para ese repositorio.
No gestiona los que hayas creado manualmente con Git.

En Linux, los worktrees suelen estar bajo:

```text
~/.config/agent-sandbox/worktrees/<repo>-<hash>/<branch>
```

Para seleccionar un repositorio explícitamente o usar la lista en un script:

```bash
agent-sandbox worktree-list --repo /path/to/repository --json
```

La salida JSON es un arreglo de objetos que contienen solo `branch` y `path`:

```json
[{"branch":"fix-login","path":"/home/user/.config/agent-sandbox/worktrees/repo-hash/fix-login"}]
```

Tanto el texto como el JSON usan el orden inverso de registro, primero el más
reciente. Incluyen registros obsoletos sin comprobar las rutas. Sin coincidencias,
JSON devuelve `[]`. Los fallos conservan los diagnósticos en stderr y un código
de salida distinto de cero. `--repo` solo se aplica a la lista; los otros comandos
seleccionan el repositorio desde el directorio actual.

Las rutas de worktrees y registros respetan `XDG_CONFIG_HOME` en Linux.
Los worktrees comparten metadatos Git con el repositorio original; proporcionan
archivos y ramas separados, no un aislamiento completo del repositorio.

En macOS, suelen estar bajo:

```text
~/Library/Application Support/agent-sandbox/worktrees/<repo>-<hash>/<branch>
```

Usa la ruta que muestra `worktree-list` para abrir un worktree; pon entre comillas
las rutas con espacios en los comandos del shell.

## Abrir y revisar {#open-and-review}

```bash
agent-sandbox worktree-editor -b fix-login
```

Requiere el comando `code` de VS Code en `PATH`. Con otro editor, abre directamente
el directorio indicado. Inspecciona los archivos y el diff y luego ejecuta allí
las comprobaciones del proyecto.

## Continuar una tarea {#continue-a-task}

```bash
agent-sandbox resume -b fix-login -a codex -q "add a regression test for the fix"
```

`resume` requiere una rama registrada para el repositorio actual y conserva el
trabajo confirmado y sin confirmar. Puedes elegir otro agente en la siguiente
invocación. Las credenciales y opciones se resuelven de nuevo; se usa la sección
JSON `resume` en lugar de `run`.

Solo una sesión puede ocupar un worktree a la vez. Espera a que termine o
cancélala antes de continuar o eliminar esa tarea.

## Eliminar una tarea terminada {#remove-a-completed-task}

Ejecuta los comandos de eliminación desde tu repositorio original, fuera del
worktree que vas a borrar:

```bash
agent-sandbox worktree-delete -b fix-login
```

Esto elimina el worktree **y su rama local**. Rechaza cambios sin confirmar a
menos que pases `--force`. Conserva o fusiona el trabajo antes de eliminar la
rama, aunque el worktree esté limpio.

::: warning La eliminación descarta trabajo
`worktree-delete -b fix-login --force` también descarta cambios sin confirmar.
`worktree-delete-all` elimina todos los worktrees registrados del repositorio
actual, sus ramas y sus cambios sin confirmar tras pedir confirmación.
`worktree-delete-all --yes` omite esa confirmación.
:::

La eliminación no borra ramas remotas ni PR de GitHub.

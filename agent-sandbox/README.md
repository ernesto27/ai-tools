# agent-sandbox

`agent-sandbox` ejecuta Codex, Claude Code, opencode o pi dentro de un
contenedor Docker y sobre un *worktree* de Git propio. El agente trabaja en un
branch y directorio separados, sin modificar la copia de trabajo desde la que
se invocó el comando.

## Instalación

Las releases se publican para Linux x86_64. Instala la más reciente con:

```bash
curl -fsSL https://raw.githubusercontent.com/ernesto27/ai-tools/master/agent-sandbox/install.sh | bash
```

El instalador descarga la última release de `agent-sandbox`, verifica su checksum
SHA-256 y deja el binario en `~/.local/bin`. Si ese directorio todavía no está
en `PATH`, el instalador indica la línea que hay que agregar al perfil de la shell.
Las releases se crean con tags `agent-sandbox-v*`.

Para compilarlo desde este directorio:

```bash
go build -o agent-sandbox ./cmd/agent-sandbox
```

## Requisitos

- Docker en ejecución.
- Git y un repositorio Git: ejecutá el comando desde cualquier directorio
  dentro del repositorio.
- Una sesión ya autenticada del agente elegido en el host o, para Codex y
  Claude Code, una clave API en `./agent-sandbox.json`. opencode y pi requieren
  la sesión del host.

| Agente | Configuración del host cuando no se usa `api-key` |
| --- | --- |
| Codex | `~/.codex/` |
| Claude Code | `~/.claude/` y `~/.claude.json` |
| opencode | `~/.config/opencode/`, `~/.local/share/opencode/` y `~/.local/state/opencode/` |
| pi | `~/.pi/agent/` |

Sin `api-key`, si falta una de esas rutas, el comando falla sin crearla.
Ejecutá y autenticá primero el agente correspondiente en el host. Con una
clave API para Codex o Claude Code, el contenedor usa un directorio de inicio
temporal y no monta las credenciales del host.

En cada ejecución, `agent-sandbox` comprueba la versión del agente seleccionado
dentro de la imagen contra npm. Construye la imagen si no existe y la reconstruye
si encuentra una versión más reciente; por eso la primera ejecución puede tardar
y necesita acceso a npm.

## Uso

Para listar las dependencias del host:

```bash
agent-sandbox doctor
```

Comprueba si `git`, `docker`, `gh` (opcional, para PRs) y `code` (opcional,
para `worktree-editor`) están en `PATH`.

```text
agent-sandbox run [-b <branch>] -a <codex|claude|opencode|pi> [-m <modelo>] [-i <imagen>] [--image <archivo>]... [--hn] [-p] [--pr] [-c <mensaje-commit>] (-q <consulta> | -f <archivo-prompt>)
agent-sandbox resume -b <branch> -a <codex|claude|opencode|pi> [opciones] (-q <consulta> | -f <archivo-prompt>)
```
| Parámetro | Descripción |
| --- | --- |
| `-b`, `--branch` | Opcional. Branch para el worktree aislado; si se omite, se genera uno. |
| `-a`, `--agent` | Obligatorio. Uno de `codex`, `claude`, `opencode` o `pi`. |
| `-m`, `--model` | Opcional. Sobrescribe el modelo que resuelve el agente. |
| `-i`, `--base-image` | Opcional. Deriva una imagen desde una base compatible, para disponer de su toolchain dentro del sandbox. |
| `--hn` | Opcional. Comparte la red del host con el contenedor, sin limitar puertos. Desactivado por defecto; ver los riesgos en "Red del host". |
| `-p`, `--push` | El agente crea el commit dentro del contenedor. Si termina correctamente y deja el worktree limpio, se hace `git push --set-upstream origin <branch>` sin crear un PR. |
| `--pr` | Opcional. El agente commitea dentro del contenedor; si termina correctamente y deja el worktree limpio, se publica el branch en `origin` y se crea o reutiliza un pull request de GitHub. No requiere `--push`; ver "Pull requests de GitHub". |
| `-q`, `--query` | Instrucción para el agente. |
| `-c`, `--commit-message` | Opcional. Con `--push` o `--pr`, se le indica al agente que use este mensaje exacto en el commit dentro del contenedor. |
| `-f`, `--file-prompt` | Archivo cuyo contenido se usa como instrucción para el agente, en lugar de `-q` o `--query`. |
| `--image <archivo>` | Opcional y repetible. Adjunta imágenes al prompt inicial de Codex o Claude Code. Cada ruta debe ser un archivo regular existente en el host; opencode y pi la ignoran. Usá `--` antes del prompt de texto para que Codex no lo interprete como otra imagen. |

Sin `-p`/`--push` ni `--pr`, los cambios quedan sin commitear en el worktree.
Con cualquiera de esas opciones, el agente commitea antes de terminar. Hay que
proporcionar exactamente una fuente de prompt: `-q`/`--query`, `-f`/`--file-prompt`
o el valor `"query"`/`"file-prompt"` del JSON. No se pueden usar ambas fuentes
a la vez ni se aceptan instrucciones posicionales. Sin `--commit-message`,
el agente elige un mensaje basado en los cambios reales.

Para continuar un worktree registrado, usá `agent-sandbox resume -b <branch>`.
El branch es el nombre mostrado por `worktree-list`. Si se combina con
`--push` o `--pr`, el agente debe commitear los cambios antes de terminar la sesión.

`run` y `resume` leen `./agent-sandbox.json` si existe en el directorio desde
el que se ejecutan. En la raíz del JSON se admiten `agent`, `model`,
`base-image`, `push`, `pr` y `hn` como valores compartidos por ambos comandos.
Cada sección admite los nombres largos de las opciones
`branch`, `agent`, `model`, `base-image`, `query`, `push`, `pr`, `hn`, `commit-message`,
`file-prompt` e `image`, además del campo `api-key`. `api-key` solo existe en
el JSON: no hay una opción de línea de comandos equivalente. Para cada campo,
prevalece la opción explícita de la línea de comandos, luego el valor de la
sección y finalmente el valor de la raíz. Un `false` o una cadena vacía en la
sección también reemplaza el valor compartido.

```json
{
  "agent": "codex",
  "model": "gpt-6.1-sol",
  "base-image": "golang:1.26-alpine",
  "push": false,
  "pr": false,
  "hn": false,
  "run": {
    "model": "gpt-5.6-sol",
    "query": "run go version and do not change any files",
    "push": false
  },
  "resume": {
    "branch": "fix-login",
    "query": "add a regression test"
  }
}
```

Con este archivo, `agent-sandbox run` usa la sección `run`; `resume` usa la
sección `resume`. También podés pasar `-q "otra tarea"` para reemplazar la
consulta del JSON. Los comandos de gestión de worktrees no leen el archivo.

### Pull requests de GitHub

`--pr` está disponible en `run` y `resume`, y también se puede activar con
`"pr": true` en la sección correspondiente de `agent-sandbox.json`. Requiere
GitHub CLI (`gh`) instalado y autenticado en el host para el servidor de
`origin`, además de permisos para hacer push y crear el PR. Las URLs de fetch
y push de `origin` deben identificar el mismo repositorio de GitHub y debe
haber un único destino de push. Con `--push` y `--pr`, el commit se hace dentro del contenedor;
el push y las operaciones de GitHub se ejecutan en el host. Para `--pr`,
el contenedor recibe acceso de escritura a los metadatos Git del repositorio
y usa la identidad Git configurada en el host.

La base del PR es el branch desde el que se creó el worktree con `run` y queda
registrada para futuros `resume`. Ese branch debe existir en `origin` y ser
distinto del branch del worktree. No se admite `--pr` al iniciar desde un HEAD
separado (*detached HEAD*) ni al continuar registros antiguos sin
`base_branch`; no se elige otra base automáticamente.

Crear un worktree y publicar sus cambios como PR:

```bash
agent-sandbox run -b fix-login -a codex --pr -q "fix the login redirect loop"
```

Continuar ese worktree y actualizar el branch del PR:

```bash
agent-sandbox resume -b fix-login -a codex --pr -q "add a regression test for the login redirect"
```

Con `--push` o `--pr`, si el agente termina con estado distinto de cero, se omite la publicación y
los cambios quedan en el worktree. Si termina correctamente pero deja cambios
sin commitear, se informa un error y no se hace push. Con el worktree limpio,
se compara el resultado con la base actual de `origin`.
Sin diferencias para revisar, no se hace push ni se crea un PR. Un worktree
limpio con commits que aportan diferencias respecto de la base también se
puede publicar.

Después del push, si ya existe un PR abierto para el mismo repositorio, branch
y base, se muestra su URL y se conserva su título, descripción y estado de
borrador. Para un PR nuevo, una invocación adicional del agente elegido genera
el título y la descripción a partir de la comparación completa; luego se crea
el PR listo para revisión, sin marcarlo como borrador. Esa invocación también
consume uso del modelo. Si falla la generación o creación del PR, el branch
ya quedó publicado; podés reintentar con `resume --pr`.

Combinar `--pr` con `--push` sigue este mismo flujo, sin duplicar el commit ni
el push. `--commit-message` controla el mensaje indicado al agente para el
commit, no el título del PR.

### Red del host

`--hn` activa el modo de red `host` de Docker para el contenedor del agente.
Está desactivado por defecto y también se puede configurar con `"hn": true`
en `run` o `resume` dentro de `agent-sandbox.json`. Para desactivar un valor
habilitado en el JSON, pasá `--hn=false`.

Este modo comparte la red del host y reduce el aislamiento del contenedor:
el agente puede acceder a servicios locales del host, sin una lista de puertos
permitidos. Usalo cuando la tarea necesite ese acceso. Con `--pr`, la
invocación adicional que genera el título y la descripción usa la misma
configuración de red.

### Claves API para Codex y Claude Code

Para usar una clave API en lugar de la sesión del host, agregá `api-key` a la
sección `run` o `resume` que vayas a ejecutar. Por ejemplo, para Codex:

```json
{
  "run": {
    "agent": "codex",
    "api-key": "<CLAVE_API_DE_OPENAI>",
    "query": "inspect this repository"
  }
}
```

Para Claude Code:

```json
{
  "run": {
    "agent": "claude",
    "api-key": "<CLAVE_API_DE_ANTHROPIC>",
    "query": "inspect this repository"
  }
}
```

### Imagen base externa

`--base-image` usa una imagen que ya trae el runtime del proyecto. Se admiten
Alpine, Debian, Ubuntu, Fedora, RHEL 8/9, UBI 8/9 y Amazon Linux 2023. Por
ejemplo, `golang:1.26-alpine` deja disponibles Go, `gofmt` y `go test` dentro
del contenedor:

```bash
agent-sandbox run -b fix-go-tests -a codex -i golang:1.26-alpine -q "run gofmt and go test ./..., then fix failures"
```

La primera ejecución crea una imagen local derivada e instala lo necesario para
ejecutar los agentes: Node.js, npm, Bash, Codex, Claude Code, opencode, pi,
ripgrep, certificados CA, curl y Git. Las siguientes reutilizan esa imagen para
la misma base y comprueban en npm la versión del agente seleccionado. Si difiere
de la instalada, reconstruyen la imagen con esa versión y conservan la referencia
de la base elegida.

La base debe ofrecer `apk` (Alpine), `apt-get` (Debian/Ubuntu), `dnf`
(Fedora/RHEL/UBI/Amazon Linux) o `microdnf` (UBI minimal). Las demás fallan
durante el build. Alpine instala Node.js desde sus paquetes; las otras bases
usan Node.js 22 de NodeSource.

En RHEL, la imagen debe tener repositorios habilitados y, si corresponde, una
suscripción válida: el sandbox no monta credenciales del host. No habilita
EPEL ni CRB/CodeReady Builder, ni soporta `yum`; si falta un paquete como
`ripgrep`, el build falla con el error del gestor de paquetes.


```bash
docker image ls 'agent-sandbox-base-*'
docker image rm <IMAGE_ID>
```

### Modelos

Al omitir `--model`, Codex usa `gpt-6.1-sol` y Claude Code usa `claude-opus-5-5`.
opencode y pi dejan que su propia configuración elija el modelo. Los valores de
`--model` se pasan directamente al agente: por ejemplo, `gpt-5.6-sol` para
Codex, `sonnet` para Claude Code y `proveedor/modelo` para opencode o pi.

## Ejemplos

Crear un worktree para Codex y dejar sus cambios listos para revisar:

```bash
agent-sandbox run -a codex -m gpt-5.6-sol -q "fix the login redirect loop"
```

Ejecutar Claude Code y publicar el branch al terminar:

```bash
agent-sandbox run -b add-test -a claude -m sonnet -p -q "add a regression test for the login redirect"
```

Continuar un worktree registrado por su nombre:

```bash
agent-sandbox resume -b fix-login -a codex -q "add a regression test for the login redirect"
```

Dejar que opencode resuelva su modelo configurado:

```bash
agent-sandbox run -b update-copy -a opencode -q "update the empty-state copy"
```

Ejecutar Codex con imagen de golang

```bash
agent-sandbox run -b fix-go-tests -a codex -i golang:1.26-alpine -q "run go test ./... and fix failures"
```

Obtener prompt de archivo
```bash
agent-sandbox run -b prompt-file-test -a codex -f prompt.md
```

Adjuntar una o más imágenes al prompt inicial de Codex o Claude Code
```bash
agent-sandbox run -a claude \
  --image "/home/user/Pictures/mockup.png" \
  --image "/home/user/Pictures/reference.png" \
  -q "compare these screenshots and implement the resulting UI"
```

## Gestionar worktrees creados por el sandbox

El comando crea los worktrees en
`~/.config/agent-sandbox/worktrees/<repo>-<hash>/<branch>` y registra los que
creó en `~/.config/agent-sandbox/worktrees.jsonl`. El identificador del
repositorio evita que branches con el mismo nombre en repositorios distintos
colisionen. Los siguientes subcomandos solo ven y eliminan esas entradas; no
afectan worktrees creados manualmente.

```bash
# Mostrar los worktrees registrados para el repositorio actual.
agent-sandbox worktree-list

# Abrir un worktree registrado en VS Code.
agent-sandbox worktree-editor -b fix-login

# Eliminar un worktree limpio y su branch.
agent-sandbox worktree-delete --branch fix-login

# Permitir eliminar también sus cambios sin commitear.
agent-sandbox worktree-delete -b fix-login --force

# Mostrar todos, pedir confirmación y eliminarlos junto con sus branches.
agent-sandbox worktree-delete-all

# Omitir la confirmación de la eliminación masiva.
agent-sandbox worktree-delete-all --yes
```

`worktree-delete` se niega a descartar cambios sin `--force`.
`worktree-delete-all` siempre elimina los cambios sin commitear después de la
confirmación (o inmediatamente con `--yes`). No ejecutes los comandos de
eliminación desde el worktree que querés borrar.
`worktree-editor` requiere `-b` y abre en vscode el branch worktree

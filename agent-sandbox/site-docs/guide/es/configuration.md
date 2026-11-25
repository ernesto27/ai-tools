# Configuración {#configuration}

`run`, `resume`, `run-tui` y `resume-tui` leen `./agent-sandbox.json` desde el
directorio donde ejecutas el comando. No lo buscan en directorios superiores.
Los comandos de gestión de worktrees no leen este archivo.

## Guardar valores predeterminados para tu proyecto {#save-defaults-for-your-project}

```json
{
  "branch": "fix-go-tests",
  "agent": "codex",
  "effort": "high",
  "baseImage": "golang:1.26-alpine",
  "push": false,
  "pr": false,
  "run": {
    "query": "run go test ./... and fix failures"
  },
  "resume": {
    "query": "add a regression test for the fix"
  }
}
```

Con esta configuración, `agent-sandbox run` usa la rama compartida `fix-go-tests`
y el agente, la imagen y el prompt guardados. `agent-sandbox resume` continúa
esa rama usando su propia sección. El campo `branch` de una sección o una opción
explícita `-b` / `--branch` reemplaza la rama compartida.

## Entender la prioridad {#understand-precedence}

Los valores se resuelven en este orden, de mayor a menor prioridad:

1. Opciones explícitas de la CLI.
2. La sección `run` o `resume`.
3. Valores compartidos en la raíz.
4. Valores predeterminados integrados.

Un valor `false` o una cadena vacía de una sección también reemplaza el valor
compartido. Por ejemplo:

```bash
agent-sandbox run --pr=false -q "inspect the failing tests"
```

Esto desactiva `pr` aunque el JSON lo active. Agrega `--push=false` si tu
configuración activa `push` y quieres dejar los cambios sin confirmar.

## Campos admitidos {#supported-fields}

Las claves JSON y opciones largas con varias palabras usan camelCase:
`baseImage`, `commitMessage`, `filePrompt` y `apiKey`. Por ejemplo, usa
`--baseImage` y `--apiKey` en la CLI.
Cambia los nombres anteriores `base-image`, `commit-message`, `file-prompt` y
`api-key` en tus archivos y comandos; se rechazan y no tienen alias.
Las opciones cortas (`-i`, `-c`, `-f`) y los nombres `hn` y `pr` no cambian.

| Ubicación | Campos |
| --- | --- |
| Raíz compartida | `branch`, `agent`, `model`, `effort`, `baseImage`, `push`, `pr`, `hn`, `file`, `cpus`, `memory`, `timeout` |
| `run` y `resume` | `branch`, `agent`, `model`, `effort`, `baseImage`, `query`, `filePrompt`, `push`, `pr`, `hn`, `commitMessage`, `image`, `file`, `apiKey`, `cpus`, `memory`, `timeout` |
| Solo `run` | `reviewers` |

Usa cadenas para los campos de texto, booleanos para `push`, `pr` y `hn`, y
arreglos de cadenas para `filePrompt`, `image`, `file` y `reviewers`. Se rechazan los campos
desconocidos, `null` y los tipos incorrectos. Ambas secciones se validan aunque
solo uses una, incluso si las opciones de la CLI reemplazan valores inválidos.

Los cuatro comandos de ejecución aceptan `--apiKey <key>` y la opción repetible
`--reviewers <username>`, sin abreviaturas. La clave de la CLI reemplaza la de
la sección; `--apiKey=""` selecciona la autenticación del equipo y sus montajes
requeridos. Solo Codex y Claude Code admiten claves no vacías. Los valores de las
opciones son visibles en la lista de procesos del equipo; en equipos compartidos,
prefiere `apiKey` en JSON y permite que solo tú puedas leer `agent-sandbox.json`
(por ejemplo, `chmod 600 agent-sandbox.json`), manteniéndolo fuera de Git
(por ejemplo, en `.gitignore`). Consulta [autenticación](authentication.md).

En `run` y `run-tui`, los revisores de la CLI reemplazan toda la lista JSON;
si omites las opciones se usa `run.reviewers`. `--reviewers=""` por sí solo la
vacía. Otras entradas vacías o con solo espacios, incluido mezclar un valor vacío
con usuarios, son errores de uso. Se quitan los espacios de los extremos y los
duplicados sin distinguir mayúsculas, conservando el orden y la escritura de la
primera aparición. Solo se aceptan nombres de usuario de GitHub simples; consulta
[publicación](publishing.md#request-reviewers).

En `resume` y `resume-tui` se valida la sintaxis de los revisores y luego se
ignoran los valores válidos. Se conservan los revisores existentes. El esquema
JSON no cambia: `apiKey` y `reviewers` en la raíz y `resume.reviewers` siguen
siendo inválidos.

## Límites de recursos {#resource-limits}

Puedes fijar límites independientes de CPU, memoria y tiempo de ejecución en
cualquiera de los cuatro comandos de ejecución.

```bash
agent-sandbox run -b limited-task -a codex --cpus 1.5 --memory 2GiB --timeout 30m -q "inspecciona este repositorio"
```

En `agent-sandbox.json`, usa un número para los núcleos de CPU y cadenas para
el tamaño de memoria y la duración.

```json
{
  "cpus": 1.5,
  "memory": "2GiB",
  "timeout": "30m",
  "resume": { "timeout": "0" }
}
```

El límite mínimo de CPU es `0.01` núcleos y el de memoria es `6MiB`. La memoria acepta tamaños enteros
en `B`, `KiB`, `MiB`, `GiB` y `TiB`. La duración acepta unidades de Go
como `s`, `m` y `h`. Para cada valor, la CLI tiene prioridad sobre la sección
del comando y la raíz compartida. Para quitar un límite heredado, pasa `0` en
su opción o usa `0` para CPU y `"0"` para memoria o duración en el JSON.
Todos los límites son opcionales.

El temporizador comienza cuando Docker inicia el contenedor del agente, después
de preparar la imagen y completar la configuración. Al vencer el plazo,
agent-sandbox solicita la detención inmediata, espera la limpieza del contenedor
y termina con el código `124`. Los cambios permanecen en el worktree para
revisarlos o reanudar la sesión. El equipo omite el push y la publicación del PR.

## Compartir archivos de referencia {#share-reference-files}

Usa `file` para montar archivos de referencia para cualquier agente:

```json
{
  "file": ["/path/to/spec.pdf", "/path/to/sample.csv"],
  "resume": {
    "file": []
  }
}
```

Una lista explícita de `--file` reemplaza el arreglo de la sección, que reemplaza
el arreglo de la raíz. Los arreglos no se combinan; un arreglo vacío en una
sección elimina los archivos de la raíz. Las listas `--image` de la CLI también
reemplazan los arreglos de su sección. Las rutas relativas se resuelven desde
el directorio de ejecución. Consulta [archivos de referencia](environment.md#mount-reference-files)
para ver las rutas del contenedor y las herramientas PDF.

Para directorios de referencia usa opciones `--folder` repetidas; JSON no acepta
un campo `folder`. Consulta [carpetas de referencia](environment.md#mount-reference-folders).

## Configurar el esfuerzo global {#set-effort-globally}

Define la cadena `effort` en la raíz para compartir un valor predeterminado entre
todos los agentes y los cuatro comandos de ejecución:

```json
{
  "effort": "low"
}
```

Un `--effort` explícito reemplaza el valor de la sección, que reemplaza el de la
raíz. Se quitan los espacios de los extremos. Un valor omitido, vacío o formado
solo por espacios se resuelve como `high`. Los valores se pasan sin validar los
niveles permitidos. El agente informa si el valor no es compatible con su modelo.


## Combinar archivos de prompt {#compose-a-prompt-from-files}

```json
{"run": {"filePrompt": ["context.md", "task.md"]}}
```

`run.filePrompt` y `resume.filePrompt` requieren arreglos. Reemplaza las cadenas
anteriores por arreglos de un elemento. Se rechazan cadenas, `null`, elementos
que no sean cadenas y rutas vacías, incluso si la CLI reemplaza el campo.
`[]` no proporciona un prompt de archivo, por lo que se necesita una consulta.
Los archivos vacíos que se puedan leer son válidos.

Repite `-f` / `--filePrompt` en cualquiera de los cuatro comandos de ejecución.
Las rutas de la CLI reemplazan todo el arreglo JSON. Una consulta explícita de
la CLI reemplaza el prompt de archivo JSON y viceversa. Fuera de esos reemplazos,
no se pueden combinar consultas y archivos de prompt.

Las rutas son relativas al directorio de invocación. Se conservan el contenido
exacto, el orden y los duplicados, con dos caracteres de salto de línea (`\n\n`)
entre archivos y sin encabezados. Un archivo faltante o ilegible detiene la
ejecución antes de iniciar el agente.

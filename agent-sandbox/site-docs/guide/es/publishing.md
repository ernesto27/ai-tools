# Push y pull requests {#push-and-pull-requests}

Elige qué debe pasar cuando el agente termine:

| Modo | Resultado |
| --- | --- |
| Sin opciones de publicación | Se indica al agente que deje los cambios sin confirmar en su worktree. |
| `--push` / `-p` | El agente crea el commit; el equipo publica la rama en `origin`. |
| `--pr` | El agente crea el commit y prepara el texto del PR; el equipo publica y crea o reutiliza un PR de GitHub. |

En ambos modos de publicación, configura tu identidad de Git en el equipo y
comprueba que puedes publicar en `origin`. El agente usa esa identidad dentro
del contenedor. El push y las operaciones de GitHub se ejecutan en el equipo.

## Publicar una rama {#push-a-branch}

```bash
agent-sandbox run -b add-test -a claude --push \
  -q "add a regression test for the login redirect"
```

Agrega `-c "Fix login redirect"` para indicar el mensaje exacto del commit.
De lo contrario, el agente elige uno a partir de sus cambios reales.

## Crear un pull request de GitHub {#create-a-github-pull-request}

Instala GitHub CLI e inicia sesión en tu entorno local para el servidor usado por
`origin`. Para github.com:

```bash
gh auth login
gh auth status
```

Parte de una rama local con nombre que exista en `origin`. Su nombre se guarda
como base del PR y debe diferir de la rama de la tarea. Las URL de fetch y push
de `origin` deben identificar el mismo repositorio GitHub, con un único destino
de push.

```bash
agent-sandbox run -b fix-login -a codex --pr \
  -q "fix the login redirect loop and add a regression test"
```

`--pr` incluye el push; no necesitas `--push`. El agente prepara el título y el
cuerpo en la misma sesión usando la comparación completa entre ramas. Un PR
nuevo se crea listo para revisión, no como borrador.
Para publicar, el agente debe terminar correctamente y el worktree debe estar
limpio. Si no hay diferencias revisables con la base, la ejecución inicial
omite tanto el push como la creación del PR.

Si ya existe un PR abierto para el mismo repositorio, rama de origen y base,
se muestra su URL y se conservan el título, el cuerpo y el estado de borrador.

## Actualizar el PR {#update-the-pr}

```bash
agent-sandbox resume -b fix-login -a codex --pr \
  -q "address the review feedback and run the relevant tests"
```

La base original se conserva en las continuaciones. No se admite `--pr` si la
tarea partió de un HEAD separado o si el registro antiguo no tiene una base guardada.

`resume --pr` requiere un PR abierto para la rama y la base registradas.
Si el agente termina correctamente y deja el worktree limpio, publica y agrega
a la descripción actual una lista de comprobación de los cambios confirmados de
esta sesión. Conserva el texto anterior, el título, el estado de borrador y los
revisores. Si el agente informa que no hubo cambios netos en la sesión, publica
sin editar la descripción. `resume-tui --pr` se comporta igual.

Si falla la creación del PR o la actualización de la descripción después del
push, la rama queda publicada y el comando informa un error. Las actualizaciones
fallidas no se guardan ni se vuelven a aplicar en la siguiente continuación.
Consulta [recuperación de la publicación](troubleshooting.md#push-or-pr-creation-fails).

## Verificación en los pull requests {#verification-in-pull-requests}

Al crear un PR o agregar la lista de una sesión continuada, el equipo recopila
los archivos disponibles en `.agent-sandbox-verification/` del worktree.
Los archivos de texto (`.txt`, `.log`, `.md`, `.json`) se incluyen en secciones
desplegables de la descripción. Las imágenes y los videos compatibles se pasan
a GitHub CLI con `--attach`. Se excluyen los archivos y directorios ocultos,
los scripts temporales y los tipos de archivo no compatibles.

Tras crear el PR o actualizar correctamente su descripción, el equipo elimina
las evidencias locales y conserva `.gitignore`. Una ejecución inicial que
reutiliza un PR existente no modifica su descripción ni las evidencias locales.
Una continuación sin cambios netos tampoco modifica ninguno de los dos.
Las ejecuciones que solo hacen push no publican ni eliminan evidencias.

La ausencia del directorio de evidencias no impide publicar. Los errores al leer
o adjuntar evidencias pueden hacer fallar la publicación después del push.
Se pide al agente que mantenga el texto lo bastante breve para respetar el límite
de 65.536 caracteres de la descripción del PR. Revisa el
[resumen de verificación](running.md#review-verification-evidence) para conocer
las comprobaciones realizadas y sus limitaciones; que el agente termine
correctamente no garantiza que hayan pasado sus pruebas.

## Solicitar revisores {#request-reviewers}

Pasa un nombre de usuario simple de GitHub por cada opción repetida, sin abreviatura:

```bash
agent-sandbox run -a claude --pr --reviewers alice --reviewers bob -q "fix the login redirect"
```

En `run` y `run-tui`, los valores explícitos de la CLI reemplazan por completo
`run.reviewers`; las listas no se combinan. Omite la opción para usar la lista
JSON. Usa `--reviewers=""` por sí solo para vaciar los revisores configurados.
Las entradas vacías o con solo espacios en cualquier otra lista, incluido mezclar
un valor vacío con nombres, son errores de uso antes de ejecutar el sandbox.

Se quitan los espacios de los extremos. Los usuarios pueden contener letras,
dígitos y guiones, y deben empezar con una letra o un dígito. Se rechazan menciones
(`@alice`), correos, equipos, listas separadas por comas y cadenas con aspecto de
opción. Los duplicados se eliminan sin distinguir mayúsculas, conservando el
orden y la escritura de la primera aparición.

Para guardar valores predeterminados, agrega los usuarios a la sección `run`:

```json
{
  "agent": "codex",
  "run": {
    "pr": true,
    "reviewers": ["alice", "bob"]
  }
}
```

Luego indica el prompt con `agent-sandbox run -q "tu tarea"`.
Solo se solicitan revisores al crear un PR nuevo con `--pr` mediante `run` o
`run-tui`. Los PR existentes se conservan. Las ejecuciones solo con push, sin
`--pr`, sin cambios revisables, con fallos del agente o canceladas no solicitan
revisores. Si la creación informa un error pero la recuperación encuentra el PR,
el equipo comprueba las solicitudes pendientes sin repetirlas; si faltan, sigue
siendo un error.

`resume` y `resume-tui` aceptan y validan las mismas opciones y luego ignoran los
revisores válidos, incluido `--reviewers=""`. No agregan, eliminan ni vuelven a
solicitar revisores. Con `--pr` siguen requiriendo un PR abierto y conservan sus
revisores. El esquema JSON no cambia: solo se aceptan en `run`;
`resume.reviewers` y `reviewers` en la raíz son inválidos.

# Solución de problemas

## Falta una dependencia

Ejecuta `software-factory doctor`. Devuelve el estado 1 si falta `agent-sandbox`,
`git` o `docker` en `PATH`. Instala la dependencia o añade su directorio a `PATH`.
Doctor solo comprueba la disponibilidad de los ejecutables; usa `docker info`
para verificar el acceso al daemon de Docker.

## No se puede cargar la configuración

Ejecuta desde la raíz del proyecto que contiene `software-factory.json`. Usa
las [propiedades camelCase documentadas](configuration.md).

La propiedad `agentSandbox` es obligatoria; usa `{}` si no necesitas ajustes del
sandbox. Los ajustes se pasan como opciones de la CLI; no se genera un JSON de
configuración del sandbox. `apiKey` y `reviewers` se ignoran temporalmente en
ambas secciones. Autentica el agente en el host. Agent Sandbox informa los
fallos de validación del sandbox; consulta los [detalles del transporte](configuration.md#sandbox-settings-transport).

## No se encuentra la rama del sandbox

Para `continue`, establece `resume.branch` con una rama existente del sandbox.
Usa `agent-sandbox worktree-list` para listarlas. En el flujo predeterminado,
deshabilitar la implementación sigue requiriendo que exista el worktree de `run.branch`.

## No se encuentra una skill personalizada

Coloca la skill en `.agents/skills/<name>/SKILL.md` o
`.claude/skills/<name>/SKILL.md`, con una ruta relativa a `software-factory.json`.
Configura el nombre del directorio, no una ruta. La falta de skills de etapas
habilitadas detiene el flujo antes de la implementación.

## No se pueden cargar Google Docs

Usa una clave de cuenta de servicio llamada `service_account.json` en la
carpeta del proyecto. Comparte el documento o su carpeta con el `client_email`
de la cuenta y habilita la API de Drive. Los nombres deben identificar una
única carpeta accesible. Consulta la [configuración de Google Drive](google-drive.md).

## Fallan las credenciales de Jira o el acceso a la incidencia

Define `JIRA_EMAIL` y `JIRA_API_TOKEN` en el entorno o en el `.env` local.
Comprueba la [URL de la incidencia](jira.md) y que la cuenta pueda leerla.

## Una revisión informa BLOCKED o UNRESOLVED

Lee la salida de la etapa para conocer los subagentes de revisión o corrección
no disponibles, las verificaciones pendientes o los hallazgos restantes. Las
revisiones permiten hasta tres rondas. Un comando exitoso no garantiza que se
hayan resuelto todos los hallazgos.

## Un flujo falló o fue interrumpido

Inspecciona el registro de la etapa y `summary.log` en el directorio de registros
indicado. Establece `resume.branch` con la rama de esa ejecución y usa
`software-factory resume <path>` con el directorio de la invocación para
recuperar desde la última etapa guardada. Conserva la tarea en `run.filePrompt`
o vuelve a pasar `--jira` para una tarea de Jira. La recuperación necesita el
resumen original, el estado de las etapas y la evidencia previa de revisión
necesaria para clasificar el riesgo.

Si no dispones de esos archivos, usa `software-factory continue` para ejecutar
la implementación y las etapas habilitadas posteriores en una nueva invocación.
Consulta la [recuperación](workflow.md#resume-a-failed-or-interrupted-run) y los
[registros de ejecución](reports.md#execution-logs).

## Falta la explicación final

Los flujos predeterminado y `continue` advierten si falla la generación o el
guardado de la explicación y conservan el resultado de las etapas anteriores.
Durante `resume`, el fallo devuelve un error para poder reintentar la recuperación.
Comprueba si `reviewChanges` está deshabilitada y revisa las advertencias de la
terminal y los [informes](reports.md). Si los registros de ejecución no están
disponibles, el flujo puede continuar pero no puede guardar la explicación final.

## No se pudo añadir el resumen de ejecución al PR

`Warning: append PR execution summary` no cambia el resultado del flujo.
Para permitir la actualización, instala y autentica la CLI de GitHub (`gh`) en
el host, comprueba el remoto `origin` del proyecto y confirma que la rama del
sandbox tenga un pull request existente que puedas editar. Lee el `summary.log`
local si no hay un PR. Consulta los [resúmenes de PR](reports.md#github-pull-request-summaries).

# Flujo de trabajo

Ejecuta los comandos desde la carpeta del proyecto que contiene `software-factory.json`.

| Qué quieres hacer | Comando |
| --- | --- |
| Empezar una tarea nueva | `software-factory` |
| Seguir trabajando en una tarea existente | `software-factory continue` |
| Recuperar una ejecución fallida o interrumpida | `software-factory resume "/path/to/run-folder"` |

## Empieza una tarea {#start-a-task}

```bash
software-factory
```

Soft Factory carga la tarea y los documentos contexto y ejecuta cinco etapas:

1. Implementa la tarea en un worktree del sandbox.
2. Revisa el código y aplica correcciones.
3. Revisa la seguridad y aplica correcciones.
4. Clasifica el riesgo como **LOW**, **MEDIUM**, **HIGH** o **UNKNOWN**.
5. Escribe una explicación de los cambios sin modificarlos.

Las revisiones de código y seguridad permiten hasta tres rondas de revisión,
corrección y verificación. Con [saveFindings habilitado](configuration.md#save-security-findings-without-corrections),
la revisión de seguridad hace una sola pasada y sugiere correcciones sin aplicarlas.

Lee los [informes](reports.md) e inspecciona los cambios antes de aceptarlos.
Puedes [elegir agentes y modelos](configuration.md#stage-agents-and-models)
u [omitir etapas](configuration.md#disabling-stages) en la configuración.

## Continúa una tarea existente {#continue-an-existing-task}

```bash
software-factory continue
```

Vuelve a ejecutar la implementación y las etapas posteriores habilitadas en un
worktree existente del sandbox. Crea una carpeta nueva de registros y muestra
su ruta.

El comando predeterminado usa `run.branch`; `continue` usa `resume.branch`.
Ambos recurren a `agentSandbox.branch` si no hay un valor en la sección. Usa la
rama de tu tarea original. Para listar los worktrees disponibles:

```bash
agent-sandbox worktree-list
```

Conserva la tarea en `run.filePrompt`. Para una tarea de Jira, vuelve a pasar `--jira`:

```bash
software-factory continue --jira 'https://your-company.atlassian.net/browse/ENG-123'
```

## Recupera una ejecución fallida o interrumpida {#resume-a-failed-or-interrupted-run}

Copia la ruta de registros que mostró la ejecución fallida:

```bash
software-factory resume "/path/to/run-folder"
```

Ejecuta este comando desde la carpeta original del proyecto. Configura
`resume.branch` (o la `branch` compartida) con la rama del sandbox de esa
invocación. La recuperación lee la configuración y los documentos actuales y
la tarea de `run.filePrompt`. Para tareas de Jira, vuelve a pasar `--jira`.

La recuperación comienza desde la última etapa guardada:

- Una etapa exitosa avanza a la siguiente.
- Una etapa fallida o interrumpida vuelve a intentarse.
- Un archivo de estado vacío comienza en la implementación.
- Una explicación final exitosa indica que no queda trabajo pendiente.

La recuperación reutiliza los registros y el resumen. Conserva juntos
`summary.log`, `stage_status.txt` y los informes de revisión archivados. La
falta del estado o de la evidencia de revisión necesaria impide la recuperación.
Usa `continue` para iniciar una nueva ejecución en el worktree existente si
esos archivos no están disponibles.

Consulta los [registros de ejecución](reports.md#execution-logs) para conocer las rutas.

## Cuando falla una etapa {#when-a-stage-fails}

Un error antes de la explicación final detiene el flujo. Un error en la
explicación muestra una advertencia en las ejecuciones predeterminadas y con
`continue`; durante la recuperación devuelve un error para que puedas reintentar.

Veredictos como **UNRESOLVED** o **BLOCKED** pueden aparecer en una ejecución
exitosa. Revisa los informes para conocer los hallazgos y las verificaciones pendientes.

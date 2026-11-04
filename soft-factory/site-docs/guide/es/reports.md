# Informes

Al terminar una ejecución, abre `summary.log` para ver el resultado de cada
etapa. Después, revisa `05-reviewChanges.md` para entender los cambios y los
registros de las etapas si necesitas más detalle. La salida también aparece
en la terminal.

## Dónde se guardan {#execution-logs}

Soft Factory muestra la carpeta de registros al iniciar. Se guarda dentro de:

```text
~/.config/software-factory/logs/<proyecto>-<id>/<rama>/
```

Cada ejecución crea una subcarpeta con fecha y hora:

| Comando | Carpeta |
| --- | --- |
| `software-factory` | `run-<fecha-hora>` |
| `software-factory continue` | `continue-<fecha-hora>` |
| `software-factory resume <path>` | Reutiliza la carpeta indicada y añade los nuevos resultados. |

## Qué archivo abrir

| Archivo | Contenido |
| --- | --- |
| `summary.log` | Estado, duración, agente, modelo y resumen de cada etapa. |
| `01-implementation.log` | Detalles de la implementación. |
| `02-codeReview.log` | Revisión de código y correcciones. |
| `03-securityReview.log` | Revisión de seguridad y sus hallazgos. |
| `04-riskClassification.log` | Clasificación de riesgo. |
| `05-reviewChanges.md` | Explicación por archivo, líneas modificadas y diagramas. |
| `stage_status.txt` | Progreso guardado para recuperar la ejecución con `resume`. |

Las etapas omitidas no generan archivos. La explicación final se guarda cuando
se completa correctamente; su registro `.log` contiene los detalles de ejecución.

## Cómo leer el resultado

El estado de ejecución indica si una etapa terminó:

- `SUCCEEDED`: terminó correctamente.
- `FAILED`: falló.
- `INTERRUPTED`: fue interrumpida.

Las revisiones tienen además su propio veredicto:

| Veredicto | Significado |
| --- | --- |
| `PASS` | No quedan hallazgos que requieran acción. |
| `UNRESOLVED` | Quedan hallazgos sin resolver. |
| `BLOCKED` | Falta información o verificación para completar la revisión. |

Una etapa puede terminar correctamente y dejar hallazgos pendientes. Lee las
revisiones antes de aceptar los cambios.

Con [`saveFindings: true`](configuration.md#save-security-findings-without-corrections),
los hallazgos de seguridad y las correcciones sugeridas se incluyen en
`summary.log` sin aplicarse. Si aparece `Security findings unavailable.`,
consulta las advertencias y el registro de seguridad.

## Recuperar una ejecución

Para continuar desde la última etapa guardada:

```bash
software-factory resume "/ruta/a/la/carpeta-de-ejecucion"
```

Conserva la carpeta completa, incluidos `summary.log`, `stage_status.txt` y los
informes archivados `codeReview.txt` y `securityReview.txt` cuando existan.
Consulta la [guía de recuperación](workflow.md#resume-a-failed-or-interrupted-run).

Si falla la explicación final, los comandos predeterminado y `continue`
muestran una advertencia y conservan los resultados anteriores. En `resume`,
el fallo devuelve un error para que puedas reintentar.

## Resumen en GitHub {#github-pull-request-summaries}

Tras una ejecución exitosa con registros, Soft Factory intenta añadir el resumen
al pull request existente de la rama. Usa `gh`, la autenticación del host y el
remoto `origin`. No crea un PR y funciona independientemente de `push` y `pr`.

Si aparece `Warning: append PR execution summary`, el resultado de la ejecución
se conserva. Puedes leer el resumen local en `summary.log`.

Los registros incluyen la tarea y los documentos contexto. Mantén su contenido
privado y fuera de los commits.

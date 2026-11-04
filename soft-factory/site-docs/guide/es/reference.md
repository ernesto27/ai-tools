# Comandos y opciones

Ejecuta los comandos del flujo de trabajo desde la carpeta del proyecto.

| Comando | Comportamiento |
| --- | --- |
| `software-factory` | Implementa usando `run.branch`, revisa código y seguridad, clasifica el riesgo y explica los cambios. |
| `software-factory continue` | Continúa la implementación y todas las etapas habilitadas usando `resume.branch`. |
| `software-factory resume <path>` | Recupera una ejecución registrada desde su estado guardado y añade registros a la misma carpeta; usa `resume.branch`. |
| `software-factory doctor` | Busca `agent-sandbox`, `git` y `docker` en `PATH`; no requiere configuración. |
| `software-factory update` | Instala la última versión mediante el instalador incluido. |

| Opción | Propósito |
| --- | --- |
| `--jira URL` | Usa una incidencia de Jira Cloud en lugar de la tarea local; disponible para los comandos del flujo de trabajo. |
| `-v`, `--version` | Muestra la etiqueta de la versión o `dev` para una compilación local. |
| `-h`, `--help` | Muestra la ayuda del comando. |

`--config` y `-c` no están disponibles. Soft Factory siempre lee
`software-factory.json` desde la raíz del proyecto, incluidos los ajustes
`agentSandbox`. No genera un archivo de configuración del sandbox; los ajustes
admitidos se pasan mediante opciones. `apiKey` y `reviewers` de las secciones
se ignoran temporalmente; usa la autenticación existente del host.

Consulta la [configuración](configuration.md) para las propiedades JSON y los
[informes](reports.md) para las ubicaciones de salida y los estados de revisión.

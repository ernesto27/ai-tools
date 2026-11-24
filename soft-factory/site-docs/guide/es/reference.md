# Comandos y opciones

Ejecuta los comandos del flujo de trabajo desde la carpeta del proyecto.

| Comando | Comportamiento |
| --- | --- |
| `software-factory` | Implementa usando `run.branch`, revisa código y seguridad, clasifica el riesgo y explica los cambios. |
| `software-factory continue` | Continúa la implementación y todas las etapas habilitadas usando `resume.branch`. |
| `software-factory resume <path>` | Recupera una ejecución registrada desde su estado guardado y añade registros a la misma carpeta; usa `resume.branch`. |
| `software-factory doctor` | Busca `agent-sandbox`, `git` y `docker` en `PATH`; no requiere configuración. |
| `software-factory update` | Instala la última versión mediante el instalador incluido. |
| `software-factory scheduled-task` | Crea, edita, elimina o lista [flujos programados](#scheduled-tasks). |

| Opción | Propósito |
| --- | --- |
| `--jira URL` | Usa una incidencia de Jira Cloud en lugar de la tarea local; disponible para los comandos del flujo de trabajo. |
| `--file PATH` | Ejecuta el flujo predeterminado con un archivo de tarea local y una rama nueva. |
| `-c`, `--config PATH` | Lee este archivo de configuración en lugar de `software-factory.json`; disponible para el flujo predeterminado, `continue`, `resume` y `audit`. |
| `-v`, `--version` | Muestra la etiqueta de la versión o `dev` para una compilación local. |
| `-h`, `--help` | Muestra la ayuda del comando. |

De forma predeterminada, Soft Factory lee `software-factory.json` desde la raíz
del proyecto, incluidos los ajustes `agentSandbox`. Pasa `--config` para usar
otro archivo:

```bash
software-factory --config configs/staging.json
```

Las rutas relativas de ese archivo siguen siendo relativas a la raíz del
proyecto, no a la carpeta del archivo. Las tareas programadas siempre usan
`software-factory.json`. No genera un archivo de configuración del sandbox; los ajustes
admitidos se pasan mediante opciones. Usa la autenticación del host.

Consulta la [configuración](configuration.md) para las propiedades JSON y los
[informes](reports.md) para las ubicaciones de salida y los estados de revisión.

## Programar un flujo de trabajo {#scheduled-tasks}

En Linux o macOS, ejecuta estos comandos desde la carpeta del proyecto que
contiene `software-factory.json`. El ejecutable instalado `software-factory`
debe estar en tu `PATH` al crear o editar una tarea, y `agent-sandbox` debe
estar en el `PATH` de cron cuando se ejecute.

```sh
software-factory scheduled-task create --name weekday-morning --file task.txt --cron '30 8 * * 1-5'
software-factory scheduled-task edit --name weekday-morning --file task.txt --cron '0 9 * * 1-5'
software-factory scheduled-task delete --name weekday-morning
```

El primer comando programa la tarea para las 8:30 de lunes a viernes. El
segundo cambia la hora a las 9:00. Los nombres son únicos dentro del proyecto.
Usa una ruta relativa o absoluta para `--file` y pon entre comillas la
expresión de cinco campos de `--cron`. Para cambiar solo el archivo de tarea,
omite `--cron` para conservar el horario.

Desde cualquier carpeta, usa `list` con la ruta del repositorio. Las rutas
relativas se resuelven desde la carpeta actual.

```sh
software-factory scheduled-task list --path /ruta/al/repositorio
```

El comando imprime un arreglo JSON con las tareas guardadas, incluidas las
pendientes de instalación en crontab. Si el repositorio no tiene tareas
guardadas, devuelve `[]`.

Cada ejecución comienza en la carpeta del proyecto, lee la configuración del
flujo de `software-factory.json` y usa el archivo programado como tarea. Cada
ejecución tiene una rama nueva que se usa en todas las etapas habilitadas.

Usa `crontab -l` para ver las tareas instaladas. La salida de cron, incluidos
los errores de inicio, se añade a
`~/.config/software-factory/scheduled-tasks/cron-output.log`. Consulta los
[informes](reports.md) para ver los registros y resúmenes de las etapas.

# Tu primera ejecución

Ejecuta Soft Factory desde el proyecto Git que quieres que modifique el agente.
Primero instala [Soft Factory y Agent Sandbox](installation.md) y autentica el agente elegido.

## Escribe una tarea

Crea `task.txt` con una petición concreta, por ejemplo:

```text
Corrige el bucle de redirección del inicio de sesión y verifica el cambio con las pruebas existentes.
```

## Configura el flujo de trabajo

Crea `software-factory.json`:

```json
{
  "documents": [],
  "agentSandbox": {
    "agent": "codex",
    "branch": "fix-login",
    "push": false,
    "pr": false,
    "run": {
      "filePrompt": "task.txt"
    }
  }
}
```

Los ajustes compartidos del agente, la rama y la publicación se aplican tanto
a `run` como a `resume`. Conserva la ruta de la tarea local en `run.filePrompt`.
Elige una rama nueva para cada tarea nueva. Consulta la
[configuración](configuration.md) para elegir agentes y documentos contexto.

## Ejecuta la tarea

```bash
software-factory doctor
software-factory
```

Soft Factory implementa la tarea, revisa el código y la seguridad, clasifica el
riesgo y escribe una explicación de los cambios. Las revisiones pueden corregir
archivos en el worktree del sandbox. Lee los [informes](reports.md) e inspecciona
los cambios antes de aceptarlos.

Para seguir trabajando en la misma rama, usa [continue](workflow.md#continue-an-existing-task).

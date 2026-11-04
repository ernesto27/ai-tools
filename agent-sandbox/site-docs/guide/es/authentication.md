# Autenticación {#authentication}

Elige la sesión del agente que ya tienes en tu entorno local o, para Codex y Claude Code,
una clave de API. `agent-sandbox` no proporciona acceso a modelos: usas tu propia
cuenta del proveedor.

## Usar tu sesión existente {#use-your-existing-host-session}

Instala el agente elegido e inicia sesión en tu entorno local siguiendo sus instrucciones.
Luego selecciónalo con `-a`:

```bash
agent-sandbox run -a claude -q "inspect this repository and summarize its structure"
```

La herramienta valida y monta estas rutas existentes:

| Agente | Rutas requeridas en el equipo |
| --- | --- |
| `codex` | `~/.codex/` |
| `claude` | `~/.claude/` y `~/.claude.json` |
| `opencode` | `~/.config/opencode/`, `~/.local/share/opencode/`, `~/.local/state/opencode/` |
| `pi` | `~/.pi/agent/` |

Si falta alguna ruta, se produce un error; la herramienta no la crea.
opencode y pi solo admiten este modo.

## Usar una clave de API {#use-an-api-key}

Pasa `--apiKey <key>` a `run`, `resume`, `run-tui` o `resume-tui`:

```bash
agent-sandbox run -a codex --apiKey "$OPENAI_API_KEY" -q "inspect this repository"
agent-sandbox resume -b fix-login -a codex --apiKey "$OPENAI_API_KEY" --pr -q "add a regression test"
```

También puedes crear `agent-sandbox.json` en el directorio de ejecución.
Para Codex:

```json
{
  "run": {
    "agent": "codex",
    "apiKey": "<YOUR_OPENAI_API_KEY>",
    "query": "inspect this repository and summarize its structure"
  }
}
```

Luego ejecuta:

```bash
agent-sandbox run
```

Para Claude Code, usa `"agent": "claude"` y tu clave de API de Anthropic.
En JSON, `apiKey` solo se admite en las secciones `run` y `resume`.

En este modo, la clave se entrega al contenedor mediante la variable de entorno
del proveedor. El contenedor usa un directorio personal temporal y no monta las
credenciales del agente de tu entorno local. Las claves no se agregan a los prompts,
registros de worktrees, ayuda ni transcripciones.

::: warning Mantén tu clave fuera de Git
Reemplaza el marcador localmente y agrega `agent-sandbox.json` al `.gitignore`
de tu proyecto antes de guardar una clave real. No incluyas claves en prompts,
capturas de pantalla ni registros compartidos.
:::

Continúa con [tu primera ejecución](first-run.md).

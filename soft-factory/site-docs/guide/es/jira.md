# Tareas de Jira

Define `JIRA_EMAIL` y `JIRA_API_TOKEN` en el entorno o en un archivo `.env`
local. Puedes usar `.env.example` como punto de partida.

```bash
software-factory --jira 'https://your-company.atlassian.net/browse/ENG-123'
```

El resumen y la descripción de la incidencia sustituyen a la tarea local en
todas las etapas. No se incluyen comentarios ni adjuntos. Los documentos de
apoyo siguen aplicándose y `software-factory.json` sigue siendo obligatorio.

Consulta la [configuración de Jira](#jira-setup) para preparar las credenciales.

## Configuración de Jira {#jira-setup}

Usa el correo de tu cuenta de Jira Cloud y un token de API de una cuenta con
acceso de lectura a la incidencia. Crea un archivo `.env` local en la carpeta
del proyecto:

```dotenv
JIRA_EMAIL=you@example.com
JIRA_API_TOKEN=your-api-token
```

Sustituye estos valores por tus credenciales. Mantén el archivo privado y fuera
de Git. También puedes definir las variables en el entorno.

Usa la URL completa de la incidencia, por ejemplo
`https://your-company.atlassian.net/browse/ENG-123`.
Debe usar HTTPS y la dirección `.atlassian.net` de tu sitio Jira Cloud.

Vuelve a pasar `--jira` al continuar la tarea:

```bash
software-factory --jira 'https://your-company.atlassian.net/browse/ENG-123' continue
```

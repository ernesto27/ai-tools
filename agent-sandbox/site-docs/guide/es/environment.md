# Imágenes y redes {#images-and-networking}

## Proporcionar las herramientas de tu proyecto {#provide-your-project-s-toolchain}

La imagen predeterminada contiene los agentes y sus herramientas auxiliares.
Si la tarea necesita un entorno de ejecución de un lenguaje, elige una imagen
base compatible que lo incluya:

```bash
agent-sandbox run -b fix-go-tests -a codex -i golang:1.26-alpine \
  -q "run gofmt and go test ./..., then fix failures"
```

`-i` abrevia `--baseImage`. La herramienta construye una imagen local derivada
de tu base y agrega Node.js, npm, Bash, los agentes, ripgrep, certificados CA,
curl, Git, Python 3 (también como `python`) y herramientas PDF de Poppler
(`pdftotext` y `pdfinfo`). También se incluyen en las imágenes predeterminadas.
Las ejecuciones siguientes reutilizan la imagen y comprueban la versión del
agente en npm, reconstruyéndola si difiere.

Las bases compatibles son Alpine, Debian, Ubuntu, Fedora, RHEL 8/9, UBI 8/9 y
Amazon Linux 2023. Deben proporcionar `apk`, `apt-get`, `dnf` o `microdnf`;
no se admiten `yum` ni otros gestores de paquetes. Se reutilizan Node.js y npm
si ambos están disponibles. En caso contrario, Alpine instala sus paquetes de
Node.js; las otras bases compatibles usan Node.js 22 de NodeSource.

Las bases RHEL necesitan repositorios funcionales y las suscripciones requeridas
dentro de la imagen. No se montan las credenciales de suscripción del equipo ni
se habilitan EPEL o CRB/CodeReady Builder. Si falta un paquete, la construcción falla.

Para inspeccionar las imágenes derivadas:

```bash
docker image ls 'agent-sandbox-base-*'
```

Elimina una imagen derivada sin uso con `docker image rm <IMAGE_ID>` para
recuperar espacio. Se reconstruirá si vuelve a ser necesaria.

## Adjuntar capturas o imágenes de referencia {#attach-screenshots-or-reference-images}

Codex y Claude Code aceptan imágenes adjuntas. Pasa un archivo regular existente
en tu entorno local por cada `--image`:

```bash
agent-sandbox run -a claude \
  --image /path/to/mockup.png \
  --image /path/to/reference.png \
  -q "compare these screenshots and implement the resulting UI"
```

Los adjuntos se montan en modo de solo lectura. opencode y pi los ignoran.
`--image` selecciona un adjunto; `-i` selecciona una imagen base de Docker.

## Montar archivos de referencia {#mount-reference-files}

Repite `--file` para proporcionar PDF, CSV u otros archivos regulares a cualquiera
de los cuatro agentes:

```bash
agent-sandbox run -a claude \
  --file "/path/to/spec.pdf" \
  --file "/path/to/sample.csv" \
  -q 'read "spec.pdf" and "sample.csv", then implement the requested feature'
```

Cada ruta debe ser un archivo regular existente. Las rutas relativas se resuelven
desde el directorio de ejecución, incluidas las de la configuración JSON.
Los archivos se montan en modo de solo lectura bajo
`/agent-sandbox-files/<number>/<filename>`. El prompt enumera los nombres y rutas
del contenedor; los nombres duplicados usan directorios numerados separados,
por lo que debes referirte a sus rutas del contenedor cuando sea necesario.

`--file` funciona con `run`, `resume`, `run-tui` y `resume-tui`. También puedes
configurar un [arreglo JSON `file`](configuration.md#share-reference-files).
Proporciona material de referencia; `-f` / `--filePrompt` lee las propias
instrucciones de la tarea.

Pide al agente que lea o extraiga los archivos como parte de la tarea. No se
convierten automáticamente. Poppler se instala al construir las imágenes;
no hay comprobación durante la ejecución ni reconstrucción automática si falta.

## Montar carpetas de referencia {#mount-reference-folders}

Repite `--folder` para compartir directorios existentes con cualquier agente:

```bash
agent-sandbox run -a codex \
  --folder "/path/to/reference-project" \
  --folder "/path/to/design-notes" \
  -q "Read the reference folders and use their conventions for this task."
```

Las carpetas se montan en modo de solo lectura bajo
`/agent-sandbox-folders/<number>/<folder-name>`, fuera del worktree de la tarea.
Las rutas relativas se resuelven desde el directorio de ejecución. Los directorios
numerados separan carpetas con el mismo nombre. El prompt enumera sus rutas en
el contenedor y pide al agente que lea sus archivos.

`--folder` funciona en los cuatro comandos de ejecución. Es exclusivo de la CLI;
`folder` no es un campo admitido en JSON.

## Comprobar el navegador dentro del contenedor {#browser-checks-inside-the-container}

Las nuevas construcciones de imágenes predeterminadas y derivadas incluyen
Chromium, fuentes, Puppeteer Core y FFmpeg. Los agentes pueden comprobar una
interfaz web y capturar pantallas o video. Para una captura dentro del contenedor:

```bash
agent-sandbox-browser http://localhost:3000 /workspace/page.png
```

Inicia primero el servidor web del proyecto. Un tercer argumento opcional indica
un selector CSS requerido. Los scripts personalizados pueden usar
`require("/opt/agent-sandbox-browser/browser.cjs").launch()`.

Estas herramientas se instalan al construir la imagen. No se comprueba si faltan
en imágenes existentes; reconstruye las antiguas para incorporarlas. Las bases
externas deben proporcionar los paquetes de navegador, fuentes y FFmpeg requeridos.
Consulta [evidencias de verificación](running.md#review-verification-evidence)
para conocer los archivos que se pide conservar a los agentes.

## Acceder a servicios de tu entorno local {#reach-services-on-your-host}

Cuando una tarea necesita servicios locales, habilita explícitamente la red del equipo:

```bash
agent-sandbox run -a codex --hn \
  -q "inspect the API running on localhost:8080 and update the client"
```

`--hn` comparte la red del equipo con el contenedor y da acceso a servicios locales
sin una lista de puertos permitidos. Reduce el aislamiento de red y está desactivado
por defecto. Usa `--hn=false` para reemplazar un valor activado en JSON.

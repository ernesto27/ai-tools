# Instalación {#installation}

## Requisitos {#what-you-need}

El binario publicado y el instalador admiten:

- **Linux x86_64**.
- **macOS Intel (x86_64)**.
- **macOS Apple silicon (arm64)**.

El instalador detecta el sistema operativo y la arquitectura, descarga el archivo
correspondiente, verifica su suma SHA-256 e instala el binario en `~/.local/bin`.
En macOS puede usar el comando integrado `shasum` para verificarlo.

También necesitas:

- Git y un repositorio Git en el que trabajar.
- Docker instalado, en ejecución y accesible para tu usuario.
- Un agente autenticado en tu entorno local o una clave de API para Codex o Claude Code.

## Instalar la última versión {#install-the-latest-release}

```bash
curl -fsSL https://raw.githubusercontent.com/ernesto27/ai-tools/master/agent-sandbox/install.sh | bash
```

## Comprobar la instalación {#check-your-setup}

```bash
agent-sandbox --version
agent-sandbox doctor
```

`doctor` comprueba si `git`, `docker`, `gh` y `code` están en `PATH`.
Finaliza correctamente aunque falten dependencias y **no** comprueba el estado
del servicio Docker, la autenticación ni los permisos. `docker info` comprueba
si tu usuario puede acceder a Docker.

Para una interfaz gráfica o un script, solicita la salida JSON:

```bash
agent-sandbox doctor --json
```

La respuesta es un arreglo con solo `name` (cadena) e `installed` (booleano),
en el orden `git`, `docker`, `gh`, `code`. Por ejemplo, si faltan las cuatro herramientas:

```json
[
  {"name": "git", "installed": false},
  {"name": "docker", "installed": false},
  {"name": "gh", "installed": false},
  {"name": "code", "installed": false}
]
```

## Actualizar {#update}

```bash
agent-sandbox update
```

Esto vuelve a ejecutar el instalador. Actualizar la CLI y actualizar el agente
del contenedor son operaciones independientes: cada ejecución o continuación
comprueba la versión del agente en npm y reconstruye su imagen si hace falta.

## Compilar desde el código fuente {#build-from-source}

Con la versión de Go indicada en `go.mod`, ejecuta lo siguiente desde el
directorio `agent-sandbox` del repositorio:

```bash
go build -o agent-sandbox ./cmd/agent-sandbox
./agent-sandbox --help
```

Luego, [elige un modo de autenticación](authentication.md).

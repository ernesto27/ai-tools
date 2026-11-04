# Instalación

## Requisitos

Soft Factory admite **Linux x86_64** y **macOS con Intel y Apple Silicon**.

- Git instalado y un proyecto en el que trabajar.
- [Agent Sandbox instalado y configurado](https://ernesto27.github.io/ai-tools/es/installation.html).
- Docker en ejecución y una sesión iniciada en el agente de programación elegido.

## Instala Soft Factory

Abre una terminal y ejecuta:

```bash
curl -fsSL https://raw.githubusercontent.com/ernesto27/ai-tools/master/soft-factory/install.sh | bash
```

El instalador instala la versión más reciente. Sigue las instrucciones que
muestre para poder ejecutar `software-factory` desde tu terminal.

## Comprueba la instalación

```bash
software-factory --version
software-factory doctor
```

El primer comando muestra la versión instalada. El segundo comprueba la
presencia de Agent Sandbox, Git y Docker. Si falta alguna herramienta,
instálala antes de empezar tu primera tarea.

## Actualiza {#update}

Para obtener la versión más reciente:

```bash
software-factory update
```

A continuación, [configura tu primera tarea](first-run.md).

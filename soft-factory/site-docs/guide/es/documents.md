# Documentos contexto

Enumera los documentos locales en `software-factory.json`:

```json
{
  "documents": ["requirements.md", "guidelines.md"]
}
```

Las rutas son relativas al archivo de configuración. Su contenido acompaña a
la tarea durante todo el flujo de trabajo.

Para incluir también Google Docs de carpetas compartidas de Drive:

```json
{
  "documents": ["requirements.md"],
  "googleDrive": {
    "folders": ["Project Documentation"]
  }
}
```

Guarda la clave de la cuenta de servicio como `service_account.json` en la
carpeta del proyecto y comparte la carpeta de Drive con esa cuenta. Cada nombre
debe identificar una única carpeta accesible. Solo se incluyen los Google Docs
directamente dentro de las carpetas seleccionadas; se excluyen las subcarpetas
y otros tipos de archivo.

Para cargar Google Docs individuales sin cargar sus carpetas, usa `files`:

```json
{
  "googleDrive": {
    "files": [
      "https://docs.google.com/document/d/DOCUMENT_ID/edit?tab=t.0"
    ]
  }
}
```

Sustituye la URL del ejemplo por la URL completa de tu documento.
Puedes configurar solo `folders`, solo `files` o ambos en el mismo bloque
`googleDrive`. Al menos una lista debe contener una entrada. Los documentos
individuales pueden estar en distintas carpetas si la cuenta de servicio tiene
acceso a ellos. Las URL no válidas fallan al cargar la configuración; los
documentos inaccesibles o los errores de exportación detienen el flujo al cargar
el contexto.

Consulta la [configuración de Google Drive](google-drive.md).

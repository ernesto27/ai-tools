# Supporting documents

List local documents in `software-factory.json`:

```json
{
  "documents": ["requirements.md", "guidelines.md"]
}
```

Document paths are relative to the configuration file. Their contents accompany the task throughout the workflow.

To also include Google Docs from shared Drive folders:

```json
{
  "documents": ["requirements.md"],
  "googleDrive": {
    "folders": ["Project Documentation"]
  }
}
```

Save your service-account key as `service_account.json` in the project folder and share the Drive folder with that account. Folder names must identify a single accessible folder. Only Google Docs directly inside the selected folders are included; subfolders and other file types are excluded.

To load individual Google Docs without loading their folders, use `files`:

```json
{
  "googleDrive": {
    "files": [
      "https://docs.google.com/document/d/DOCUMENT_ID/edit?tab=t.0"
    ]
  }
}
```

Replace the example URL with your document's full Google Docs URL.
You can configure `folders` only, `files` only, or both in the same `googleDrive` block. At least one array must contain an entry. Individual documents can be in different folders, provided the service account has access to them. Invalid URLs fail when configuration loads; inaccessible documents or export errors stop the workflow during context loading.

See [Google Drive setup](google-drive.md) for instructions.

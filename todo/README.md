# Daylist

A responsive todo app built with plain HTML, CSS, and JavaScript. Add tasks,
check them off, delete them, or filter by status. Tasks are saved in your
browser's localStorage; no account, backend, dependencies, or build is needed.

From the repository root, start a local web server:

```sh
python3 -m http.server 8000 --directory todo
```

Open <http://localhost:8000>. You can also open `todo/index.html` directly,
although localStorage behavior for local files varies between browsers.
Use the same browser and address to return to your saved tasks. Clearing
browser site data removes saved tasks. If storage is blocked or full, the
app displays a warning and keeps working for the current page session.

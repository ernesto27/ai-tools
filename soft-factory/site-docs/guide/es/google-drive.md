# Configuración de una cuenta de servicio de Google Drive

Esta guía configura una cuenta de servicio con acceso a una carpeta del Drive
de una cuenta personal de Gmail. Usa el navegador para configurar Google Cloud
y compartir la carpeta.

## 1. Selecciona o crea un proyecto de Google Cloud

1. Abre [Google Cloud Console](https://console.cloud.google.com/).
2. Inicia sesión con tu cuenta de Google.
3. Abre el selector de proyectos de la parte superior.
4. Selecciona un proyecto existente o haz clic en **New Project** (Nuevo proyecto).
5. Si creas uno, escribe un nombre, haz clic en **Create** (Crear) y selecciónalo.

## 2. Habilita Google Drive API

1. Con el proyecto seleccionado, abre **APIs & Services → Library** (APIs y servicios → Biblioteca).
2. Busca **Google Drive API**.
3. Ábrela y haz clic en **Enable** (Habilitar). Si ya está habilitada, continúa.

## 3. Crea la cuenta de servicio

1. Abre **IAM & Admin → Service Accounts** (IAM y administración → Cuentas de servicio).
2. Haz clic en **Create service account** (Crear cuenta de servicio).
3. Escribe un nombre, por ejemplo `drive-reader`.
4. Haz clic en **Create and continue** (Crear y continuar).
5. Deja vacíos los campos opcionales de rol del proyecto y acceso de usuarios y
   termina con **Done** (Listo). El acceso a la carpeta se concede aparte en Drive.
6. Copia el correo de la cuenta desde su página de detalles. Tiene esta forma:
   `drive-reader@YOUR_PROJECT_ID.iam.gserviceaccount.com`.

Si ya creaste la cuenta de servicio, reutilízala.

## 4. Descarga la clave JSON de la cuenta de servicio

1. Selecciona esa cuenta de servicio.
2. Abre la pestaña **Keys** (Claves).
3. Haz clic en **Add key → Create new key** (Agregar clave → Crear clave nueva).
4. Selecciona **JSON** y haz clic en **Create** (Crear).
5. Guarda el archivo descargado en una ubicación privada. Puedes llamarlo
   `service_account.json`.

Si ya tienes su clave JSON, reutilízala en lugar de crear otra.

## 5. Confirma el tipo de credencial

Abre el JSON descargado localmente. Debe contener estos campos:

```json
{
  "type": "service_account",
  "client_email": "drive-reader@YOUR_PROJECT_ID.iam.gserviceaccount.com",
  "private_key": "..."
}
```

Es un ejemplo abreviado, no una credencial utilizable. Conserva el archivo
completo descargado. Trata la clave privada como un secreto y no la compartas.

Una credencial de cliente OAuth llamada
`client_secret_...apps.googleusercontent.com.json` es de otro tipo. Descarga la
clave desde la pestaña **Keys** (Claves) de tu cuenta de servicio.

## 6. Comparte la carpeta de Drive

1. Abre [Google Drive](https://drive.google.com/) con tu cuenta personal de Gmail.
2. Busca la carpeta a la que quieres conceder acceso.
3. Haz clic derecho y selecciona **Share / Compartir**.
4. Pega el correo exacto de la cuenta de servicio, que debe coincidir con `client_email` en su clave.
5. Selecciona **Viewer / Lector**.
6. Desmarca **Notify people / Notificar a las personas**.
7. Haz clic en **Share / Compartir** para guardar el permiso.

La cuenta de servicio puede acceder a los archivos compartidos con ella;
compartir una carpeta no le da acceso a todo tu Drive.

Referencia: [instrucciones de Google sobre cuentas de servicio y acceso compartido](https://developers.google.com/workspace/guides/create-credentials#access_google_workspace_files_directly_with_a_service_account).

## 7. Verifica el permiso de la carpeta

1. Vuelve a abrir el diálogo **Share / Compartir** de la carpeta.
2. En **People with access / Personas con acceso**, busca el correo de la cuenta de servicio.
3. Confirma que su rol sea **Viewer / Lector**.
4. Compara el correo con `client_email` en la clave descargada.

La configuración está completa cuando la API está habilitada, la clave está
guardada y la carpeta está compartida con el correo de la cuenta de servicio correspondiente.

## 8. Comprueba problemas habituales

| Síntoma | Qué comprobar |
| --- | --- |
| Drive indica que el correo no tiene cuenta de Google | Copia el correo de una cuenta de servicio existente y habilitada e intenta compartir de nuevo sin notificaciones. |
| La cuenta de servicio no figura entre los accesos de la carpeta | Repite el paso de compartir y guarda el permiso. |
| El JSON descargado no tiene `"type": "service_account"` | Descarga una clave JSON desde **IAM & Admin → Service Accounts → tu cuenta → Keys**. |
| Google Drive API está deshabilitada | Selecciona el proyecto de la cuenta de servicio y habilita la API en **APIs & Services → Library**. |

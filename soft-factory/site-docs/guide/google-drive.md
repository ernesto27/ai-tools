# Google Drive service-account setup

This guide sets up a service account with access to a folder in a personal
Gmail Drive. Use the browser to configure Google Cloud and share the folder.

## 1. Select or create a Google Cloud project

1. Open [Google Cloud Console](https://console.cloud.google.com/).
2. Sign in with your Google account.
3. Open the project selector at the top.
4. Select your existing project, or click **New Project**.
5. If creating a project, enter a name and click **Create**, then select it.

## 2. Enable Google Drive API

1. With the project selected, open **APIs & Services → Library**.
2. Search for **Google Drive API**.
3. Open it and click **Enable**. If already enabled, continue.

## 3. Create the service account

1. Open **IAM & Admin → Service Accounts**.
2. Click **Create service account**.
3. Enter a name, for example `drive-reader`.
4. Click **Create and continue**.
5. Leave the optional project-role and user-access fields empty, then finish
   with **Done**. Folder access is granted separately in Google Drive.
6. Copy the service account's email directly from its details page. It has this
   form: `drive-reader@YOUR_PROJECT_ID.iam.gserviceaccount.com`.

If you already created the service account, reuse that account.

## 4. Download the service-account JSON key

1. Select that service account.
2. Open its **Keys** tab.
3. Click **Add key → Create new key**.
4. Select **JSON** and click **Create**.
5. Save the downloaded file in a private location. You may name it
   `service_account.json`.

If you already have its JSON key, reuse it instead of creating another.

## 5. Confirm the credential type

Open the downloaded JSON locally. It should contain these fields:

```json
{
  "type": "service_account",
  "client_email": "drive-reader@YOUR_PROJECT_ID.iam.gserviceaccount.com",
  "private_key": "..."
}
```

This is a shortened illustration, not a usable credential. Keep the complete
downloaded file. Treat its private key as a secret and do not share it.

An OAuth client credential named `client_secret_...apps.googleusercontent.com.json`
is a different credential type. Download the key from your service account's
**Keys** tab.

## 6. Share your Drive folder

1. Open [Google Drive](https://drive.google.com/) with your personal Gmail account.
2. Find the folder you want to grant access to.
3. Right-click it and select **Share** / **Compartir**.
4. Paste the service account's exact email, matching `client_email` in its key.
5. Select **Viewer** / **Lector**.
6. Uncheck **Notify people** / **Notificar a las personas**.
7. Click **Share** / **Compartir** to save the permission.

The service account can access the files shared with it; sharing one folder
does not grant access to your entire Drive.

Reference: [Google's service-account and sharing instructions](https://developers.google.com/workspace/guides/create-credentials#access_google_workspace_files_directly_with_a_service_account).

## 7. Verify the folder permission

1. Open the folder's **Share** / **Compartir** dialog again.
2. Under **People with access** / **Personas con acceso**, find the service
   account's email.
3. Confirm its role is **Viewer** / **Lector**.
4. Compare the email with the `client_email` in the downloaded key.

Setup is complete when the API is enabled, the service-account key is saved,
and the folder is shared with the matching service-account email.

## 8. Check common setup problems

| Symptom | What to check |
| --- | --- |
| Drive says the email has no Google account | Copy the email directly from an existing, enabled service account and retry sharing with notifications unchecked. |
| The service account is missing from the folder's access list | Repeat the sharing step and save the permission. |
| The downloaded JSON has no `"type": "service_account"` | Download a JSON key from **IAM & Admin → Service Accounts → your account → Keys**. |
| Google Drive API is disabled | Select the service account's project and enable the API under **APIs & Services → Library**. |


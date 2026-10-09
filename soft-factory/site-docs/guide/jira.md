# Jira tasks

Set `JIRA_EMAIL` and `JIRA_API_TOKEN` in your environment or in a local `.env` file. You can use `.env.example` as a starting point.

```bash
software-factory --jira 'https://your-company.atlassian.net/browse/ENG-123'
```

To review existing changes against a Jira task:

```bash
software-factory --jira 'https://your-company.atlassian.net/browse/ENG-123' review
```

The issue summary and description replace the local task for every stage. Comments and attachments are not included. Supporting documents still apply, and `software-factory.json` is still required.

See [Jira setup](#jira-setup) for credentials and configuration.

## Jira setup

Use your Jira Cloud account email and an API token for an account that can
read the issue. Create a local `.env` file in the project folder:

```dotenv
JIRA_EMAIL=you@example.com
JIRA_API_TOKEN=your-api-token
```

Replace these values with your credentials. Keep this file private and out
of Git. You can also set these variables in your environment.

Use the full issue URL, such as
`https://your-company.atlassian.net/browse/ENG-123`.
The URL must use HTTPS and your Jira Cloud site's `.atlassian.net` address.

Pass `--jira` again when continuing the task:

```bash
software-factory --jira 'https://your-company.atlassian.net/browse/ENG-123' continue
```

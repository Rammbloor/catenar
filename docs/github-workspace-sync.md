# GitHub workspace sync

## Status

Existing-repository pull and push are implemented. Local YAML workspaces remain the canonical and fully supported storage mode. GitHub sync is opt-in and never blocks local use.

## Repository layout

One private repository can contain multiple independent workspaces:

```text
catenar-workspaces/
  workspaces/
    payments-api/
      workspace.yaml
      requests/
        create-payment.yaml
    users-api/
      workspace.yaml
      requests/
        get-user.yaml
```

The configured workspace path is always relative to the repository root. Existing Catenar YAML contracts are reused without a cloud-only format.

## Authentication and permissions

- For HTTPS, Catenar accepts a GitHub fine-grained Personal Access Token directly in **Settings → Workspace → GitHub sync**. The token is stored only in the operating system credential manager: Keychain on macOS and Credential Manager on Windows. It is never placed in `workspace.yaml`, the repository URL, Git configuration, diagnostics, or Catenar's JSON settings.
- Create the token with **Only select repositories**, choose the target repository, and grant **Contents: Read and write**. This limits the token to the one repository used by the workspace.
- SSH URLs remain supported and use the user's existing SSH key. Catenar does not collect or alter that key.
- Persist only non-secret link data in private application settings: repository URL, branch, workspace path and last synchronized commit.
- Creating a repository requires explicit confirmation and always creates a private repository by default.
- Existing repositories must be selected explicitly. Catenar must not enumerate or modify unrelated repositories beyond the granted installation scope.

## Sync flow

1. Save the active workspace locally and validate it.
2. Fetch the configured branch.
3. Compare the remote workspace revision with the last synchronized revision.
4. Pull clean remote changes before writing local changes.
5. If both sides changed, stop and require the user to choose pull or push. The selected direction is explicit and Git history still has to advance with a fast-forward; Catenar never force-pushes.
6. Commit only the selected workspace directory and push it to the configured branch.
7. Record the synchronized commit SHA outside the workspace YAML.

## Failure behavior

- Offline, expired credentials and GitHub outages never prevent local editing or saving.
- Failed pushes keep local changes intact and expose a retry action.
- Secret metadata is redacted before diagnostics export and is never added to Git history by the sync layer.
- Repository deletion, unlinking and credential revocation are separate explicit actions.

## Delivery stages

1. Done: repository link model and connection status through SSH, system Git credentials, or an app-managed HTTPS token stored in the operating system credential manager.
2. Done: pull and push for an existing private repository.
3. Done: explicit conflict direction without silent overwrite or force-push.
4. Future: optional private repository creation through a GitHub App installation. This requires a registered GitHub App or OAuth client and is intentionally not emulated by collecting a personal token.

## Using the feature

1. Open or create a local Catenar workspace.
2. Open **Settings → Workspace → GitHub sync**.
3. For HTTPS, use **Create a fine-grained token on GitHub**, limit it to the selected repository and grant **Contents: Read and write**. Paste it into Catenar. For SSH, use a `git@github.com:…` address instead.
4. Enter an existing repository URL, branch and a relative folder such as `catenar/payments-api`.
5. Link the repository, then choose **Pull from GitHub** or **Push to GitHub** when changes are available.

Catenar clones a private working copy under its application config directory, commits only the selected workspace folder and stores no credentials in the workspace repository.

Notion and Obsidian are not first-class providers for this format: they would require lossy conversion or attachment-style storage, while Git preserves YAML structure, history and reviewability.

# CodeQL Action Sync Tool
![Logo](docs/logo.png)

A tool for syncing the [CodeQL Action](https://github.com/github/codeql-action/) from GitHub.com to GitHub Enterprise Server, including copying the CodeQL bundle. This allows the CodeQL Action to work even if your GitHub Enterprise Server or GitHub Actions runners do not have internet access.

**Development Status:** Ready for Production Use

## Installation
The CodeQL Action sync tool can be downloaded from the [releases page](https://github.com/github/codeql-action-sync-tool/releases/latest/) of this repository.

## Usage
The sync tool can be used in two different ways.

If you have a machine that is able to access GitHub.com and the GitHub Enterprise Server instance then simply follow the steps under ["I have a machine that can access both GitHub.com and GitHub Enterprise Server"](#i-have-a-machine-that-can-access-both-githubcom-and-github-enterprise-server).

If your GitHub Enterprise Server instance is on a completely isolated network where no machines have access to both GitHub.com and GitHub Enterprise Server then follow the steps under ["I don't have a machine that can access both GitHub.com and GitHub Enterprise Server"](#i-dont-have-a-machine-that-can-access-both-githubcom-and-github-enterprise-server) instead.

### I have a machine that can access both GitHub.com and GitHub Enterprise Server.
From a machine with access to both GitHub.com and GitHub Enterprise Server use the `./codeql-action-sync sync` command to copy the CodeQL Action and bundles.

**Required Arguments:**
* `--destination-url` - The URL of the GitHub Enterprise Server instance to push the Action to.
* `--destination-token` - A [Personal Access Token](https://docs.github.com/en/enterprise/user/github/authenticating-to-github/creating-a-personal-access-token) for the destination GitHub Enterprise Server instance. If the destination repository is in an organization that does not yet exist or that you are not an owner of, your token will need to have the `site_admin` scope in order to create the organization or update the repository in it. The organization can also be created manually or an existing organization that you own can be used, in which case the `repo` and `workflow` scopes are sufficient. The token can also be provided by setting the `CODEQL_ACTION_SYNC_TOOL_DESTINATION_TOKEN` environment variable. A GitHub App installation token can be used instead together with the `--github-app-auth` flag.

**Optional Arguments:**
* `--cache-dir` - A temporary directory in which to store data downloaded from GitHub.com before it is uploaded to GitHub Enterprise Server. If not specified a directory next to the sync tool will be used.
* `--source-token` - A token to access the API of GitHub.com. This is normally not required, but can be provided if you have issues with API rate limiting. The token does not need to have any scopes.
* `--destination-repository` - The `owner/repo` of the repository in which to create or update the CodeQL Action. If not specified `github/codeql-action` will be used.
* `--actions-admin-user` - The name of the Actions admin user, which will be used if you are updating the bundled CodeQL Action. If not specified `actions-admin` will be used.
* `--force` - By default the tool will not overwrite existing repositories. Providing this flag will allow it to.
* `--push-ssh` - Push Git contents over SSH rather than HTTPS. To use this option you must have SSH access to your GitHub Enterprise instance configured.
* `--github-app-auth` - Authenticate using a GitHub App installation token provided as the `--destination-token`, rather than a Personal Access Token. See ["Authenticating with a GitHub App"](#authenticating-with-a-github-app).

### I don't have a machine that can access both GitHub.com and GitHub Enterprise Server.
From a machine with access to GitHub.com use the `./codeql-action-sync pull` command to download a copy of the CodeQL Action and bundles to a local folder.

**Optional Arguments:**
* `--cache-dir` - The directory in which to store data downloaded from GitHub.com. If not specified a directory next to the sync tool will be used.
* `--source-token` - A token to access the API of GitHub.com. This is normally not required, but can be provided if you have issues with API rate limiting. The token does not need to have any scopes.

Next copy the sync tool and cache directory to another machine which has access to GitHub Enterprise Server.

Now use the `./codeql-action-sync push` command to upload the CodeQL Action and bundles to GitHub Enterprise Server.

**Required Arguments:**
* `--destination-url` - The URL of the GitHub Enterprise Server instance to push the Action to.
* `--destination-token` - A [Personal Access Token](https://docs.github.com/en/enterprise/user/github/authenticating-to-github/creating-a-personal-access-token) for the destination GitHub Enterprise Server instance. If the destination repository is in an organization that does not yet exist or that you are not an owner of, your token will need to have the `site_admin` scope in order to create the organization or update the repository in it. The organization can also be created manually or an existing organization that you own can be used, in which case the `repo` and `workflow` scopes are sufficient. The token can also be provided by setting the `CODEQL_ACTION_SYNC_TOOL_DESTINATION_TOKEN` environment variable. A GitHub App installation token can be used instead together with the `--github-app-auth` flag.

**Optional Arguments:**
* `--cache-dir` - The directory to which the Action was previously downloaded.
* `--destination-repository` - The name of the repository in which to create or update the CodeQL Action. If not specified `github/codeql-action` will be used.
* `--actions-admin-user` - The name of the Actions admin user, which will be used if you are updating the bundled CodeQL Action. If not specified `actions-admin` will be used.
* `--force` - By default the tool will not overwrite existing repositories. Providing this flag will allow it to.
* `--push-ssh` - Push Git contents over SSH rather than HTTPS. To use this option you must have SSH access to your GitHub Enterprise instance configured.
* `--github-app-auth` - Authenticate using a GitHub App installation token provided as the `--destination-token`, rather than a Personal Access Token. See ["Authenticating with a GitHub App"](#authenticating-with-a-github-app).

## Authenticating with a GitHub App
Instead of a Personal Access Token, the `push` and `sync` commands can authenticate to GitHub Enterprise Server using a [GitHub App installation access token](https://docs.github.com/en/enterprise-server@latest/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app).

1. Create a GitHub App on your GitHub Enterprise Server instance with the following repository permissions: `Administration` (read and write), `Contents` (read and write), `Metadata` (read-only) and `Workflows` (read and write).
2. Install the GitHub App on the organization that owns the destination repository (`github` unless you specify a different `--destination-repository`). If the destination repository already exists, make sure the installation has access to it.
3. Generate an installation access token for the installation and provide it using `--destination-token` (or the `CODEQL_ACTION_SYNC_TOOL_DESTINATION_TOKEN` environment variable), together with the `--github-app-auth` flag.

Installation access tokens are not associated with a user, so when using `--github-app-auth`:
* The destination organization must already exist, as it can't be created automatically. The destination repository can't be owned by a user account.
* The Actions admin user is not impersonated, so `--actions-admin-user` is ignored.
* Installation access tokens expire after one hour. If the token expires before the push completes, generate a new token and run the command again. Content that has already been uploaded will not be uploaded again.

## Contributing
For more details on contributing improvements to this tool, see our [contributor guide](CONTRIBUTING.md).

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
* `--destination-token` - A [Personal Access Token](https://docs.github.com/en/enterprise/user/github/authenticating-to-github/creating-a-personal-access-token) for the destination GitHub Enterprise Server instance. If the destination repository is in an organization that does not yet exist or that you are not an owner of, your token will need to have the `site_admin` scope in order to create the organization or update the repository in it. The organization can also be created manually or an existing organization that you own can be used, in which case the `repo` and `workflow` scopes are sufficient. The token can also be provided by setting the `CODEQL_ACTION_SYNC_TOOL_DESTINATION_TOKEN` environment variable.

**Optional Arguments:**
* `--cache-dir` - A temporary directory in which to store data downloaded from GitHub.com before it is uploaded to GitHub Enterprise Server. If not specified a directory next to the sync tool will be used.
* `--source-token` - A token to access the API of GitHub.com. This is normally not required, but can be provided if you have issues with API rate limiting. The token does not need to have any scopes.
* `--destination-repository` - The `owner/repo` of the repository in which to create or update the CodeQL Action. If not specified `github/codeql-action` will be used.
* `--actions-admin-user` - The name of the Actions admin user, which will be used if you are updating the bundled CodeQL Action. If not specified `actions-admin` will be used.
* `--force` - By default the tool will not overwrite existing repositories. Providing this flag will allow it to.
* `--push-ssh` - Push Git contents over SSH rather than HTTPS. To use this option you must have SSH access to your GitHub Enterprise instance configured.
* `--include-platforms` - Only download release assets for the listed platforms. Valid values are `linux64`, `linux-arm64`, `osx64`, and `win64`.
* `--exclude-platforms` - Download release assets for every platform except those listed. This cannot be used with `--include-platforms`.
* `--bundle-archive-format` - Only download CodeQL bundles in the selected format. Valid values are `tar.gz` and `tar.zst`. Requires explicit `tools` URLs in CodeQL workflows; see [Archive format restrictions](#archive-format-restrictions).

Platform lists are comma-separated. For example, the following syncs Linux x64 and Windows assets, omits the combined all-platform bundle, and downloads CodeQL bundles only as gzip archives. Use it only with workflows configured as described in [Archive format restrictions](#archive-format-restrictions):

```shell
./codeql-action-sync sync \
  --destination-url https://github.example.com \
  --include-platforms linux64,win64 \
  --bundle-archive-format tar.gz
```

To sync every platform except macOS:

```shell
./codeql-action-sync sync \
  --destination-url https://github.example.com \
  --exclude-platforms osx64
```

With none of these flags, the tool continues to copy every release asset. Platform filters also apply to platform-specific checksums, language bundles, and update-job proxies. Non-archive metadata is retained, while the combined all-platform CodeQL bundle is omitted when a platform filter is active. The archive format flag applies only to CodeQL bundles; update-job proxies retain their published format. If a required platform bundle is not published in the requested format, the command fails rather than silently falling back. When an archive format is specified, every explicitly included platform must publish a primary bundle in that format, even if the release has no assets for that platform at all.

These flags limit new downloads and uploads. They do not delete assets copied to GitHub Enterprise Server by an earlier sync.

#### Archive format restrictions

**Warning:** `--bundle-archive-format` controls which bundles are copied, not which archive the CodeQL Action requests. The Action normally chooses gzip on Windows and chooses zstd on Linux/macOS when the CLI and runner support it. If that format was excluded, the Action may try downloading it from GitHub.com, which fails on air-gapped runners. Leave this flag unset to preserve automatic archive selection.

When restricting the format, set the CodeQL init step's `tools` input to an explicit URL for a retained bundle on your destination server. For example, a Linux x64 job using a gzip-only sync:

```yaml
- uses: github/codeql-action/init@v4
  with:
    languages: javascript
    tools: https://github.example.com/github/codeql-action/releases/download/<bundle-tag>/codeql-bundle-linux64.tar.gz
```

Replace `<bundle-tag>` with a synced release tag compatible with your Action version, and adjust the host, repository, platform, and archive format for each job. The explicit URL pins the bundle version, so update it when upgrading the bundle. Ensure each runner can extract the selected archive format.

### I don't have a machine that can access both GitHub.com and GitHub Enterprise Server.
From a machine with access to GitHub.com use the `./codeql-action-sync pull` command to download a copy of the CodeQL Action and bundles to a local folder.

**Optional Arguments:**
* `--cache-dir` - The directory in which to store data downloaded from GitHub.com. If not specified a directory next to the sync tool will be used.
* `--source-token` - A token to access the API of GitHub.com. This is normally not required, but can be provided if you have issues with API rate limiting. The token does not need to have any scopes.
* `--include-platforms` - Only download release assets for the listed platforms. Valid values are `linux64`, `linux-arm64`, `osx64`, and `win64`.
* `--exclude-platforms` - Download release assets for every platform except those listed. This cannot be used with `--include-platforms`.
* `--bundle-archive-format` - Only download CodeQL bundles in the selected format. Valid values are `tar.gz` and `tar.zst`. Requires explicit `tools` URLs in CodeQL workflows; see [Archive format restrictions](#archive-format-restrictions).

The filtering semantics are the same as for `sync` above. Reusing a cache with different filters removes now-excluded local assets before the cache can be pushed, including assets from historical releases no longer referenced by the current Action defaults. Historical release metadata and matching assets are retained.

Next copy the sync tool and cache directory to another machine which has access to GitHub Enterprise Server.

Now use the `./codeql-action-sync push` command to upload the CodeQL Action and bundles to GitHub Enterprise Server.

**Required Arguments:**
* `--destination-url` - The URL of the GitHub Enterprise Server instance to push the Action to.
* `--destination-token` - A [Personal Access Token](https://docs.github.com/en/enterprise/user/github/authenticating-to-github/creating-a-personal-access-token) for the destination GitHub Enterprise Server instance. If the destination repository is in an organization that does not yet exist or that you are not an owner of, your token will need to have the `site_admin` scope in order to create the organization or update the repository in it. The organization can also be created manually or an existing organization that you own can be used, in which case the `repo` and `workflow` scopes are sufficient. The token can also be provided by setting the `CODEQL_ACTION_SYNC_TOOL_DESTINATION_TOKEN` environment variable.

**Optional Arguments:**
* `--cache-dir` - The directory to which the Action was previously downloaded.
* `--destination-repository` - The name of the repository in which to create or update the CodeQL Action. If not specified `github/codeql-action` will be used.
* `--actions-admin-user` - The name of the Actions admin user, which will be used if you are updating the bundled CodeQL Action. If not specified `actions-admin` will be used.
* `--force` - By default the tool will not overwrite existing repositories. Providing this flag will allow it to.
* `--push-ssh` - Push Git contents over SSH rather than HTTPS. To use this option you must have SSH access to your GitHub Enterprise instance configured.

## Contributing
For more details on contributing improvements to this tool, see our [contributor guide](CONTRIBUTING.md).

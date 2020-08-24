# Contributing
Hi there! We're thrilled that you'd like to contribute to this project. Your help is essential for keeping it great.

Contributions to this project are [released](https://help.github.com/articles/github-terms-of-service/#6-contributions-under-repository-license) to the public under the [project's open source license](LICENSE.md).

Please note that this project is released with a [Contributor Code of Conduct](CODE_OF_CONDUCT.md). By participating in this project you agree to abide by its terms.

Here are a few things you can do that will increase the likelihood of your pull request being accepted:
* Ensure your code is `go fmt` formatted and does not contain any unused imports.
* If you introduce any new dependencies, use [licensed](https://github.com/github/licensed/)'s `cache` command to ensure their licenses are included in the repository and compiled binaries.
* Write tests.
* Keep your change as focused as possible. If there are multiple changes you would like to make that are not dependent upon each other, consider submitting them as separate pull requests.

## Releasing New Versions
Repository maintainers can release new versions by following this procedure:
1. Tag the new version with a three-part semver version number.
2. Push the tag.
3. Ensure the GitHub Actions run triggered by this push completes successfully.

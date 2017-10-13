# Contributing

When contributing to this repository, please follow the guidelines below. Any pull requests opened that do not follow these guidelines will not be reviewed.

## Pull Requests

### Branch Naming

We use a centralized single repository workflow with this Github repository. This means that all development will take place in this repository. In order to clarify and differenciate individual development branches, all branches pushed to this repository must start with the username or initials of the author. Example: `tt/docker-integration` as opposed to `docker-integration`.

### Rebasing

Before a pull request is opened the commit history of that branch must be squashed and cleaned. During the development process many git commits may have taken place as the final branch was changed. This is difficult to read through during the pull request review process and even harder to understand at a later time when looking back through too many commits. For this reason when a pull request is open it should have no 'merge' commits where a branch was updated from master. Instead [rebase](https://git-scm.com/docs/git-rebase) from an updated master branch. In addition the many commits that are part of the development process should be [squashed](http://gitready.com/advanced/2009/02/10/squashing-commits-with-rebase.html) to larger, atomic, clear commits that clearly demonstrate the changes in that branch.

### Issues

All pull requests should be linked to an issue. Issues are the places where we can make sure that a change is needed and agree on the path forward. A pull request without an existing issue can be a waster of time for the author and the reviewer.

### Descriptions

Pull requests should be opened with a clear description of what change is being made, to which files, for what reason and how a reviewer might validate this change. In addition any related issues or other pull requests should be linked in the description. [This blog post](https://quickleft.com/blog/pull-request-templates-make-code-review-easier/) describes a pull request template and workflow that accomplishes these goals. While we do not require this template, you may find it helpful in organizing your pull request.

## Commits

Commits are the way that we quickly communicate exactly which changes happened, where they happened and why. This is difficult to do in a short message in a log of commits. In order to have the most clear and readable commits we use the following rules:

* Capitalize the subject line
* Do not end the subject line with a period
* Use the imperative mood in the subject line
* Separate subject from body with a blank line
* Limit the subject line to 50 characters
* Wrap the body at 72 characters
* Use the body to explain what and why vs. how

The first three of these are always applicable. The imperative mood means that all commit subject lines should start with words like 'Remove', 'Add', 'Refactor' and 'Fix'. While a commit message body is not always required it can be helpful in further explaining a complicated change. These rules are taken from [How to Write a Git Commit Message](https://chris.beams.io/posts/git-commit/).

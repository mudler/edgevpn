
---
title: "Contributing"
linkTitle: "Contribution guidelines"
weight: 159
description: >
  See how to contribute to EdgeVPN
---

## Contributing to EdgeVPN
Contribution guidelines for the EdgeVPN project are on the [Github repository](https://github.com/mudler/edgevpn/blob/master/CONTRIBUTING.md). Here you can find some heads up for contributing to the documentation website.

## Contributing to the Docs website

### We Develop with Github
We use [github to host code](https://github.com/mudler/edgevpn), to track issues and feature requests, as well as accept pull requests.

We use [Hugo](https://gohugo.io/) to format and generate our website, the
[Docsy](https://github.com/google/docsy) theme for styling and site structure, 
and Github Actions to manage the deployment of the site. 
Hugo is an open-source static site generator that provides us with templates, 
content organisation in a standard directory structure, and a website generation 
engine. You write the pages in Markdown (or HTML if you want), and Hugo wraps them up into a website.

All submissions, including submissions by project members, require review. We
use GitHub pull requests for this purpose. Consult
[GitHub Help](https://help.github.com/articles/about-pull-requests/) for more
information on using pull requests.

### Any contributions you make will be under the Software License of the repository
In short, when you submit code changes, your submissions are understood to be under the same License that covers the project. Feel free to contact the maintainers if that's a concern.

### Updating a single page

If you've just spotted something you'd like to change while using the docs, Docsy has a shortcut for you:

1. Click **Edit this page** in the top right hand corner of the page you want to modify.
2. If you don't already have an up to date fork of the project repo, you are prompted to get one - click **Fork this repository and propose changes** or **Update your Fork** to get an up to date version of the project to edit. The appropriate page in your fork is displayed in edit mode.


### Quick start with a local checkout

Here's a quick guide to updating the docs with a git local checkout. It assumes you're familiar with the
GitHub workflow and you're happy to use the automated preview of your doc
updates:

1. Fork the [the repo](https://github.com/mudler/edgevpn) on GitHub.
2. Make your changes, if are related to docs
   to see the preview run `make serve` from the `docs` dir, then browse to [localhost:1313](http://localhost:1313)
3. If you're not yet ready for a review, add "WIP" to the PR name to indicate 
  it's a work in progress.
4. Continue updating your doc and pushing your changes until you're happy with 
  the content.
5. When you're ready for a review, add a comment to the PR, and remove any
  "WIP" markers.
6. When you are satisfied send a pull request (PR).

### License
By contributing, you agree that your contributions will be licensed under the project Licenses.

# Git/Phabricator Mirror

This repo contains code that automatically mirrors source code metadata between
git-notes and Phabricator.

## Setup

This tool expects to run in an environment with the following attributes:

1.  The "arcanist" command line tool is installed and included in the PATH.
2.  A ".arcrc" file is located where arcanist will find it, specifies a URL for
    a Phabricator instance which should be used for any repos that do not
    override the default instance, and provides auth credentials for every
    Phabricator instance that is used.
3.  The current working directory contains a clone of every git repo that needs
    to be mirrored.
4.  The git command line tool is installed, and included in the PATH.
5.  The git command line tool is configured with the credentials it needs to
    push to the remotes for all of those repos.
6.  The "mysql" command line tool is installed, and has been preconfigured with
    the IP address, username, and password necessary to connect to the
    Phabricator database. This is a (hopefully temporary) workaround to the
    fact that the Phabricator API does not yet support querying revision
    transactions.

## Installation

Assuming you have the [Go tools installed](https://golang.org/doc/install), run the following command:

    go get github.com/google/git-phabricator-mirror/git-phabricator-mirror

## Metadata

The source code metadata is stored in git-notes, using the formats described
[here](https://github.com/google/git-appraise#metadata).
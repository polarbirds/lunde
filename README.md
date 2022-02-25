[![Build Status](https://travis-ci.org/polarbirds/lunde.svg?branch=master)](https://travis-ci.org/polarbirds/lunde)

![Puffin lol](https://store.audubon.org/sites/default/files/styles/product_bubble/public/images/plushes/puffin-plush.jpg)


## Usage

* !reddit [sort] [subreddit]
* !pumpit
* !status [new status]

## Setup secrets locally (to avoid accidentally pushing them :P)
* Duplicate `dev-cfg/cfg.yml`
* Rename duplicate to `cfg-develop.yml`. This file is gitignored.
* Rename filename to `cfg-develop.yml` in the run-command in `Makefile`
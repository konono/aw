# Changelog

## [1.4.0](https://github.com/konono/aw/compare/v1.3.0...v1.4.0) (2026-06-08)


### Features

* add pre-built image support for air-gapped environments ([726b191](https://github.com/konono/aw/commit/726b19110fcac38129ea839d88af7b6c2d669691))
* **cmd:** add `aw export` command for building and saving images ([d49fa44](https://github.com/konono/aw/commit/d49fa44a12b57c5ff7c40c8e633c5b0a04e3b636))
* **docker:** add ImageExists and Save methods to Client interface ([7473880](https://github.com/konono/aw/commit/7473880c9333c22f96d43b0d35748e53115fb4b7))
* **export:** add --apply flag and fix snapshot ro mount issue ([9c021bc](https://github.com/konono/aw/commit/9c021bcba583acc82683e8deb26b27ebcacc6b63))
* **export:** add --snapshot, --include, --env flags for pre-built image export ([4a385c4](https://github.com/konono/aw/commit/4a385c42922282a075456e7e4ff3ef5d897c2eba))
* **profile:** add `image` field for pre-built container images ([9692d45](https://github.com/konono/aw/commit/9692d45336d6e87c08e36f35908015c0f28dbd4a))
* **profile:** add FindProfileSource to locate defining config file ([b46030d](https://github.com/konono/aw/commit/b46030d9942ffc724a51b1e42c5c5c5d7899b9c9))
* **profile:** add image to trust system and migration support ([d1c6737](https://github.com/konono/aw/commit/d1c6737a6ac35ee37459426031855378aa6e629d))
* **profile:** add skip_devbox_install and skip_mise_install options ([7b412dc](https://github.com/konono/aw/commit/7b412dcf9dfbb69886183173a70d977170c382ad))
* **stage:** skip build when pre-built image is specified ([d66f25c](https://github.com/konono/aw/commit/d66f25c60acef4146c1894eaca474a134a061346))


### Bug Fixes

* **cmd:** quote image name in export config snippet ([b6f16fb](https://github.com/konono/aw/commit/b6f16fbcfded00b7e611fe508f247890ec5bce6c))
* **docker:** preserve inspect errors for pre-built images ([524fb7a](https://github.com/konono/aw/commit/524fb7abdc8589a814d9e528960cbacf96ae1373))
* **profile:** allow image to coexist with os and dockerfile ([4d106b8](https://github.com/konono/aw/commit/4d106b8e90754840ffb90df197c0fa5ad7f0c0e6))
* **test:** check os.WriteFile errors in applyExportResult tests ([5e6b677](https://github.com/konono/aw/commit/5e6b6773018dfe54631c0fe3832067c8c39b15ec))
* **test:** check os.WriteFile errors in FindProfileSource tests ([1bfa2cf](https://github.com/konono/aw/commit/1bfa2cfa1ae3b8af65b995f2968caf02aa2e1bb2))

## [1.3.0](https://github.com/konono/aw/compare/v1.2.0...v1.3.0) (2026-06-01)


### Features

* **init:** add --update flag for config migration ([d16243a](https://github.com/konono/aw/commit/d16243ab38e653dd465d156b964694d3d6918b25))


### Bug Fixes

* resolve merge conflict with main branch ([9609fed](https://github.com/konono/aw/commit/9609fededaf1b00a876c3657382f083156d50f8e))

## [1.2.0](https://github.com/konono/aw/compare/v1.1.1...v1.2.0) (2026-05-24)


### Features

* support global mise.toml in ~/.config/aw/ ([eca2022](https://github.com/konono/aw/commit/eca2022b32fccda0f03fdd40c42a9565e9dd724e))
* support global mise.toml in ~/.config/aw/ ([216dd2b](https://github.com/konono/aw/commit/216dd2b1a487686a784f1a391ee8dcec94e73edd))

## [1.1.1](https://github.com/konono/aw/compare/v1.1.0...v1.1.1) (2026-05-19)


### Bug Fixes

* **ssh:** fix SSH agent forwarding reliability ([b18f508](https://github.com/konono/aw/commit/b18f508a993a0a79d4aef4f3d7c3819f8f862881))

## [1.1.0](https://github.com/konono/aw/compare/v1.0.1...v1.1.0) (2026-05-19)


### Features

* harden container security against prompt injection ([863997d](https://github.com/konono/aw/commit/863997d53ced742ae358d13d09fc7526d3970d49))


### Bug Fixes

* remove unused sensitiveFieldLabels variable ([70ff2b0](https://github.com/konono/aw/commit/70ff2b0bf426dc31a7992ec843a1a818615a8caa))
* remove unused syncDirIfExists replaced by syncDirOrRemove ([81dd88b](https://github.com/konono/aw/commit/81dd88beebf0d1034575037c592ad1db2f046b2e))
* use sensitiveFieldDescriptions map in trust prompt ([1328624](https://github.com/konono/aw/commit/13286248fce9a78c1f5ec9768d72064d26493c75))

## [1.0.1](https://github.com/konono/aw/compare/v1.0.0...v1.0.1) (2026-05-19)


### Bug Fixes

* check os.Chdir error return in tests to satisfy errcheck ([110dbd1](https://github.com/konono/aw/commit/110dbd1fcc67f371356f196e584db84283e2457b))
* rename commitlint config to .mjs for ESM compatibility ([9b13588](https://github.com/konono/aw/commit/9b135880d41621bd09b5a38c8ca65d43f0ad2c99))

## Changelog

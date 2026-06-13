# Changelog

## [2.0.0](https://github.com/konono/aw/compare/v1.8.0...v2.0.0) (2026-06-13)


### ⚠ BREAKING CHANGES

* devbox.json user configuration is no longer supported. Use mise.toml for additional tool management.

### Features

* add -c flag to override launch command ([f78e977](https://github.com/konono/aw/commit/f78e9777ac7cc0884437cbc1cdddfe68a3c211ef))
* add binary download support for opencode tool installation ([24ae02d](https://github.com/konono/aw/commit/24ae02ddbb85c0cfb9e4d02fcdb5260fc83aaecd))
* add gh_token option to replace mount_gh ([15991ac](https://github.com/konono/aw/commit/15991ac72d386669d85aa263113e88effdcf311a))
* add package_manager field to support both apt and devbox modes ([b30b9e4](https://github.com/konono/aw/commit/b30b9e4d0fd7b9cb3b16e44c18a2947f69f8a36a))
* configure git credential helper when GITHUB_TOKEN is set ([20b5be5](https://github.com/konono/aw/commit/20b5be596f6f12575bfc8fe3f046f53bb0079a36))
* replace Nix/devbox with apt/npm for tool installation ([94e3ab6](https://github.com/konono/aw/commit/94e3ab6f9a27d978a7a8cfe810c3cb5571948491))
* switch from npm to standalone installers for all tools ([d4ac971](https://github.com/konono/aw/commit/d4ac971811887ec291697d48bbdebc1b33a97b76))


### Bug Fixes

* add gawk to Dockerfile templates for codex install.sh compatibility ([7b171f0](https://github.com/konono/aw/commit/7b171f0054bfdb0923595e833f39e7055ef72bac))
* handle tar.gz binary downloads and fix -c flag edge cases ([9434e7c](https://github.com/konono/aw/commit/9434e7cea313bfd64f17d5bf3cf2d35e687efa66))
* resolve lint issues and remove macOS Podman from CI ([fdae5d3](https://github.com/konono/aw/commit/fdae5d35fc48d70df8555058058aa6b11da7c9d3))
* update CLAUDE.md injection to reflect standalone installer architecture ([a58b0f6](https://github.com/konono/aw/commit/a58b0f6aefa988110192fa62fbb34fb1ea88e89a))
* use ~/.npm-global prefix for npm global installs ([67d4d65](https://github.com/konono/aw/commit/67d4d65ebd21d96d97f5da2dc8f38ca50d1603d5))
* use env-based git credential helper instead of git config ([e85250a](https://github.com/konono/aw/commit/e85250a5868a24df471c539ce84a9a9b6ef5089d))
* use official install scripts for codex and opencode ([23e0010](https://github.com/konono/aw/commit/23e0010a999c7a67c068582a9c2760d937ac29ed))


### Performance Improvements

* consolidate Dockerfile layers to reduce image size ([fa312e7](https://github.com/konono/aw/commit/fa312e729c31404424e1d44f247de43e4d14a957))

## [1.8.0](https://github.com/konono/aw/compare/v1.7.0...v1.8.0) (2026-06-12)


### Features

* adopt OpenShift-style GID 0 pattern for UID-independent container images ([bc704b6](https://github.com/konono/aw/commit/bc704b6bda555ceb8d76149a9106385d7aca252c))
* adopt OpenShift-style GID 0 pattern for UID-independent container images ([ca33d7c](https://github.com/konono/aw/commit/ca33d7cd135b92158befb38d4e896f813e4b9c7e))


### Bug Fixes

* correct error messages from "environment: docker" to "environment: container" ([40a2eaa](https://github.com/konono/aw/commit/40a2eaad4433ffa6eab5013c50e1f0d0b6fcc1c9))
* fix Nix permission errors when running with arbitrary UID ([89fd960](https://github.com/konono/aw/commit/89fd960ec1015f78e3af52483fa7f9f011a188ae))
* **ssh:** remove duplicate passwd entries injected by Podman --userns=keep-id ([c504f60](https://github.com/konono/aw/commit/c504f60eaf2441971ec48b29dec4ca74216b1983))
* **ssh:** remove duplicate passwd entries injected by Podman --userns=keep-id ([530faf2](https://github.com/konono/aw/commit/530faf2c1cb94b9cff7ffadce8502e45b74ee2d4))

## [1.7.0](https://github.com/konono/aw/compare/v1.6.0...v1.7.0) (2026-06-11)


### Features

* add rootless Podman support with SELinux handling ([29a0074](https://github.com/konono/aw/commit/29a0074aa37b76fd58f90d07578e4990a7199c9f))

## [1.6.0](https://github.com/konono/aw/compare/v1.5.0...v1.6.0) (2026-06-11)


### Features

* **entrypoint:** add progress logging for startup phases ([b434550](https://github.com/konono/aw/commit/b4345506ffbd4088aaf9050f37f2d147ad28fc5b))


### Bug Fixes

* **container:** add AUDIT_WRITE capability to suppress sudo audit errors on RHEL10 ([b274519](https://github.com/konono/aw/commit/b274519fb93d986b43a22358110fd6d5d702a4e1))
* **container:** add AUDIT_WRITE capability to suppress sudo audit errors on RHEL10 ([b8c2780](https://github.com/konono/aw/commit/b8c2780dbe65d53a74ac6b374e5b6e142fcd463c))
* forward signals to container process and add --init flag ([832a5a7](https://github.com/konono/aw/commit/832a5a7c64ce44f008102f9e9746318b81744dd7))
* forward signals to container process and add --init flag ([5452051](https://github.com/konono/aw/commit/5452051e43bc8b416d9417413354617e39923a5e))

## [1.5.0](https://github.com/konono/aw/compare/v1.4.5...v1.5.0) (2026-06-09)


### Features

* add --recent, -C/--cwd, and directory history ([4663a12](https://github.com/konono/aw/commit/4663a1249138147ee4856c672ae7698fcbf7b78c))
* add --recent, -C/--cwd, and directory history for quick project switching ([47b14e4](https://github.com/konono/aw/commit/47b14e4a1e589b7ff2d631981edbe4adf2fdd4ab))


### Bug Fixes

* handle errcheck lint errors in dirhistory tests ([94da255](https://github.com/konono/aw/commit/94da2555dd079bab38486104edcedfa547be09ca))

## [1.4.5](https://github.com/konono/aw/compare/v1.4.4...v1.4.5) (2026-06-09)


### Bug Fixes

* **image:** chmod devbox binary after root install ([6f73d77](https://github.com/konono/aw/commit/6f73d778ff7ec27f1431dfeda3b4e7cf9370ace8))
* **image:** chown ~/.local/share for both devbox and mise ([2b3849a](https://github.com/konono/aw/commit/2b3849a3b6360eeb72b80251ee284a58311a2448))
* **image:** chown ~/.local/share for devbox and mise permissions ([1156779](https://github.com/konono/aw/commit/11567794cbb0e6bb4fc37d56d08091d3a7a4a8c0))
* **image:** install devbox as root to avoid sudo PAM failure on UBI ([f9b4f64](https://github.com/konono/aw/commit/f9b4f64df166cecb60fa9715931dbf6dda3fed92))
* **image:** use devbox launcher instead of installer to avoid sudo ([36b890d](https://github.com/konono/aw/commit/36b890dade45bbb6dc1a310eb64a43b76d4d253c))

## [1.4.4](https://github.com/konono/aw/compare/v1.4.3...v1.4.4) (2026-06-09)


### Bug Fixes

* **image:** create devbox global directory before copying devbox.json ([c85859b](https://github.com/konono/aw/commit/c85859bc53c453c2c65fd449d93dc0bd7a85a431))
* **image:** create devbox global directory before copying devbox.json ([1d64359](https://github.com/konono/aw/commit/1d64359fcdd454901c7ef3698bdb6757e276d96e))

## [1.4.3](https://github.com/konono/aw/compare/v1.4.2...v1.4.3) (2026-06-09)


### Bug Fixes

* **container:** install user devbox.json packages globally ([eb1aa89](https://github.com/konono/aw/commit/eb1aa89672d33e0265c6ec4ab89ae94e11a32bd8))
* **container:** make appendContainerContext idempotent to prevent CLAUDE.md bloat ([5643f32](https://github.com/konono/aw/commit/5643f320251f21ebc10b64f143d15aba8d23a31f))
* **container:** make appendContainerContext idempotent to prevent CLAUDE.md bloat ([4171304](https://github.com/konono/aw/commit/41713049159756c675781995ce4729fd204b2a43))

## [1.4.2](https://github.com/konono/aw/compare/v1.4.1...v1.4.2) (2026-06-09)


### Bug Fixes

* **container:** increase --pids-limit from 1000 to 8192 ([062f794](https://github.com/konono/aw/commit/062f794237c64c83806c30e3c3f3ca520c3261ea))
* **container:** increase --pids-limit from 1000 to 8192 ([92e08a9](https://github.com/konono/aw/commit/92e08a92a3f46b16af7ac50a8eb046986533056a))
* **test:** check os.WriteFile error returns in config tests ([2ea9c4a](https://github.com/konono/aw/commit/2ea9c4afc5eff2dec52bad206b52b0a264969f9d))

## [1.4.1](https://github.com/konono/aw/compare/v1.4.0...v1.4.1) (2026-06-08)


### Bug Fixes

* **export:** allow re-export when image is already set in profile ([b100890](https://github.com/konono/aw/commit/b1008908a35c057aeccbc6667b43b2810a74eafe))
* **export:** bypass entrypoint.sh during snapshot ([dc388b4](https://github.com/konono/aw/commit/dc388b4b874e24871e032924fd987941aec1ecd1))
* **export:** bypass entrypoint.sh during snapshot to avoid ro filesystem error ([4e04663](https://github.com/konono/aw/commit/4e04663f8cbe68780dd10c6331084011ff0d6e5f))
* **export:** restore ENTRYPOINT and CMD after snapshot commit ([fc3bcb1](https://github.com/konono/aw/commit/fc3bcb1b13299c3801cd2f608d97e3a3baf0f3ff))
* **export:** use 2-space indent for YAML output in --apply ([499c67a](https://github.com/konono/aw/commit/499c67a98154afaecb6b432e83cce855b9cfbffd))

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

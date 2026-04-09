# Changelog

## [1.4.0](https://github.com/saintedlama/archscout/compare/v1.3.0...v1.4.0) (2026-04-09)


### Features

* add filtering methods for exported/unexported types, functions, and variables; enhance collection capabilities with regex matching ([fdcbf86](https://github.com/saintedlama/archscout/commit/fdcbf86ec2d5d8e6e4c965827423904ce054e222))
* add PackageGraph for managing workspace-internal package dependencies; implement related methods and tests ([0668179](https://github.com/saintedlama/archscout/commit/06681795df4f97ca304e24d1a2bfa47cbfbff48c))

## [1.3.0](https://github.com/saintedlama/archscout/compare/v1.2.0...v1.3.0) (2026-04-09)


### Features

* add unique targets and source packages methods for dependency collections ([14e9d8c](https://github.com/saintedlama/archscout/commit/14e9d8cd22ec36ad2a8fe0c3e5fcae86286a685e))

## [1.2.0](https://github.com/saintedlama/archscout/compare/v1.1.0...v1.2.0) (2026-04-09)


### Features

* add Module type for generating fully-qualified package patterns and enhance rule evaluation with existence checks ([3bb9967](https://github.com/saintedlama/archscout/commit/3bb9967ef7c52e308c12c46d2685cf769db41e5c))

## [1.1.0](https://github.com/saintedlama/goarch/compare/v1.0.0...v1.1.0) (2026-04-01)


### Features

* add dependency tracking and rules for architectural validation ([6effa56](https://github.com/saintedlama/goarch/commit/6effa56d9007ed21af5a5a91bbd317cf826ea3dc))
* add filters for cleaner tests ([d316670](https://github.com/saintedlama/goarch/commit/d3166709297d36f50da19ea81d7be502342ad1b9))
* add workspace independent rules ([867cadb](https://github.com/saintedlama/goarch/commit/867cadbcb27b871d3f452b53a8c754351576dfe1))
* enhance workspace management with immutable collections and ref formatting options ([4ed6e49](https://github.com/saintedlama/goarch/commit/4ed6e4958242a827f24bd9b80f6b2308e3ddf5ef))
* implement TreeNode structure and Tree method for dependency hierarchy ([3b3c068](https://github.com/saintedlama/goarch/commit/3b3c068b12128a4f32ae980d9ac174695c51260d))
* provide an in memory cache and option to avoid loading the workspace inbetween tests ([98847ec](https://github.com/saintedlama/goarch/commit/98847ec3913f03da13b474055cc336440a0df0ce))
* update ref formatting options to support newline as default separator and add WithoutSeparator option ([e45b0b2](https://github.com/saintedlama/goarch/commit/e45b0b2485fdd837247bb2c423277ed3b57e9857))


### Bug Fixes

* ensure newline at end of file in format_test.go ([761a0e2](https://github.com/saintedlama/goarch/commit/761a0e2e4ce2d2f1476e1f80ced8f26d76e281c9))

## 1.0.0 (2026-03-29)


### Features

* introduce files accessor in workspace ([00a8fea](https://github.com/saintedlama/archscout/commit/00a8fea4bc9d4ad50a7eed263546bff24a642121))
* simplify matchers ([79d1c8f](https://github.com/saintedlama/archscout/commit/79d1c8f0e0b45279be0f493d20faf3ccc632c379))

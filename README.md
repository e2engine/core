# E2Engine Core

Core domain model, execution logic, and public APIs for E2Engine.

This module provides the shared models and execution components used across E2Engine, including test and environment definitions, test execution, runtime service management, scheduling, persistence interfaces, and transport primitives.

## Installation

```bash
go get github.com/e2engine/core
```

## Packages

**api**

Public E2Engine service API for working with environments, tests, test suites, and execution.

**model**

Core E2Engine domain models, validation, and shared types.

**repository**

E2Engine repositories contracts for persistence of environments, tests, test suites, and execution state.

**execute**

Test execution, evaluation, persistence, runtime management, and workers.

**scheduler**

Test job scheduling and publishing.

**transport**

Transport primitives used by E2Engine components.

## Development

Run tests:

```bash
make test
```

Run the linter:

```bash
make lint
```

Run race detection:

```bash
make test-race
```

Run the complete verification suite:

```bash
make verify
```

## E2Engine

This repository is part of E2Engine.

- [core](https://github.com/e2engine/core) — core domain model, execution logic, and public APIs
- [repository](https://github.com/e2engine/repository) — persistence implementations
- [runner-local](https://github.com/e2engine/runner-local) — local test execution
- [cli](https://github.com/e2engine/cli) — command-line interface
- [tests](https://github.com/e2engine/tests) — end-to-end tests for E2Engine
- demo — executable demonstration system and E2Engine usage examples

## License

Licensed under the Apache License, Version 2.0.
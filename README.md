# E2Engine Core

Core domain model, execution logic, and public APIs for E2Engine.

Part of [E2Engine](https://e2engine.dev), an open-source platform for declarative end-to-end testing.

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

E2Engine repository contracts for persistence of environments, tests, test suites, and execution state.

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

## Links

- [E2Engine website](https://e2engine.dev)
- [E2Engine documentation](https://e2engine.dev/docs)
- [E2Engine on GitHub](https://github.com/e2engine)

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE).
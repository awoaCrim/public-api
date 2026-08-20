# Backend Development Guidelines

> Best practices for backend development in this project.

---

## Overview

This directory contains project-specific backend guidelines for the Go/Gin/GORM application. The documents describe conventions that are already present in the repository and executable contracts that should be preserved when changing backend code.

---

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Directory Structure](./directory-structure.md) | Module organization and file layout | Documented |
| [Database Guidelines](./database-guidelines.md) | ORM patterns, queries, migrations | Documented |
| [Deployment Guidelines](./deployment-guidelines.md) | Committed archive deployment, health checks, and rollback | Documented |
| [Error Handling](./error-handling.md) | Error types, handling strategies | Documented |
| [Quality Guidelines](./quality-guidelines.md) | Code standards, forbidden patterns | Documented |
| [Logging Guidelines](./logging-guidelines.md) | Structured logging, log levels | Documented |

---

## Maintenance

When a feature establishes a reusable cross-layer contract, update the owning guideline with concrete signatures, payload fields, validation/error behavior, test assertions, and a wrong-versus-correct example. Keep the guidance tied to real paths and symbols; do not add aspirational rules without an implementation boundary.

**Language**: All documentation should be written in **English**.

# Migrations

Managed by [golang-migrate](https://github.com/golang-migrate/migrate). Files are embedded into the binary at build time.

## Naming convention

```
{version}_{description}.up.sql    ← applies the change
{version}_{description}.down.sql  ← reverts the change
```

Version must be a sequential integer with leading zeros: `001`, `002`, `003`, ...

## Example

```
001_create_messages.up.sql
001_create_messages.down.sql
002_add_model_column.up.sql
002_add_model_column.down.sql
```

## Rules

- Never edit or delete an existing migration — create a new one instead.
- `up` and `down` must be inverses of each other.
- Always provide a `down` file, even if rollback is unlikely.
- `down` files run in reverse order (003 → 002 → 001).

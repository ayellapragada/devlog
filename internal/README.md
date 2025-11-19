# internal/

This directory contains the core internal packages that power DevLog.
These packages are not intended to be imported by external code.

This code is expected to change rapidly and will not have other README.mds maintained.

## Key Dependencies

- **Standard Library:** Extensive use of `context`, `net/http`, and `database/sql`
- **SQLite:** Uses `modernc.org/sqlite` for database operations
- **UUID:** Uses `github.com/google/uuid` for unique identifiers

## Architecture Flow

```
┌─────────────┐
│   daemon    │  Coordinates components
└──────┬──────┘
       │
   ┌───┴────┬────────────┐
   │        │            │
┌──▼──┐  ┌─▼──┐    ┌────▼────┐
│ api │  │cfg │    │ storage │
└─────┘  └────┘    └─────────┘
   │                     │
   ▼                     ▼
┌────────┐          ┌────────┐
│ events │◄─────────┤session │
└────────┘          └────────┘
```

1. **daemon** initializes all components
2. **api** receives HTTP requests and validates events
3. **events** defines the data model
4. **storage** persists to SQLite
5. **session** groups events into sessions
6. **config** provides configuration to all components

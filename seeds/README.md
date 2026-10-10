# Founding agent templates

`founding_agents.sql` bootstraps active Priest, Scientist, Soldier, and Historian
templates in the existing Tiger Data application database. Each has a distinct
personality prompt and four starting beliefs: three `held` and one `forming`.
The prompts cover evidence and memory, Observer/God/Messenger relationships,
reactions and rebuttals, belief revisions, tradition support, and councils. The
Historian also covers discovery, council, and closing chronicles.

Apply the application schema migrations first. With the Tiger Data connection
loaded into `DATABASE_URL`, run from the repository root:

```bash
psql "$DATABASE_URL" -X -v ON_ERROR_STOP=1 -f seeds/founding_agents.sql
```

All inserts run in one transaction. Repeating the seed creates no duplicates:
agent types with an active template are skipped. Missing types receive the next
version number and ordered initial beliefs. Existing active and historical
template versions, game-specific agents, beliefs, and traditions are preserved.
The bootstrap leaves the nullable editor account unset rather than inventing an
administrator identity.

New civilizations copy the active prompts and beliefs through the existing game
creation transaction. The seed does not create worlds, grant account permissions,
or start workflows. Subsequent personality or belief changes belong in new
template versions created through the admin portal.

# Schema Updates

After stopping the Goliath binary, apply schema updates:

```bash
$ VERSION=<latest schema version>
$ cockroach sql --insecure --database=Goliath < schema/${VERSION}.sql
```
# Schema Updates

After stopping the Goliath binary, apply schema updates:

```bash
$ VERSION=<latest schema version>
$ cockroach --insecure --database=Goliath < ${VERSION}.sql
```
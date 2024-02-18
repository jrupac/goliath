## Example Operations

## Schema Updates

After stopping the Goliath binary, apply schema updates:

```bash
$ VERSION=<latest schema version>
$ cockroach sql --insecure --database=Goliath < schema/${VERSION}.sql
```

## Example Queries

### Delete all articles associated with a feed

```cockroach
DELETE
from article
where feed = < id >;
```

### Reset last retrieved timestamp of feed (to force refresh)

```cockroach
UPDATE feed
set latest = TIMESTAMPTZ '1970-01-01'
WHERE id = < id >;
```

### Update feed URL of existing feed

```cockroach
UPDATE feed
set url = '<URL>'
where id = < id >;
```
# Example Operations

## Schema Updates

After stopping the Goliath binary, apply schema updates:

```bash
$ VERSION=<latest schema version>
$ ./cockroach sql --insecure --database=Goliath < scripts/${VERSION}.sql
```

### Docker

If running in Dockerized mode, start the containers and then attach to the
running CockroachDB container and execute:

```bash
$ ./goliath.sh up

# In a separate shell
$ docker exec -it crdb-service /bin/bash
[root@crdb cockroach] $ ./cockroach sql --insecure < scripts/${VERSION}.sql
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
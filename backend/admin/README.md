# Administrative Operations

## Admin Server gRPC

### Feed Management

#### Get all feeds

```shell
$ grpc_cli call <URL> AdminService.GetFeeds 'Username: "<username>"'
```

#### Add new feed

```shell
$ grpc_cli call <URL> AdminService.AddFeed <<EOF
Username: "<username>"
Title: "<title>"
URL: "<URL of feed>"
Link: "<URL of homepage>"
Folder: "<folder>"
EOF
```

#### Remove a feed

```shell
$ grpc_cli call <URL> AdminService.RemoveFeed <<EOF
Username: "<username>"
Id: <id>
EOF
```

#### Edit a feed

```shell
$ grpc_cli call <URL> AdminService.EditFeed <<EOF
Username: "<username>"
Id: <id>
Folder: "<Name of folder>"
EOF
```

### User Preferences

#### Get mute words

```shell
$ grpc_cli call <URL> AdminService.GetMuteWords 'Username: "<username>"'
```

#### Add mute words

```shell
$ grpc_cli call <URL> AdminService.AddMuteWord <<EOF
Username: "<username>"
MuteWord: "<word>"
EOF
```

#### Delete mute words

```shell
$ grpc_cli call <URL> AdminService.DeleteMuteWord <<EOF
Username: "<username>"
MuteWord: "<word>"
EOF
```

#### Get unmuted feeds

```shell
$ grpc_cli call <URL> AdminService.GetUnmutedFeeds 'Username: "<username>"'
```

#### Add unmuted feeds

```shell
$ grpc_cli call <URL> AdminService.AddUnmutedFeed <<EOF
Username: "<username>"
UnmutedFeedId: <id>
EOF
```

#### Delete unmuted feeds

```shell
$ grpc_cli call <URL> AdminService.DeleteUnmutedFeed <<EOF
Username: "<username>"
UnmutedFeedId: <id>
EOF
```

## Schema Updates

When running in Dockerized mode, execute the following command against the
CockroachDB container:

> [!CAUTION]
> Running schema updates while serving traffic can give unexpected results.

```bash
$ ./goliath.sh up

$ VERSION=<version> # such as "v19_add_saved_to_article.sql"
$ docker exec -it crdb-service bash -c "./cockroach sql --insecure < scripts/${VERSION}"
```

## CRDB SQL Shell

To get access to the CRDB SQL shell, run:

```shell
# Against the `prod` profile:
$ docker exec -it crdb-service ./cockroach sql --insecure --database=goliath

# Against the `dev` profile:
$ docker exec -it crdb-dev ./cockroach sql --insecure --database=goliath

# Against the `debug` profile:
$ docker exec -it crdb-debug ./cockroach sql --insecure --database=goliath
```

### Example Queries

#### Get all feed names and ids:

```cockroach
SELECT id, title
from Feed;
```

#### Delete all articles associated with a feed

```cockroach
DELETE
from article
where feed = $id;
```

#### Reset last retrieved timestamp of feed (to force refresh)

```cockroach
UPDATE feed
set latest = TIMESTAMPTZ '1970-01-01'
WHERE id = $id;
```

#### Update feed URL of existing feed

```cockroach
UPDATE feed
set url = '<URL>'
where id = $id;
```

## Debugger Support

Run the Docker Compose containers in the `debug` profile:

```shell
$ ./goliath.sh up --env debug --attached -- --build
```

This builds the backend binary in a debugging configuration
(`-gcflags="all=-N -l"`) and runs the binary under the `dlv` binary listening on
port 40000. This port is also exposed on the `debug` profile when running the
containers.

Set up a configuration where the debugger can be attached to. In IntelliJ, this
is a `Go Remote` run configuration that attaches to port 40000.

Once the containers are started, `dlv` will wait to start the process until the
run configuration in IntelliJ is begun. After that point, use the debugger in
IntelliJ as normal, setting breakpoints, examining local variables, etc.
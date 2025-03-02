# Administrative Operations

## Admin Server gRPC

### Get all feeds

```shell
$ grpc_cli call <URL> AdminService.GetFeeds <<EOF
Username: "<Username>"
EOF
```

### Add new feed

```shell
$ grpc_cli call <URL> AdminService.AddFeed <<EOF
Username: "<Username>"
Title: "<Title>"
URL: "<URL of feed>"
Link: "<URL of homepage>"
Folder: "<Folder>"
EOF
```

### Remove a feed

```shell
$ grpc_cli call <URL> AdminService.RemoveFeed <<EOF
Username: "<Username>"
Id: <id>
EOF
```

### Edit a feed

```shell
$ grpc_cli call <URL> AdminService.EditFeed <<EOF
Username: "<Username>"
Id: <id>
Folder: "<Name of folder>"
EOF
```

### Get mute words

```shell
$ grpc_cli call <URL> AdminService.GetMuteWords <<EOF
Username: "<Username>"
EOF
```

### Add mute words

```shell
$ grpc_cli call <URL> AdminService.AddMuteWord <<EOF
Username: "<Username>"
MuteWord: "<word>"
EOF
```

### Delete mute words

```shell
$ grpc_cli call <URL> AdminService.DeleteMuteWord <<EOF
Username: "<Username>"
MuteWord: "<word>"
EOF
```

### Get unmuted feeds

```shell
$ grpc_cli call <URL> AdminService.GetUnmutedFeeds <<EOF
Username: "<Username>"
EOF
```

### Add unmuted feeds

```shell
$ grpc_cli call <URL> AdminService.AddUnmutedFeed <<EOF
Username: "<Username>"
UnmutedFeedId: <id>
EOF
```

### Delete unmuted feeds

```shell
$ grpc_cli call <URL> AdminService.DeleteUnmutedFeed <<EOF
Username: "<Username>"
UnmutedFeedId: <id>
EOF
```

## CRDB Debugging

To get access to the CRDB sql shell, run:

```shell
$ docker exec -it crdb-service /bin/bash
[root@crdb cockroach]# ./cockroach sql --insecure --database=goliath
```

## Debugger Support

Run the Docker Compose containers in the `debug` profile:

```shell
$ ./goliath.sh up --env debug --attached -- --build
```
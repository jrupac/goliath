## Example Queries

## Get all feeds

```shell
$ grpc_cli call <URL> AdminService.GetFeeds <<EOF
Username: "<Username>"
EOF
```

## Add new feed

```shell
$ grpc_cli call <URL> AdminService.AddFeed <<EOF
Username: "<Username>"
Title: "<Title>"
URL: "<URL of feed>"
Link: "<URL of homepage>"
Folder: "<Folder>"
EOF
```

## Remove a feed

```shell
$ grpc_cli call <URL> AdminService.RemoveFeed <<EOF
Username: "<Username"
Id: <id>
EOF
```
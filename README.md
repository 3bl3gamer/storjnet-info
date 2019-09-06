# StorjInfo

Starts single node and saves node IDs from incoming connections. Preriodically updates nodes info by calling `LookupNode` and `NodeInfo`.

May go down after upcoming [Kademlia removal](https://storj.io/blog/2019/08/so-youre-a-storage-node-operator.-which-satellites-do-you-trust/). Or may not, details yet unclear.

## DB setup
```bash
createuser storjinfo -P
createdb storjinfo_db -O storjinfo --echo
psql -U storjinfo storjinfo_db -c "CREATE SCHEMA storjinfo"

go run migrations/*.go init
go run migrations/*.go
```

## Building
`go build -v`

## Start collecting data
Start storj node listening on `--server.private-address 127.0.0.1:7778` (it's default value but forwarding may be required in case of docker).

Then do `./storj3stat run`.

## Start HTTP server
`./storj3stat start-http-server`

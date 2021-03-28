# StorjNet

## Useful APIs

### GET /api/neighbors/\<subnet\>

Where `subnet` may be actual subnet address like `1.2.3.0` or just IP `1.2.3.4`.

**Response**

```json
{"ok": true, "result": {"count": 3}}
```

### POST /api/neighbors

**Request payload**

```json
{
  "subnets": ["1.2.3.4", "2.3.4.0", "2.3.5.0"],
  "myNodeIds": ["1AaaAA","1AaaAB","1AaaAC"]
}
```

* `subnets` — list of subnets/IPs;
* `myNodeIds` — optional list of node IDs to count foreign nodes.

**Response**

```json
{
  "ok": true,
  "result": {
    "counts": [
      {
        "subnet": "1.2.3.0",
        "nodesTotal": 4,
        "foreignNodesCount": 2
      },
      {
        "subnet": "2.3.4.0",
        "nodesTotal": 1,
        "foreignNodesCount": 0
      }
    ]
  }
}
```

* `subnet` — requested subnet (with trailing `.0`);
* `nodesTotal` — total nodes in subnet;
* `foreignNodesCount` — count of subnet nodes except `myNodeIds`.

Items will be absent for empty subnets.

## DB setup
```bash
sudo su - postgres
createuser storjnet -P  # with password "storj"
createdb storjnet_db -O storjnet --echo
psql storjnet_db -c "CREATE SCHEMA storjnet AUTHORIZATION storjnet"
psql storjnet_db -c "CREATE EXTENSION pgcrypto WITH SCHEMA storjnet"

go run migrations/*.go init
go run migrations/*.go
```

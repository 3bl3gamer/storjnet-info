# StorjNet

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
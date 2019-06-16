#!/bin/bash
#sudo su - postgres -c 'createuser storjinfo -P'
sudo su - postgres -c 'createdb storjinfo_db -O storjinfo --echo'
psql -U storjinfo storjinfo_db -c "CREATE SCHEMA storjinfo"

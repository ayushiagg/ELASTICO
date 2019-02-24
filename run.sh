#!/bin/bash
sudo service rabbitmq-server restart
go build elastico.go
./elastico

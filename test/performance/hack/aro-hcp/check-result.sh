#!/usr/bin/env bash

db_pod_name=$(kubectl -n maestro get pods -l name=maestro-db -ojsonpath='{.items[0].metadata.name}')

kubectl -n maestro exec ${db_pod_name} -- psql -d maestro -U maestro -c 'select count(*) from resources'
kubectl -n maestro exec ${db_pod_name} -- psql -d maestro -U maestro -c "select created_at,updated_at,extract(epoch from age(updated_at,created_at)) from resources where consumer_name='maestro-cluster-9' order by created_at"
kubectl -n maestro exec ${db_pod_name} -- psql -d maestro -U maestro -c "select created_at,updated_at,extract(epoch from age(updated_at,created_at)) from resources where consumer_name='maestro-cluster-10' order by created_at"
kubectl -n maestro exec ${db_pod_name} -- psql -d maestro -U maestro -c "select pg_size_pretty(pg_total_relation_size('resources')) as total, pg_size_pretty(pg_relation_size('resources')) as data"

# Plans Index

## Sessions

- date: 2026-04-29
  jira: ARO-26559
  jira_url: https://redhat.atlassian.net//browse/ARO-26559
  plan: plans/2026-04-29-resync-status-hash-limit.md
  status: "Done"
  pr: ~
  summary: "Introduced statusHashBatchSize=2000 hybrid resync: real hashes for first 2000 resources, blank hashes for the rest to stay under the MQTT 512 KB packet limit."

- date: 2026-04-30
  jira: ARO-26559
  jira_url: https://redhat.atlassian.net//browse/ARO-26559
  plan: plans/2026-04-30-resync-status-hash-batching.md
  status: "Done"
  pr: ~
  summary: "Replaced blank-hash fallback with proper batching: resyncConsumer now sends sequential CloudEvents of ≤1000 resources each with real hashes, including soft-deleted resources, with a V(2) log per batch showing X/Y progress and byte size."

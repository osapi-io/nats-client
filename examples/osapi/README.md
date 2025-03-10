# OSAPI

A worker consumer to interact with a work queue stream.

API Client (requester) → Publishes a unique request to NATS JetStream (REQUESTS stream).
Server (worker) → Listens for new requests, processes them, and stores the response in NATS Key-Value (responses bucket) using the request UUID as the key.
API Client (requester) → Fetches the response from NATS KV Store when ready.






## Create the Request Stream

nats stream add REQUESTS --subjects "requests.*" --storage file --retention workq
nats stream edit REQUESTS --discard new


## Create the Request Stream

cat > consumer.json <<EOF
{
  "durable_name": "request-worker",
  "ack_policy": "explicit",
  "deliver_policy": "new",
  "filter_subject": "requests.*",
  "replay_policy": "instant",
  "max_deliver": 10,
  "backoff": [5000000000, 10000000000, 15000000000]
}
EOF

nats consumer add REQUESTS request-worker --config=consumer.json


### DLQ

nats stream create requests_dlq --subjects '$JS.EVENT.ADVISORY.CONSUMER.MAX_DELIVERIES.REQUESTS.*'


### Publish Request

REQUEST_ID=$(uuidgen)

nats pub "requests.dns" '{"query": "example.com"}' -H "NATS-Msg-Id: $REQUEST_ID"
nats pub "requests.dns" "{\"id\": \"$REQUEST_ID\", \"query\": \"example.com\"}" -H "NATS-Msg-Id: $REQUEST_ID"




### Pull requst from JS

REQUEST=$(nats consumer next REQUESTS request-worker --raw)
echo "Received request: $REQUEST"


### Create a KV Store for Processed Messages

nats kv add responses


### WOrker


bash -c '
while true; do
  # Get raw JSON message (no headers, only data)
  MESSAGE=$(nats consumer next REQUESTS request-worker --raw 2>/dev/null || echo "ERROR")

  if [[ "$MESSAGE" == "ERROR" ]]; then
    echo "No messages available, waiting..."
    sleep 1
    continue
  fi

  echo "Extracted message body: $MESSAGE"

  # Extract request ID from JSON payload
  REQUEST_ID=$(echo "$MESSAGE" | jq -r ".id" 2>/dev/null)

  if [[ -z "$REQUEST_ID" || "$REQUEST_ID" == "null" ]]; then
    echo "Invalid or missing request ID!"
    continue
  fi

  # Extract query value from JSON
  QUERY=$(echo "$MESSAGE" | jq -r ".query" 2>/dev/null)

  echo "Storing response in KV store for request ID: $REQUEST_ID"

  # Construct response JSON
  RESPONSE="{\"id\": \"$REQUEST_ID\", \"query\": \"$QUERY\", \"response\": \"Success\"}"

  # Store response in KV store
  nats kv put responses "$REQUEST_ID" "$RESPONSE"
done'


## Retrieve the Response from KV


nats kv get responses "$REQUEST_ID"

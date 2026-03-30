#!/bin/bash

URL="http://host.docker.internal:3000"
MAX_SECONDS=${TIMEOUT:-500}
START_TIME=$(date +%s)

echo "Waiting for $URL"
echo "Timeout: $MAX_SECONDS seconds"

while true; do
    CURRENT_TIME=$(date +%s)
    ELAPSED_TIME=$((CURRENT_TIME - START_TIME))

    if [ $ELAPSED_TIME -ge $MAX_SECONDS ]; then
        echo "❌ Error"
        exit 1
    fi

    HTTP_CODE=$(curl --head --location --connect-timeout 5 --max-time 5 --write-out %{http_code} --silent --output /dev/null "$URL")

    if [ "$HTTP_CODE" -eq "200" ]; then
        echo "✅ Success, elapsed time: ${ELAPSED_TIME} seconds"
        exit 0
    else
        echo "⏳ ($ELAPSED_TIME/${MAX_SECONDS}s) Code received [$HTTP_CODE]. Retrying in 2 seconds..."

        sleep 2
    fi
done
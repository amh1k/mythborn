FROM temporalio/admin-tools:1.32.0@sha256:a9f84fb9a374b2374fe2e67c8efc0468ff3f1c66c8a0b14597ec86e349e62bca AS tools
FROM temporalio/server:1.32.0@sha256:c3e752127759616bb1615e0f9ba0e21635aeb5fdeb922de4f371c350955f46ae
USER root
RUN apk add --no-cache postgresql-client
COPY --from=tools /usr/local/bin/temporal-sql-tool /usr/local/bin/temporal-sql-tool
COPY --from=tools /usr/local/bin/temporal /usr/local/bin/temporal
COPY --from=tools /etc/temporal/schema /etc/temporal/schema
COPY deploy/temporal/config.yaml /etc/mythborn/temporal.yaml
COPY --chmod=755 deploy/temporal/setup.sh deploy/temporal/start.sh /etc/mythborn/
USER temporal
ENV TEMPORAL_SERVER_CONFIG_FILE_PATH=/etc/mythborn/temporal.yaml
EXPOSE 7233
ENTRYPOINT ["/etc/mythborn/start.sh"]

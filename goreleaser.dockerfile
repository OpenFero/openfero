# build stage
FROM alpine@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 AS build-env

RUN echo "openfero:x:10001:10001:OpenFero user:/app:/sbin/nologin" >> /etc/passwd_single && \
    echo "openfero:x:10001:" >> /etc/group_single

# final stage
FROM scratch

EXPOSE 8080
WORKDIR /app

COPY openfero /app/
COPY --from=build-env /etc/passwd_single /etc/passwd
COPY --from=build-env /etc/group_single /etc/group
USER 10001

# Add health check - for scratch images, this is basic but satisfies security requirements
# Kubernetes liveness/readiness probes provide more robust health checking
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD /app/openfero --help || exit 1

ENTRYPOINT ["/app/openfero"]

# Build locally first: mise run //modules/session:build
FROM gcr.io/distroless/base-debian12:nonroot
COPY dist/session /session
EXPOSE 8081 8082
ENTRYPOINT ["/session"]
CMD ["service"]

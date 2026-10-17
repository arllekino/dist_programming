# Build locally first: mise run //modules/identity:build
FROM gcr.io/distroless/base-debian12:nonroot
COPY dist/identity /identity
EXPOSE 8081 8082
ENTRYPOINT ["/identity"]
CMD ["service"]

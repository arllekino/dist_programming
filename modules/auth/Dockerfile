# Build locally first: mise run //modules/auth:build
FROM gcr.io/distroless/base-debian12:nonroot
COPY dist/auth /auth
EXPOSE 8082
ENTRYPOINT ["/auth"]
CMD ["service"]

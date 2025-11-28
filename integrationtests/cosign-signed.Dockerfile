FROM gcr.io/distroless/static:nonroot
ARG GITHUB_REPOSITORY
LABEL org.opencontainers.image.source="https://github.com/${GITHUB_REPOSITORY}"
LABEL org.opencontainers.image.description="Test image signed with cosign (signature + attestations)"
LABEL test.image.type="cosign-signed"
USER nobody
